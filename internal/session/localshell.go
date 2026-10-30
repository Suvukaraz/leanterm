package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/config"
)

type EventEmitter interface {
	Emit(event string, data any)
}

// LocalSession manages a local shell process connected to a PTY.
type LocalSession struct {
	mu        sync.Mutex
	tmpID     string
	title     string
	state     string // "connecting", "connected", "disconnected", "closed"
	stream    *Stream
	cfg       config.LocalShellConfig
	emitter   EventEmitter
	proc      shellProcess
	// cwd is the last folder the shell reported (OSC 7 / OSC 9;9).
	cwd string
	ctx       context.Context
	cancel    context.CancelFunc
	startedAt time.Time
	cols      int
	rows      int
}

func NewLocalSession(tmpID string, cfg config.LocalShellConfig, emitter EventEmitter) *LocalSession {
	if tmpID == "" {
		tmpID = uuid.NewString()
	}

	sink := &eventSink{
		event:   fmt.Sprintf("term:data:%s", tmpID),
		emitter: emitter,
	}

	ls := &LocalSession{
		tmpID:     tmpID,
		title:     "Local Shell",
		state:     "connecting",
		stream:    NewStream(tmpID, sink),
		cfg:       cfg,
		emitter:   emitter,
		startedAt: time.Now(),
		cols:      80,
		rows:      24,
	}

	return ls
}

type eventSink struct {
	event   string
	emitter EventEmitter
}

func (s *eventSink) Send(seq uint64, b64 string) {
	if s.emitter != nil {
		s.emitter.Emit(s.event, StreamPacket{Seq: seq, B64: b64})
	}
}

func (ls *LocalSession) TmpID() string { return ls.tmpID }

// ShellPID returns the process id of the running shell, or 0 when it is not running.
func (ls *LocalSession) ShellPID() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.proc == nil || ls.state != "connected" {
		return 0
	}
	return ls.proc.Pid()
}
func (ls *LocalSession) Title() string { return ls.title }
func (ls *LocalSession) Kind() string  { return "local" }
func (ls *LocalSession) State() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.state
}
func (ls *LocalSession) Stream() *Stream { return ls.stream }

func (ls *LocalSession) Start() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	return ls.startLocked()
}

func (ls *LocalSession) startLocked() error {
	ls.ctx, ls.cancel = context.WithCancel(context.Background())
	ls.state = "connecting"
	ls.notifyStateLocked("connecting")

	// Greeting line on start (ANSI green): \r\nCONNECTING WITH 'Local Shell' (<local time>)\r\n\n
	timeStr := time.Now().Format("15:04:05 02-01-2006")
	greeting := fmt.Sprintf("\r\n\x1b[32mCONNECTING WITH '%s' (%s)\x1b[0m\r\n\n", ls.title, timeStr)
	_, _ = ls.stream.Write([]byte(greeting))

	// Resolve shell binary
	shell := ls.cfg.Binary
	if shell == "" {
		shell = defaultShell()
	}

	var args []string
	if ls.cfg.Options != "" {
		args = strings.Fields(ls.cfg.Options)
	}

	// Working directory
	dir := ls.cfg.Directory
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		} else {
			dir = "."
		}
	}

	args, env := withCwdReporting(shell, args, localShellEnv(os.Environ()))
	proc, err := startShell(ls.ctx, shellSpec{
		Path: shell,
		Args: args,
		Dir:  dir,
		Env:  env,
		Cols: ls.cols,
		Rows: ls.rows,
	})
	if err != nil {
		ls.state = "disconnected"
		ls.notifyStateLocked("disconnected")
		errMsg := fmt.Sprintf("\r\n\x1b[31mERROR: Failed to launch shell '%s': %v\x1b[0m\r\n", shell, err)
		_, _ = ls.stream.Write([]byte(errMsg))
		return err
	}

	ls.proc = proc
	ls.state = "connected"
	ls.notifyStateLocked("connected")

	// Goroutine to read from PTY into Stream
	go ls.readLoop(proc)

	// Goroutine to wait for process exit
	go ls.waitLoop(proc)

	return nil
}

