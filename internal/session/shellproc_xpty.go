package session

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/charmbracelet/x/xpty"
)

// xptyShell runs the shell in a pseudo terminal from x/xpty: a classic PTY on
// Unix, a ConPTY pseudo console on Windows (Windows 10 1809+).
type xptyShell struct {
	pty xpty.Pty
	cmd *exec.Cmd

	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
}

func startShellOS(ctx context.Context, spec shellSpec) (shellProcess, error) {
	if err := checkPtySupport(); err != nil {
		return nil, err
	}
	p, err := xpty.NewPty(clampDim(spec.Cols), clampDim(spec.Rows))
	if err != nil {
		return nil, fmt.Errorf("create pseudo terminal: %w", err)
	}

	path := spec.Path
	if resolved, err := exec.LookPath(path); err == nil {
		path = resolved
	}
	cmd := exec.Command(path, spec.Args...)
	cmd.Path = path
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	prepareCmd(cmd)
	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	afterStart(p)

	s := &xptyShell{pty: p, cmd: cmd}
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	return s, nil
}

func (s *xptyShell) Read(p []byte) (int, error)  { return s.pty.Read(p) }
func (s *xptyShell) Write(p []byte) (int, error) { return s.pty.Write(p) }
func (s *xptyShell) Pid() int                    { return s.cmd.Process.Pid }

func (s *xptyShell) Resize(cols, rows int) error {
	return s.pty.Resize(clampDim(cols), clampDim(rows))
}

// Wait blocks until the shell has exited (cmd.Wait does not work for ConPTY
// processes, so x/xpty waits on the process itself).
func (s *xptyShell) Wait() error {
	s.waitOnce.Do(func() {
		s.waitErr = xpty.WaitProcess(context.Background(), s.cmd)
	})
	return s.waitErr
}

// Close ends the shell and the pseudo terminal: on Unix closing the PTY hangs
// up the shell (SIGHUP), on Windows the process is terminated. Closing a pseudo
// console can wait until its output is read on older Windows versions, so that
// happens in the background (the read loop keeps draining it meanwhile).
func (s *xptyShell) Close() error {
	s.closeOnce.Do(func() {
		if killOnClose {
			_ = s.cmd.Process.Kill()
			go func() {
				_ = s.pty.Close()
				_ = s.Wait()
			}()
			return
		}
		_ = s.pty.Close()
	})
	return nil
}

func clampDim(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32767 {
		return 32767
	}
	return n
}
