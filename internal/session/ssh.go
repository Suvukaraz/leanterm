package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/forward"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHSession is one SSH connection with an interactive shell (or, for the
// "sftp" method / "no remote command", only the transport).
//
// Locking: s.mu only guards the fields below and is never held across network
// I/O, so State(), Write() and Resize() stay responsive while connecting.
type SSHSession struct {
	mu            sync.Mutex
	tmpID         string
	conn          config.Connection
	password      string
	keyPassphrase string
	socksPassword string
	jumpPassword  string
	globalNet     config.NetworkConfig
	autoAccept    bool
	prompter      Prompter
	saveSecret    func(field, value string)
	shellless     bool // no PTY/shell (method "sftp" or no_remote_command)
	state         string
	stream        *Stream
	emitter       EventEmitter
	client        *ssh.Client
	sshSession    *ssh.Session
	stdin         io.WriteCloser
	forwarder     *forward.Forwarder
	ctx           context.Context
	cancel        context.CancelFunc
	generation    int // incremented per connect/disconnect; stale goroutines check it
	cols          int
	rows          int
}

type SSHSessionOptions struct {
	TmpID         string
	Conn          config.Connection
	Password      string
	KeyPassphrase string
	SocksPassword string
	JumpPassword  string
	GlobalNet     config.NetworkConfig
	AutoAccept    bool
	Emitter       EventEmitter
	Prompter      Prompter
	SaveSecret    func(field, value string)
	Shellless     bool
}

func NewSSHSession(opts SSHSessionOptions) *SSHSession {
	tmpID := opts.TmpID
	if tmpID == "" {
		tmpID = uuid.NewString()
	}

	sink := &eventSink{
		event:   fmt.Sprintf("term:data:%s", tmpID),
		emitter: opts.Emitter,
	}

	return &SSHSession{
		tmpID:         tmpID,
		conn:          opts.Conn,
		password:      opts.Password,
		keyPassphrase: opts.KeyPassphrase,
		socksPassword: opts.SocksPassword,
		jumpPassword:  opts.JumpPassword,
		globalNet:     opts.GlobalNet,
		autoAccept:    opts.AutoAccept,
		prompter:      opts.Prompter,
		saveSecret:    opts.SaveSecret,
		shellless:     opts.Shellless || opts.Conn.SSH.NoRemoteCommand,
		state:         "disconnected",
		stream:        NewStream(tmpID, sink),
		emitter:       opts.Emitter,
		cols:          80,
		rows:          24,
	}
}

func (s *SSHSession) TmpID() string { return s.tmpID }
func (s *SSHSession) Title() string { return s.conn.Title }
func (s *SSHSession) Kind() string  { return "ssh" }
func (s *SSHSession) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
func (s *SSHSession) Stream() *Stream { return s.stream }
func (s *SSHSession) SSHClient() *ssh.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// Start connects synchronously (callers run it in a goroutine). Calls while a
// connection attempt is running or the session is connected are ignored.
func (s *SSHSession) Start() error {
	s.mu.Lock()
	if s.state == "connecting" || s.state == "connected" || s.state == "closed" {
		s.mu.Unlock()
		return nil
	}
	s.generation++
	gen := s.generation
	s.ctx, s.cancel = context.WithCancel(context.Background())
	ctx := s.ctx
	s.setStateLocked("connecting")
	cols, rows := s.cols, s.rows
	s.mu.Unlock()

	timeStr := time.Now().Format("15:04:05 02-01-2006")
	greeting := fmt.Sprintf("\r\n\x1b[32mCONNECTING WITH\x1b[0m \x1b[33m'%s'\x1b[0m (%s)\r\n\n", s.conn.Title, timeStr)
	_, _ = s.stream.Write([]byte(greeting))

	err := s.connect(ctx, gen, cols, rows)
	if err != nil {
		s.mu.Lock()
		stale := s.generation != gen || s.state == "closed"
		if !stale {
			s.teardownLocked()
		}
		s.mu.Unlock()
		if !stale {
			s.reportError(err)
		}
	}
	return err
}

