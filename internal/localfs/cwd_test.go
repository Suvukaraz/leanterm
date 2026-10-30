//go:build linux

package localfs

import (
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

type chanEmitter chan string

func (c chanEmitter) Emit(event string, data any) {
	if s, ok := data.(string); ok {
		c <- event + "=" + s
	}
}

func TestTrackerFollowsShellCwd(t *testing.T) {
	old := CwdPollInterval
	CwdPollInterval = 20 * time.Millisecond
	defer func() { CwdPollInterval = old }()

	first := t.TempDir()
	cmd := exec.Command("sleep", "5")
	cmd.Dir = first
	if err := cmd.Start(); err != nil {
		t.Skip("sleep not available:", err)
	}
	defer cmd.Process.Kill()

	var pid atomic.Int64
	pid.Store(int64(cmd.Process.Pid))
	events := make(chanEmitter, 4)
	tr := NewTracker(func(string) int { return int(pid.Load()) }, events)

	if cwd, _ := tr.Getwd("t1"); cwd != first {
		t.Fatalf("Getwd = %q, want %q", cwd, first)
	}

	tr.SetFollow("t1", true)
	defer tr.Stop("t1")

	// A "cd" in the shell is a new process cwd; simulate it with a second process.
	second := t.TempDir()
	cmd2 := exec.Command("sleep", "5")
	cmd2.Dir = second
	if err := cmd2.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd2.Process.Kill()
	pid.Store(int64(cmd2.Process.Pid))

	select {
	case ev := <-events:
		if ev != "session:cwd:t1="+second {
			t.Fatalf("event = %q", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no cwd event")
	}

	// Unknown shells fall back to the home folder.
	pid.Store(0)
	home, _ := os.UserHomeDir()
	if cwd, _ := tr.Getwd("t1"); cwd != home {
		t.Fatalf("Getwd fallback = %q, want %q", cwd, home)
	}
}
