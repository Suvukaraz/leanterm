package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/telnet"
)

type TelnetSessionOptions struct {
	TmpID         string
	Conn          config.Connection
	Password      string
	SocksPassword string
	JumpPassword  string
	GlobalNet     config.NetworkConfig
	Emitter       EventEmitter
}

type TelnetSession struct {
	mu            sync.Mutex
	tmpID         string
	conn          config.Connection
	password      string
	socksPassword string
	jumpPassword  string
	globalNet     config.NetworkConfig
	state         string // "connecting", "connected", "disconnected", "closed"
	stream        *Stream
	emitter       EventEmitter
	client        *telnet.Client
	netConn       net.Conn
	matcher       *PromptMatcher
	ctx           context.Context
	cancel        context.CancelFunc
	cols          int
	rows          int
}

func parseEscapeChar(s string) byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if len(s) == 2 && s[0] == '^' {
		c := s[1]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c >= '@' && c <= '_' {
			return c - '@'
		}
	}
	if len(s) == 1 {
		return s[0]
	}
	return 0x1D // default ^]
}

func NewTelnetSession(opts TelnetSessionOptions) *TelnetSession {
	tmpID := opts.TmpID
	if tmpID == "" {
		tmpID = uuid.NewString()
	}

	sink := &eventSink{
		event:   fmt.Sprintf("term:data:%s", tmpID),
		emitter: opts.Emitter,
	}

	return &TelnetSession{
		tmpID:         tmpID,
		conn:          opts.Conn,
		password:      opts.Password,
		socksPassword: opts.SocksPassword,
		jumpPassword:  opts.JumpPassword,
		globalNet:     opts.GlobalNet,
		state:         "connecting",
		stream:        NewStream(tmpID, sink),
		emitter:       opts.Emitter,
		cols:          80,
		rows:          24,
	}
}

func (s *TelnetSession) TmpID() string { return s.tmpID }
func (s *TelnetSession) Title() string { return s.conn.Title }
func (s *TelnetSession) Kind() string  { return "telnet" }

func (s *TelnetSession) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *TelnetSession) Stream() *Stream { return s.stream }

func (s *TelnetSession) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.startLocked()
}

func (s *TelnetSession) startLocked() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.state = "connecting"
	s.notifyStateLocked("connecting")

	port := s.conn.Port
	if port <= 0 {
		port = 23
	}

	// 1. Spawning status with CLI representation
	cmdLine := "telnet"
	if s.conn.Telnet.IPVersion == "4" {
		cmdLine += " -4"
	} else if s.conn.Telnet.IPVersion == "6" {
		cmdLine += " -6"
	}
	if s.conn.Telnet.BindAddress != "" {
		cmdLine += " -b " + s.conn.Telnet.BindAddress
	}
	if s.conn.Telnet.EscapeChar != "" {
		cmdLine += " -e " + s.conn.Telnet.EscapeChar
	}
	cmdLine += fmt.Sprintf(" %s %d", s.conn.Host, port)
	s.notifyStatusLocked(fmt.Sprintf("SPAWNING:%s", cmdLine))

	// 2. Greeting line in terminal
	greeting := fmt.Sprintf("\x1b[32m=== LeanTerm: Connecting to %s (%s:%d) via Telnet ===\x1b[0m\r\n", s.conn.Title, s.conn.Host, port)
	_, _ = s.stream.Write([]byte(greeting))

	// 3. Dial transport
	s.notifyStatusLocked("CONNECTING")

	timeout := time.Duration(s.conn.Terminal.TimeoutConnect) * time.Second
	if timeout <= 0 {
		timeout = 40 * time.Second
	}

	dialOpts := DialOptions{
		NetworkMode:   s.conn.Network.Mode,
		Host:          s.conn.Host,
		Port:          port,
		Timeout:       timeout,
		IPVersion:     s.conn.Telnet.IPVersion,
		BindAddress:   s.conn.Telnet.BindAddress,
		Socks:         s.conn.Network.Socks,
		SocksPassword: s.socksPassword,
		Jump:          s.conn.Network.Jump,
		JumpPassword:  s.jumpPassword,
		GlobalNetwork: s.globalNet,
	}

	netConn, err := DialTransport(s.ctx, dialOpts)
	if err != nil {
		s.handleErrorLocked(fmt.Sprintf("Failed to connect: %v", err))
		return err
	}
	s.netConn = netConn

	// Set TCP KeepAlive
	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(60 * time.Second)
	}

	// 4. Create Telnet client
	escChar := parseEscapeChar(s.conn.Telnet.EscapeChar)
	tOpts := telnet.Options{
		TerminalType: s.conn.Telnet.TerminalType,
		NAWS:         s.conn.Telnet.NAWS,
		LineEnding:   s.conn.Telnet.LineEnding,
		EscapeChar:   escChar,
		Username:     s.conn.Auth.User,
		Cols:         s.cols,
		Rows:         s.rows,
	}
	s.client = telnet.NewClient(netConn, tOpts)

	// 5. Initialize PromptMatcher for automatic login
	s.matcher = NewPromptMatcher(PromptMatcherOptions{
		AuthType:   s.conn.Auth.Type,
		Username:   s.conn.Auth.User,
		Password:   s.password,
		LineEnding: "\r\n",
		Prompts:    s.conn.Terminal.Prompts,
		TimeoutSec: s.conn.Terminal.TimeoutConnect,
		OnSend: func(text string) {
			s.mu.Lock()
			cl := s.client
			s.mu.Unlock()
			if cl != nil {
				_, _ = cl.Write([]byte(text))
			}
		},
		OnStatus: func(code string) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.notifyStatusLocked(code)
		},
		OnConnected: func(note string) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.state = "connected"
			s.notifyStateLocked("connected")
			if note != "" {
				s.notifyStatusLocked(fmt.Sprintf("CONNECTED:%s", note))
			} else {
				s.notifyStatusLocked("CONNECTED")
			}
		},
		OnError: func(errCode string) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.handleErrorLocked(errCode)
		},
	})

	if s.conn.Auth.Type == "manual" {
		s.state = "connected"
		s.notifyStateLocked("connected")
		s.notifyStatusLocked("CONNECTED")
	}

	// Read loop
	go s.readLoop()

	return nil
}