func (s *SSHSession) connect(ctx context.Context, gen, cols, rows int) error {
	port := s.conn.Port
	if port <= 0 {
		port = 22
	}

	user := s.conn.Auth.User
	if s.conn.Auth.Type == "publickey" && s.conn.Auth.KeyUser != "" {
		user = s.conn.Auth.KeyUser
	}
	if user == "" {
		if s.prompter == nil {
			return errors.New("LOGIN:Empty username is not valid to login here")
		}
		resp, err := ask(ctx, s.prompter, PromptRequest{
			TmpID:   s.tmpID,
			Kind:    "credentials",
			Title:   "Login",
			Message: fmt.Sprintf("Enter the user name for %s", s.conn.Host),
			Fields:  []PromptField{{Name: "user", Label: "User"}},
		})
		if err != nil {
			return fmt.Errorf("LOGIN:%w", err)
		}
		user = strings.TrimSpace(resp.Values["user"])
		if user == "" {
			return errors.New("LOGIN:Empty username is not valid to login here")
		}
	}

	s.mu.Lock()
	password, keyPassphrase := s.password, s.keyPassphrase
	s.mu.Unlock()

	authMethods, err := BuildAuthMethods(AuthOptions{
		AuthType:      s.conn.Auth.Type,
		User:          user,
		Host:          s.conn.Host,
		Password:      password,
		KeyFile:       s.conn.Auth.KeyFile,
		KeyPassphrase: keyPassphrase,
		Exclusive:     s.conn.Auth.Exclusive,
		Ctx:           ctx,
		Prompter:      s.prompter,
		TmpID:         s.tmpID,
		SaveSecret: func(field, value string) {
			s.mu.Lock()
			switch field {
			case "password":
				s.password = value
			case "key_passphrase":
				s.keyPassphrase = value
			}
			s.mu.Unlock()
			if s.saveSecret != nil {
				s.saveSecret(field, value)
			}
		},
	})
	if err != nil {
		return err
	}

	timeout := time.Duration(s.conn.Terminal.TimeoutConnect) * time.Second
	if timeout <= 0 {
		timeout = 40 * time.Second
	}

	netConn, err := DialTransport(ctx, DialOptions{
		NetworkMode:   s.conn.Network.Mode,
		Host:          s.conn.Host,
		Port:          port,
		Timeout:       timeout,
		IPVersion:     s.conn.SSH.IPVersion,
		Socks:         s.conn.Network.Socks,
		SocksPassword: s.socksPassword,
		Jump:          s.conn.Network.Jump,
		JumpPassword:  s.jumpPassword,
		GlobalNetwork: s.globalNet,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to %s:%d: %w", s.conn.Host, port, err)
	}

	// Abort the handshake when the user disconnects/closes meanwhile.
	stopWatch := context.AfterFunc(ctx, func() { _ = netConn.Close() })
	defer stopWatch()

	hkm := NewHostKeyManager("", s.autoAccept, s.emitter).WithPrompter(s.prompter, s.tmpID)
	targetAddr := net.JoinHostPort(s.conn.Host, strconv.Itoa(port))
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hkm.HostKeyCallbackContext(ctx),
		// No handshake timeout: it may wait for dialogs (host key, password).
		// The TCP connect itself is bounded by the dialer timeout.
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(netConn, targetAddr, sshConfig)
	if err != nil {
		_ = netConn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("SSH handshake failed: %w", err)
	}
	client := ssh.NewClient(clientConn, chans, reqs)

	forwarder := forward.NewForwarder()
	_ = forwarder.Start(ctx, client, s.conn.SSH)

	if s.shellless {
		s.mu.Lock()
		if s.generation != gen || s.state == "closed" {
			s.mu.Unlock()
			_ = forwarder.Close()
			_ = client.Close()
			return context.Canceled
		}
		s.client = client
		s.forwarder = forwarder
		s.setStateLocked("connected")
		s.mu.Unlock()
		if s.conn.Method != "sftp" {
			_, _ = s.stream.Write([]byte("\x1b[33mPort forwarding only (no remote command)\x1b[0m\r\n"))
		}
		go s.waitClient(client, gen)
		return nil
	}

	sess, err := client.NewSession()
	if err != nil {
		_ = forwarder.Close()
		_ = client.Close()
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	if s.conn.SSH.ForwardAgent {
		if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
			if agentConn, err := net.Dial("unix", authSock); err == nil {
				if err := agent.ForwardToAgent(client, agent.NewClient(agentConn)); err == nil {
					_ = agent.RequestAgentForwarding(sess)
				}
			}
		}
	}
	if s.conn.SSH.ForwardX11 {
		_ = forward.SetupX11(ctx, sess, client)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	for _, env := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		if v := os.Getenv(env); v != "" {
			_ = sess.Setenv(env, v) // servers may refuse; that's fine
		}
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sess.Close()
		_ = forwarder.Close()
		_ = client.Close()
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		_ = forwarder.Close()
		_ = client.Close()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdoutPipe, _ := sess.StdoutPipe()
	stderrPipe, _ := sess.StderrPipe()

	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = forwarder.Close()
		_ = client.Close()
		return fmt.Errorf("failed to start remote shell: %w", err)
	}

	s.mu.Lock()
	if s.generation != gen || s.state == "closed" {
		s.mu.Unlock()
		_ = sess.Close()
		_ = forwarder.Close()
		_ = client.Close()
		return context.Canceled
	}
	s.client = client
	s.sshSession = sess
	s.stdin = stdinPipe
	s.forwarder = forwarder
	// Apply a resize that happened while connecting.
	if s.cols != cols || s.rows != rows {
		_ = sess.WindowChange(s.rows, s.cols)
	}
	s.setStateLocked("connected")
	s.mu.Unlock()

	go s.readStream(stdoutPipe)
	go s.readStream(stderrPipe)
	go s.waitSession(sess, gen)
	return nil
}

func parseOSC7(data []byte) string {
	oscPrefix := []byte("\x1b]7;file://")
	idx := bytes.Index(data, oscPrefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(oscPrefix)
	end := -1
	for i := start; i < len(data); i++ {
		if data[i] == 0x07 {
			end = i
			break
		}
		if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
			end = i
			break
		}
	}
	if end < 0 {
		return ""
	}
	raw := string(data[start:end])
	slashIdx := strings.Index(raw, "/")
	if slashIdx >= 0 {
		pathPart := raw[slashIdx:]
		decoded, err := url.PathUnescape(pathPart)
		if err == nil {
			return decoded
		}
		return pathPart
	}
	return ""
}

func (s *SSHSession) readStream(r io.Reader) {
	if r == nil {
		return
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if cwd := parseOSC7(buf[:n]); cwd != "" && s.emitter != nil {
				s.emitter.Emit(fmt.Sprintf("session:cwd:%s", s.tmpID), cwd)
			}
			_, _ = s.stream.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (s *SSHSession) waitSession(sess *ssh.Session, gen int) {
	err := sess.Wait()
	s.finishRemote(gen, exitReason(err))
}

func (s *SSHSession) waitClient(client *ssh.Client, gen int) {
	err := client.Wait()
	s.finishRemote(gen, exitReason(err))
}

func exitReason(err error) string {
	var exitErr *ssh.ExitError
	switch {
	case err == nil:
		return "remote shell exited"
	case errors.As(err, &exitErr):
		return fmt.Sprintf("remote shell exited with status %d", exitErr.ExitStatus())
	default:
		return "connection ended by remote peer"
	}
}

// finishRemote handles the end of a connection that was not caused by Disconnect/Close.
func (s *SSHSession) finishRemote(gen int, reason string) {
	s.mu.Lock()
	if s.generation != gen || s.state != "connected" {
		s.mu.Unlock()
		return
	}
	s.teardownLocked()
	s.setStateLocked("disconnected")
	s.mu.Unlock()

	s.emitStatus("DISCONNECTED", "DISCONNECTED:"+reason)
	timeStr := time.Now().Format("15:04:05 02-01-2006")
	_, _ = s.stream.Write([]byte(fmt.Sprintf("\r\n\x1b[31mDISCONNECTED (PRESS <ENTER> TO RECONNECT) (%s)\x1b[0m\r\n", timeStr)))
}

func (s *SSHSession) reportError(err error) {
	msg := err.Error()
	if errors.Is(err, context.Canceled) {
		msg = "connection attempt cancelled"
	}
	s.mu.Lock()
	s.setStateLocked("disconnected")
	s.mu.Unlock()

	s.emitStatus("CLOSE", "CLOSE:"+msg)
	timeStr := time.Now().Format("15:04:05 02-01-2006")
	_, _ = s.stream.Write([]byte(fmt.Sprintf("\r\n\x1b[31mERROR: %s\x1b[0m\r\n\x1b[31mDISCONNECTED (PRESS <ENTER> TO RECONNECT) (%s)\x1b[0m\r\n", msg, timeStr)))
}

// teardownLocked closes all network resources (caller holds s.mu).
func (s *SSHSession) teardownLocked() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	if s.sshSession != nil {
		_ = s.sshSession.Close()
		s.sshSession = nil
	}
	if s.forwarder != nil {
		_ = s.forwarder.Close()
		s.forwarder = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}

func (s *SSHSession) setStateLocked(st string) {
	if s.state == st {
		return
	}
	s.state = st
	if s.emitter != nil {
		s.emitter.Emit("session:state", map[string]any{
			"tmpID": s.tmpID,
			"state": st,
			"title": s.conn.Title,
		})
	}
}

func (s *SSHSession) emitStatus(code, text string) {
	if s.emitter != nil {
		s.emitter.Emit("session:status", map[string]any{
			"tmpID": s.tmpID,
			"code":  code,
			"text":  text,
			"time":  time.Now().Format(time.RFC3339),
		})
	}
}

func (s *SSHSession) Write(data []byte) error {
	s.mu.Lock()
	state := s.state
	stdin := s.stdin
	s.mu.Unlock()

	if state == "disconnected" {
		if bytes.ContainsAny(data, "\r\n") {
			go func() { _ = s.Start() }()
		}
		return nil
	}
	if stdin == nil {
		return nil // still connecting: input is dropped, like a real terminal before login
	}
	_, err := stdin.Write(data)
	return err
}

func (s *SSHSession) Resize(cols, rows int) error {
	s.mu.Lock()
	if cols > 0 {
		s.cols = cols
	}
	if rows > 0 {
		s.rows = rows
	}
	sess := s.sshSession
	cols, rows = s.cols, s.rows
	s.mu.Unlock()

	if sess != nil {
		return sess.WindowChange(rows, cols)
	}
	return nil
}

func (s *SSHSession) Ack(seq uint64) {
	s.stream.Ack(seq)
}

func (s *SSHSession) Disconnect() error {
	s.mu.Lock()
	if s.state == "closed" || s.state == "disconnected" {
		s.mu.Unlock()
		return nil
	}
	s.generation++
	s.teardownLocked()
	s.setStateLocked("disconnected")
	s.mu.Unlock()

	timeStr := time.Now().Format("15:04:05 02-01-2006")
	_, _ = s.stream.Write([]byte(fmt.Sprintf("\r\n\x1b[31mDISCONNECTED (PRESS <ENTER> TO RECONNECT) (%s)\x1b[0m\r\n", timeStr)))
	return nil
}

func (s *SSHSession) Close() error {
	s.mu.Lock()
	s.generation++
	s.teardownLocked()
	s.setStateLocked("closed")
	s.mu.Unlock()

	s.stream.Close()
	return nil
}

func (s *SSHSession) Restart() error {
	_ = s.Disconnect()
	go func() { _ = s.Start() }()
	return nil
}
