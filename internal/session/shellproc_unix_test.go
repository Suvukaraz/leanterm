//go:build !windows

package session

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Ctrl+C typed into the PTY must reach the shell as SIGINT, which needs the PTY
// to be its controlling terminal (job control works the same way).
func TestShellGetsInterruptFromPTY(t *testing.T) {
	spec := shellSpec{
		Path: "/bin/sh",
		Args: []string{"-c", `trap 'echo got-int; exit 0' INT; echo ready; while :; do sleep 0.1; done`},
		Env:  os.Environ(), Cols: 80, Rows: 24,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	proc, err := startShell(ctx, spec)
	if err != nil {
		t.Fatalf("startShell: %v", err)
	}
	defer proc.Close()

	out := make(chan string, 8)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := proc.Read(buf)
			if n > 0 {
				out <- string(buf[:n])
			}
			if err != nil {
				close(out)
				return
			}
		}
	}()

	var seen strings.Builder
	waitFor := func(want string) {
		t.Helper()
		for !strings.Contains(seen.String(), want) {
			select {
			case s, ok := <-out:
				if !ok {
					t.Fatalf("terminal closed before %q, output %q", want, seen.String())
				}
				seen.WriteString(s)
			case <-ctx.Done():
				t.Fatalf("no %q in output %q", want, seen.String())
			}
		}
	}
	waitFor("ready")
	if _, err := proc.Write([]byte{0x03}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	waitFor("got-int")
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

// A LeanTerm started with SIGINT ignored (with & from a script, nohup) must
// still give its shells a working Ctrl+C. The test runs itself again with
// SIGINT ignored, as such a start would.
func TestInterruptWorksWhenStartedWithSIGINTIgnored(t *testing.T) {
	if os.Getenv("LEANTERM_SIGINT_HELPER") == "1" {
		TestShellGetsInterruptFromPTY(t)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", `trap '' INT; exec "$0" -test.run '^TestInterruptWorksWhenStartedWithSIGINTIgnored$' -test.v`, exe)
	cmd.Env = append(os.Environ(), "LEANTERM_SIGINT_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS") {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}
}
