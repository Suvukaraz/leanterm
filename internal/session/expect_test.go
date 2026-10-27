package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

func TestExpectRunner_Sequential(t *testing.T) {
	var mu sync.Mutex
	var sent []string
	var statuses []string

	entries := []config.ExpectEntry{
		{
			Enabled:      true,
			Pattern:      `login:\s*$`,
			Send:         "admin",
			AppendReturn: true,
			OnMatch:      "next",
			Timeout:      2,
		},
		{
			Enabled:      true,
			Pattern:      `Password:\s*$`,
			Send:         "secret123",
			AppendReturn: true,
			OnMatch:      "next",
			Timeout:      2,
		},
		{
			Enabled:      true,
			Pattern:      `\$ $`,
			Send:         "echo ready",
			AppendReturn: true,
			OnMatch:      "stop",
			Timeout:      2,
		},
	}

	runner := NewExpectRunner(ExpectOptions{
		Entries: entries,
		SendFunc: func(b []byte) error {
			mu.Lock()
			defer mu.Unlock()
			sent = append(sent, string(b))
			return nil
		},
		StatusFunc: func(st string) {
			mu.Lock()
			defer mu.Unlock()
			statuses = append(statuses, st)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	// Feed "login: "
	time.Sleep(20 * time.Millisecond)
	runner.Feed([]byte("host login: "))

	time.Sleep(50 * time.Millisecond)
	// Feed "Password: "
	runner.Feed([]byte("Password: "))

	time.Sleep(50 * time.Millisecond)
	// Feed prompt "$ "
	runner.Feed([]byte("user@host:~$ "))

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expect runner timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(sent) != 3 {
		t.Fatalf("expected 3 sent strings, got %d: %v", len(sent), sent)
	}
	if sent[0] != "admin\n" {
		t.Errorf("sent[0] = %q, want %q", sent[0], "admin\n")
	}
	if sent[1] != "secret123\n" {
		t.Errorf("sent[1] = %q, want %q", sent[1], "secret123\n")
	}
	if sent[2] != "echo ready\n" {
		t.Errorf("sent[2] = %q, want %q", sent[2], "echo ready\n")
	}
}

func TestExpectRunner_ImmediateSend(t *testing.T) {
	var sent []string
	entries := []config.ExpectEntry{
		{
			Enabled:      true,
			Pattern:      "", // empty pattern -> send immediately
			Send:         "stty sane",
			AppendReturn: true,
			OnMatch:      "next",
		},
	}

	runner := NewExpectRunner(ExpectOptions{
		Entries: entries,
		SendFunc: func(b []byte) error {
			sent = append(sent, string(b))
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	runner.Run(ctx)

	if len(sent) != 1 || sent[0] != "stty sane\n" {
		t.Fatalf("immediate send failed, sent = %v", sent)
	}
}

func TestExpectRunner_GotoAndLoopGuard(t *testing.T) {
	var statusList []string
	var mu sync.Mutex

	// Rule 1 jumps to Rule 2, Rule 2 jumps to Rule 1 -> infinite loop guard should stop it at 1000 jumps
	entries := []config.ExpectEntry{
		{
			Enabled:      true,
			Pattern:      "",
			Send:         "ping",
			OnMatch:      "goto",
			OnMatchGoto:  2,
		},
		{
			Enabled:      true,
			Pattern:      "",
			Send:         "pong",
			OnMatch:      "goto",
			OnMatchGoto:  1,
		},
	}

	runner := NewExpectRunner(ExpectOptions{
		Entries: entries,
		SendFunc: func(b []byte) error { return nil },
		StatusFunc: func(st string) {
			mu.Lock()
			defer mu.Unlock()
			statusList = append(statusList, st)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	runner.Run(ctx)

	mu.Lock()
	defer mu.Unlock()

	foundGuard := false
	for _, s := range statusList {
		if strings.Contains(s, "Loop guard exceeded 1000 jumps") {
			foundGuard = true
			break
		}
	}
	if !foundGuard {
		t.Errorf("expected 1000 jumps loop guard to trigger, got statuses: %v", statusList)
	}
}

func TestExpectRunner_TimeoutAndClose(t *testing.T) {
	var closedReason string
	entries := []config.ExpectEntry{
		{
			Enabled: true,
			Pattern: "NeverMatchedString",
			Timeout: 1, // 1 second timeout
			OnFail:  "close",
		},
	}

	runner := NewExpectRunner(ExpectOptions{
		Entries: entries,
		CloseFunc: func(reason string) {
			closedReason = reason
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	runner.Run(ctx)

	if !strings.Contains(closedReason, "TIMEOUT:1 seconds expecting pattern 'NeverMatchedString'!!") {
		t.Errorf("unexpected close reason: %q", closedReason)
	}
}

func TestExpectRunner_VaultHidden(t *testing.T) {
	var mu sync.Mutex
	var sent []string
	var statuses []string

	vault := secrets.NewVault(secrets.VaultOptions{
		ConfigDir: t.TempDir(),
	})
	vault.SetConnection("conn-1", "exp_sec", "MyVaultPassword")

	entries := []config.ExpectEntry{
		{
			ID:           "exp_sec",
			Enabled:      true,
			Pattern:      `Password:`,
			Hidden:       true,
			AppendReturn: true,
			OnMatch:      "stop",
			Timeout:      2,
		},
	}

	runner := NewExpectRunner(ExpectOptions{
		ConnID:  "conn-1",
		Entries: entries,
		Vault:   vault,
		SendFunc: func(b []byte) error {
			mu.Lock()
			defer mu.Unlock()
			sent = append(sent, string(b))
			return nil
		},
		StatusFunc: func(st string) {
			mu.Lock()
			defer mu.Unlock()
			statuses = append(statuses, st)
		},
		SubstCtx: vars.SubstContext{
			ConnID: "conn-1",
			Vault:  vault,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	runner.Feed([]byte("Password:"))

	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(sent) != 1 || sent[0] != "MyVaultPassword\n" {
		t.Fatalf("expected hidden vault password sent, got: %v", sent)
	}

	for _, st := range statuses {
		if strings.Contains(st, "MyVaultPassword") {
			t.Errorf("status leaked secret: %s", st)
		}
	}
}
