//go:build !windows

package session

import (
	"context"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixShell struct {
	cmd *exec.Cmd
	pty *os.File
}

func startShellOS(ctx context.Context, spec shellSpec) (shellProcess, error) {
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(spec.Rows), Cols: uint16(spec.Cols)})
	if err != nil {
		return nil, err
	}
	return &unixShell{cmd: cmd, pty: f}, nil
}

func (s *unixShell) Read(p []byte) (int, error)  { return s.pty.Read(p) }
func (s *unixShell) Write(p []byte) (int, error) { return s.pty.Write(p) }
func (s *unixShell) Close() error                { return s.pty.Close() }
func (s *unixShell) Wait() error                 { return s.cmd.Wait() }
func (s *unixShell) Pid() int                    { return s.cmd.Process.Pid }

func (s *unixShell) Resize(cols, rows int) error {
	return pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// defaultShell is used when no shell is configured: $SHELL, else bash.
func defaultShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}

// withCwdReporting is a no-op on Unix: the folder is read from /proc instead.
func withCwdReporting(_ string, args, env []string) ([]string, []string) {
	return args, env
}
