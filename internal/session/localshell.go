package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
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
	cmd       *exec.Cmd
	ptyFile   *os.File
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
	if ls.cmd == nil || ls.cmd.Process == nil || ls.state != "connected" {
		return 0
	}
	return ls.cmd.Process.Pid
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
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
	}

	// Prepare command
	var args []string
	if ls.cfg.Options != "" {
		args = strings.Fields(ls.cfg.Options)
	}

	cmd := exec.CommandContext(ls.ctx, shell, args...)

	// Working directory
	dir := ls.cfg.Directory
	if dir == "" {
		dir = os.Getenv("HOME")
		if dir == "" {
			dir = "."
		}
	}
	cmd.Dir = dir
	cmd.Env = os.Environ()

	// Initial winsize
	ws := &pty.Winsize{
		Rows: uint16(ls.rows),
		Cols: uint16(ls.cols),
	}

	ptyFile, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		ls.state = "disconnected"
		ls.notifyStateLocked("disconnected")
		errMsg := fmt.Sprintf("\r\n\x1b[31mERROR: Failed to launch shell '%s': %v\x1b[0m\r\n", shell, err)
		_, _ = ls.stream.Write([]byte(errMsg))
		return err
	}

	ls.cmd = cmd
	ls.ptyFile = ptyFile
	ls.state = "connected"
	ls.notifyStateLocked("connected")

	// Goroutine to read from PTY into Stream
	go ls.readLoop(ptyFile)

	// Goroutine to wait for process exit
	go ls.waitLoop(cmd)

	return nil
}

func (ls *LocalSession) readLoop(ptyFile *os.File) {
	buf := make([]byte, 32*1024)
	for {
		n, err := ptyFile.Read(buf)
		if n > 0 {
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

func (ls *LocalSession) waitLoop(cmd *exec.Cmd) {
	_ = cmd.Wait()

	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.state == "closed" {
		return
	}

	ls.state = "disconnected"
	if ls.ptyFile != nil {
		_ = ls.ptyFile.Close()
		ls.ptyFile = nil
	}

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

	if ls.ptyFile != nil {
		_, err := ls.ptyFile.Write(data)
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

	if ls.ptyFile != nil {
		return pty.Setsize(ls.ptyFile, &pty.Winsize{
			Rows: uint16(ls.rows),
			Cols: uint16(ls.cols),
		})
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
	if ls.ptyFile != nil {
		_ = ls.ptyFile.Close()
		ls.ptyFile = nil
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
	if ls.ptyFile != nil {
		_ = ls.ptyFile.Close()
		ls.ptyFile = nil
	}
	ls.stream.Close()
	ls.notifyStateLocked("closed")
	return nil
}

func (ls *LocalSession) Restart() error {
	_ = ls.Disconnect()
	return ls.Start()
}
