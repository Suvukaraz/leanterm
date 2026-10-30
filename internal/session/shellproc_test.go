package session

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Starts a real shell in a pseudo terminal (PTY on Unix, ConPTY on Windows) and
// checks output, resizing and exit.
func TestStartShellRunsCommand(t *testing.T) {
	spec := shellSpec{Path: "/bin/sh", Args: []string{"-c", "echo leanterm-ok"}, Env: os.Environ(), Cols: 100, Rows: 30}
	if runtime.GOOS == "windows" {
		spec.Path = "cmd.exe"
		spec.Args = []string{"/c", "echo leanterm-ok"}
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = os.TempDir()
	}
	spec.Dir = dir

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	proc, err := startShell(ctx, spec)
	if err != nil {
		t.Fatalf("startShell: %v", err)
	}
	defer proc.Close()
	if proc.Pid() <= 0 {
		t.Fatalf("Pid() = %d", proc.Pid())
	}
	if err := proc.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	output := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for !strings.Contains(sb.String(), "leanterm-ok") {
			n, err := proc.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		output <- sb.String()
	}()

	select {
	case out := <-output:
		if !strings.Contains(out, "leanterm-ok") {
			t.Fatalf("output %q does not contain the echoed text", out)
		}
	case <-ctx.Done():
		t.Fatal("no output from the shell")
	}

	done := make(chan error, 1)
	go func() { done <- proc.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("shell did not exit")
	}
}

func TestDefaultShellIsSet(t *testing.T) {
	if defaultShell() == "" {
		t.Fatal("defaultShell() is empty")
	}
}
