package termview

import (
	"context"
	"os"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// Local is a local shell connected to a terminal engine.
type Local struct {
	Engine term.Engine
	proc   session.ShellProcess
	cancel context.CancelFunc

	mu      sync.Mutex
	title   string
	dir     string
	exited  bool
	changed func()
}

// StartLocal starts the default shell in engine (""= default engine).
// changed is called (on any goroutine) whenever the terminal needs a redraw.
func StartLocal(engine string, changed func()) (*Local, error) {
	ctx, cancel := context.WithCancel(context.Background())
	shell := session.DefaultShell()
	env := append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=LeanTerm")
	args, env := session.WithCwdReporting(shell, nil, env)
	home, _ := os.UserHomeDir()
	proc, err := session.StartShell(ctx, session.ShellSpec{Path: shell, Args: args, Env: env, Dir: home, Cols: 80, Rows: 24})
	if err != nil {
		cancel()
		return nil, err
	}
	l := &Local{proc: proc, cancel: cancel, changed: changed}
	eng, err := term.New(engine, term.Options{Cols: 80, Rows: 24, Callbacks: term.Callbacks{
		Output: func(p []byte) { _, _ = proc.Write(p) },
		Title:  func(s string) { l.mu.Lock(); l.title = s; l.mu.Unlock(); changed() },
		Dir:    func(s string) { l.mu.Lock(); l.dir = s; l.mu.Unlock(); changed() },
	}})
	if err != nil {
		cancel()
		_ = proc.Close()
		return nil, err
	}
	l.Engine = eng
	go l.read()
	return l, nil
}

func (l *Local) read() {
	buf := make([]byte, 64<<10)
	for {
		n, err := l.proc.Read(buf)
		if n > 0 {
			l.Engine.Feed(buf[:n])
			l.changed()
		}
		if err != nil {
			break
		}
	}
	l.mu.Lock()
	l.exited = true
	l.mu.Unlock()
	l.changed()
}

// Resize resizes the PTY (the view resizes the engine).
func (l *Local) Resize(cols, rows int) { _ = l.proc.Resize(cols, rows) }

// Title, Dir and Exited report the state of the shell.
func (l *Local) Title() string { l.mu.Lock(); defer l.mu.Unlock(); return l.title }
func (l *Local) Dir() string   { l.mu.Lock(); defer l.mu.Unlock(); return l.dir }
func (l *Local) Exited() bool  { l.mu.Lock(); defer l.mu.Unlock(); return l.exited }

// Close ends the shell and the engine.
func (l *Local) Close() {
	l.cancel()
	_ = l.proc.Close()
	l.Engine.Close()
}
