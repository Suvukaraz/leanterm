package platform

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Quotes, pipes and && must reach the shell unchanged (cmd.exe on Windows).
func TestShellCommandPassesCommandLineThrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := `echo "a b" && echo second | sort`
	out, err := ShellCommand(ctx, command).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v (%s)", command, err, out)
	}
	text := strings.ReplaceAll(string(out), "\r", "")
	want := "a b\n"
	if runtime.GOOS == "windows" {
		want = "\"a b\" \n" // cmd's echo keeps the quotes and the space before &&
	}
	if !strings.HasPrefix(text, want) || !strings.Contains(text, "second") {
		t.Fatalf("output %q, want prefix %q and \"second\"", text, want)
	}
}

func TestShellCommandReportsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ShellCommand(ctx, "exit 3").Run(); err == nil {
		t.Fatal("exit 3 did not fail")
	}
}
