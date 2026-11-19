//go:build !windows

package session

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

// Signals that were ignored when LeanTerm started (e.g. SIGINT for a program
// started with & or nohup) stay ignored across exec, so Ctrl+C would not work
// in local shells. Catching them makes exec reset them to the default for
// the shells; LeanTerm itself keeps ignoring them.
func init() {
	for _, sig := range []os.Signal{syscall.SIGINT, syscall.SIGQUIT} {
		if signal.Ignored(sig) {
			c := make(chan os.Signal, 1)
			signal.Notify(c, sig)
			go func() {
				for range c {
				}
			}()
		}
	}
}

// prepareCmd makes the shell a session leader with the PTY as its controlling
// terminal, so job control and Ctrl+C reach it.
func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
}

// afterStart closes our copy of the PTY's child side: the shell has its own, and
// reads of the terminal only fail (ending the session) once it is gone.
func afterStart(p xpty.Pty) {
	if u, ok := p.(*xpty.UnixPty); ok {
		_ = u.Slave().Close()
	}
}

// killOnClose: see xptyShell.Close.
const killOnClose = false

func checkPtySupport() error { return nil }

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