// localShellEnv returns the environment for the local shell. When LeanTerm is
// started from a desktop launcher, TERM is unset or "dumb", which makes shells
// and tools disable colors. The PTY is rendered by an xterm-compatible engine,
// so advertise xterm-256color with truecolor support like the SSH session does.
func localShellEnv(base []string) []string {
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if strings.HasPrefix(kv, "TERM=") || strings.HasPrefix(kv, "COLORTERM=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
}

func (ls *LocalSession) readLoop(proc shellProcess) {
	buf := make([]byte, 32*1024)
	var scanner cwdScanner
	for {
		n, err := proc.Read(buf)
		if n > 0 {
			if cwd := scanner.Scan(buf[:n]); cwd != "" {
				ls.reportCwd(cwd)
			}
			_, _ = ls.stream.Write(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				// Normal terminal EOF on shell exit
			}
			break
		}
	}
}

// reportCwd remembers a folder the shell reported and tells the file browser
// ("session:cwd:<tmpID>", the event SSH sessions send for OSC 7).
func (ls *LocalSession) reportCwd(cwd string) {
	ls.mu.Lock()
	changed := cwd != ls.cwd
	ls.cwd = cwd
	ls.mu.Unlock()
	if changed && ls.emitter != nil {
		ls.emitter.Emit("session:cwd:"+ls.tmpID, cwd)
	}
}

// ReportedCwd is the last folder the shell reported, "" if it reports none.
func (ls *LocalSession) ReportedCwd() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.cwd
}

func (ls *LocalSession) waitLoop(proc shellProcess) {
	_ = proc.Wait()

	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.state == "closed" {
		return
	}

	// A restarted shell already replaced this one.
	if ls.proc != proc {
		return
	}
	ls.state = "disconnected"
	_ = proc.Close()
	ls.proc = nil

	// Greeting line on disconnect (ANSI red): DISCONNECTED (PRESS <ENTER> TO RECONNECT) (<time>)
	timeStr := time.Now().Format("15:04:05 02-01-2006")
	disconnectMsg := fmt.Sprintf("\r\n\x1b[31mDISCONNECTED (PRESS <ENTER> TO RECONNECT) (%s)\x1b[0m\r\n", timeStr)
	_, _ = ls.stream.Write([]byte(disconnectMsg))

	ls.notifyStateLocked("disconnected")
}

func (ls *LocalSession) notifyStateLocked(st string) {
	if ls.emitter != nil {
		ls.emitter.Emit("session:state", map[string]any{
			"tmpID": ls.tmpID,
			"state": st,
			"title": ls.title,
		})
	}
}

func (ls *LocalSession) Write(data []byte) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.state == "disconnected" {
		// If disconnected and Enter is pressed, reconnect!
		for _, b := range data {
			if b == '\r' || b == '\n' {
				return ls.startLocked()
			}
		}
		return nil
	}

	if ls.proc != nil {
		_, err := ls.proc.Write(data)
		return err
	}
	return nil
}

func (ls *LocalSession) Resize(cols, rows int) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if cols > 0 {
		ls.cols = cols
	}
	if rows > 0 {
		ls.rows = rows
	}

	if ls.proc != nil {
		return ls.proc.Resize(ls.cols, ls.rows)
	}
	return nil
}

func (ls *LocalSession) Ack(seq uint64) {
	ls.stream.Ack(seq)
}

func (ls *LocalSession) Disconnect() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.cancel != nil {
		ls.cancel()
	}
	if ls.proc != nil {
		_ = ls.proc.Close()
		ls.proc = nil
	}
	ls.state = "disconnected"
	ls.notifyStateLocked("disconnected")
	return nil
}

func (ls *LocalSession) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.state = "closed"
	if ls.cancel != nil {
		ls.cancel()
	}
	if ls.proc != nil {
		_ = ls.proc.Close()
		ls.proc = nil
	}
	ls.stream.Close()
	ls.notifyStateLocked("closed")
	return nil
}

func (ls *LocalSession) Restart() error {
	_ = ls.Disconnect()
	return ls.Start()
}
