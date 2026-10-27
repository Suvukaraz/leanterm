package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

func TestExecutePrePost_DefaultSilent(t *testing.T) {
	stream := NewStream("test-prepost", nil)
	var statuses []string

	entries := []config.PrePostExec{
		{
			Command: "echo 'hello from pre-exec'",
			Ask:     false,
			Default: true,
		},
		{
			Command: "echo 'ignored because not default'",
			Ask:     false,
			Default: false,
		},
	}

	err := ExecutePrePost(context.Background(), PrePostOptions{
		Phase:   "PRE_EXEC",
		Entries: entries,
		Stream:  stream,
		StatusFunc: func(s string) {
			statuses = append(statuses, s)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := stream.ring.Bytes()
	out := string(buf)
	if !strings.Contains(out, "hello from pre-exec") {
		t.Errorf("expected pre-exec output in ring buffer, got: %q", out)
	}
	if strings.Contains(out, "ignored because not default") {
		t.Errorf("did not expect ignored command to run")
	}
}

func TestExecutePrePost_AskDialogSkip(t *testing.T) {
	stream := NewStream("test-prepost-skip", nil)
	var statuses []string

	entries := []config.PrePostExec{
		{
			Command: "echo 'should be skipped'",
			Ask:     true,
			Default: true,
		},
	}

	err := ExecutePrePost(context.Background(), PrePostOptions{
		Phase:   "PRE_EXEC",
		Entries: entries,
		Stream:  stream,
		AskHandler: func(phase string, items []config.PrePostExec) ([]config.PrePostExec, bool) {
			return nil, true // Skip!
		},
		StatusFunc: func(s string) {
			statuses = append(statuses, s)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := stream.ring.Bytes()
	if len(buf) > 0 {
		t.Errorf("expected empty buffer when skipped, got: %q", string(buf))
	}

	foundSkipped := false
	for _, s := range statuses {
		if strings.Contains(s, "Skipped by user") {
			foundSkipped = true
			break
		}
	}
	if !foundSkipped {
		t.Errorf("expected 'Skipped by user' status, got %v", statuses)
	}
}

func TestExecutePrePost_VariableSubst(t *testing.T) {
	stream := NewStream("test-prepost-vars", nil)

	entries := []config.PrePostExec{
		{
			Command: "echo 'User: <USER> Host: <IP>'",
			Ask:     false,
			Default: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ExecutePrePost(ctx, PrePostOptions{
		Phase:   "PRE_EXEC",
		Entries: entries,
		SubstCtx: vars.SubstContext{
			User: "sven",
			IP:   "192.168.1.100",
		},
		Stream: stream,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(stream.ring.Bytes())
	if !strings.Contains(out, "User: sven Host: 192.168.1.100") {
		t.Errorf("variable substitution did not replace user/ip, output: %q", out)
	}
}
