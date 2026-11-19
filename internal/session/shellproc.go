package session

import (
	"context"
	"io"
)

// shellProcess is a local shell running in a pseudo terminal: a PTY on Unix,
// a ConPTY pseudo console on Windows.
type shellProcess interface {
	// Read returns the terminal output; it fails once the terminal is closed.
	io.Reader
	// Write sends keyboard input to the shell.
	io.Writer
	// Close closes the terminal, which ends the shell.
	io.Closer
	Resize(cols, rows int) error
	// Wait blocks until the shell has exited.
	Wait() error
	Pid() int
}

// shellSpec describes the shell to start.
type shellSpec struct {
	Path string
	Args []string
	Dir  string
	Env  []string
	Cols int
	Rows int
}

// startShell starts spec in a new pseudo terminal; cancelling ctx ends the shell.
func startShell(ctx context.Context, spec shellSpec) (shellProcess, error) {
	return startShellOS(ctx, spec)
}

// ShellProcess is a shell running in a pseudo terminal (see shellProcess).
type ShellProcess = shellProcess

// ShellSpec describes a shell to start with [StartShell].
type ShellSpec = shellSpec

// StartShell starts spec in a new pseudo terminal; cancelling ctx ends the shell.
func StartShell(ctx context.Context, spec ShellSpec) (ShellProcess, error) {
	return startShell(ctx, spec)
}

// DefaultShell is the shell used when none is configured.
func DefaultShell() string { return defaultShell() }

// WithCwdReporting adds the options that make a shell report its working
// directory, where LeanTerm knows how (PowerShell and cmd on Windows).
func WithCwdReporting(shell string, args, env []string) ([]string, []string) {
	return withCwdReporting(shell, args, env)
}