func (s *TelnetSession) readLoop() {
	buf := make([]byte, 8192)
	for {
		s.mu.Lock()
		cl := s.client
		s.mu.Unlock()

		if cl == nil {
			break
		}

		n, err := cl.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if s.matcher != nil {
				s.matcher.FeedOutput(chunk)
			}
			_, _ = s.stream.Write(chunk)
		}
		if err != nil {
			break
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == "closed" {
		return
	}

	s.state = "disconnected"
	s.notifyStateLocked("disconnected")
	s.notifyStatusLocked("CLOSE:Connection ended by remote peer!!")

	disconnectMsg := "\r\n\x1b[31mConnection ended by remote peer!!\x1b[0m\r\n\x1b[33mPress <Enter> to reconnect\x1b[0m\r\n"
	_, _ = s.stream.Write([]byte(disconnectMsg))
}

func (s *TelnetSession) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == "disconnected" {
		// Reconnect on Enter
		for _, b := range data {
			if b == '\r' || b == '\n' {
				return s.restartLocked()
			}
		}
		return nil
	}

	if s.client == nil {
		return io.ErrClosedPipe
	}

	if s.matcher != nil {
		s.matcher.ManualInput()
	}

	_, err := s.client.Write(data)
	return err
}

func (s *TelnetSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cols = cols
	s.rows = rows

	if s.client != nil {
		return s.client.SendNAWS(cols, rows)
	}
	return nil
}

func (s *TelnetSession) Ack(seq uint64) {
	s.stream.Ack(seq)
}

func (s *TelnetSession) SendCommand(cmd byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client.SendCommand(cmd)
	}
	return nil
}

func (s *TelnetSession) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == "disconnected" || s.state == "closed" {
		return nil
	}

	s.state = "disconnected"
	if s.cancel != nil {
		s.cancel()
	}
	if s.matcher != nil {
		s.matcher.Stop()
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.netConn != nil {
		_ = s.netConn.Close()
		s.netConn = nil
	}

	s.notifyStateLocked("disconnected")
	s.notifyStatusLocked("DISCONNECTED:User requested disconnect")

	banner := "\r\n\x1b[33mDisconnected. Press <Enter> to reconnect\x1b[0m\r\n"
	_, _ = s.stream.Write([]byte(banner))

	return nil
}

func (s *TelnetSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = "closed"
	if s.cancel != nil {
		s.cancel()
	}
	if s.matcher != nil {
		s.matcher.Stop()
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.netConn != nil {
		_ = s.netConn.Close()
		s.netConn = nil
	}

	s.notifyStateLocked("closed")
	return nil
}

func (s *TelnetSession) Restart() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.restartLocked()
}

func (s *TelnetSession) restartLocked() error {
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.netConn != nil {
		_ = s.netConn.Close()
		s.netConn = nil
	}
	if s.matcher != nil {
		s.matcher.Stop()
	}

	return s.startLocked()
}

func (s *TelnetSession) handleErrorLocked(errMsg string) {
	s.state = "disconnected"
	s.notifyStateLocked("disconnected")
	s.notifyStatusLocked(fmt.Sprintf("DISCONNECTED:%s", errMsg))

	line := fmt.Sprintf("\r\n\x1b[31m%s\x1b[0m\r\n\x1b[33mPress <Enter> to reconnect\x1b[0m\r\n", errMsg)
	_, _ = s.stream.Write([]byte(line))
}

func (s *TelnetSession) notifyStateLocked(state string) {
	if s.emitter != nil {
		s.emitter.Emit("session:state", map[string]any{
			"tmpID": s.tmpID,
			"state": state,
			"title": s.conn.Title,
		})
	}
}

func (s *TelnetSession) notifyStatusLocked(code string) {
	if s.emitter != nil {
		s.emitter.Emit("session:status", map[string]any{
			"tmpID": s.tmpID,
			"code":  code,
			"text":  code,
			"time":  time.Now().Format(time.RFC3339),
		})
	}
}
