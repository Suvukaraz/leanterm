package keepass

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
)

func TestKeePassClient_GetField_WithCache(t *testing.T) {
	var callCount int32

	mockRunner := func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
		atomic.AddInt32(&callCount, 1)

		if !strings.Contains(stdin, "master123") {
			t.Errorf("expected master password in stdin, got %q", stdin)
		}

		// Check args
		// Expect: show -s -k key.key db.kdbx Root/web1 -a Password
		foundAttr := false
		for i, a := range args {
			if a == "-a" && i+1 < len(args) {
				foundAttr = true
				if args[i+1] == "Password" {
					return []byte("supersecret123\n"), nil
				}
				if args[i+1] == "UserName" {
					return []byte("admin\n"), nil
				}
			}
		}
		if !foundAttr {
			t.Errorf("missing -a attribute in args: %v", args)
		}
		return []byte("val"), nil
	}

	client := NewClient(Options{
		Config: config.KeePassConfig{
			Enabled:  true,
			Database: "/path/to/db.kdbx",
			KeyFile:  "/path/to/key.key",
		},
		MasterPassword: "master123",
		TTL:            1 * time.Minute,
		Runner:         mockRunner,
	})

	ctx := context.Background()

	// 1. First fetch: should invoke runner
	val, err := client.GetField(ctx, "Root/web1", "Password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "supersecret123" {
		t.Errorf("got %q, want supersecret123", val)
	}
	if count := atomic.LoadInt32(&callCount); count != 1 {
		t.Errorf("expected 1 call, got %d", count)
	}

	// 2. Second fetch: should hit cache, NOT invoke runner
	valCached, err := client.GetField(ctx, "Root/web1", "Password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valCached != "supersecret123" {
		t.Errorf("got %q, want supersecret123", valCached)
	}
	if count := atomic.LoadInt32(&callCount); count != 1 {
		t.Errorf("expected callCount to remain 1 after cache hit, got %d", count)
	}

	// 3. Different field: should invoke runner
	valUser, err := client.GetField(ctx, "Root/web1", "UserName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valUser != "admin" {
		t.Errorf("got %q, want admin", valUser)
	}
	if count := atomic.LoadInt32(&callCount); count != 2 {
		t.Errorf("expected 2 calls, got %d", count)
	}
}

func TestKeePassClient_SubstKeePass(t *testing.T) {
	mockRunner := func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "-a" && i+1 < len(args) {
				if args[i+1] == "UserName" {
					return []byte("alice\n"), nil
				}
				if args[i+1] == "Password" {
					return []byte("secret456\n"), nil
				}
			}
		}
		return []byte("default"), nil
	}

	client := NewClient(Options{
		Config: config.KeePassConfig{
			Enabled:  true,
			Database: "/data/vault.kdbx",
		},
		MasterPassword: "pwd",
		Runner:         mockRunner,
	})

	ctx := context.Background()
	input := "ssh <UserName|Servers/srv1>@example.com -p <Password|Servers/srv1>"
	out, hasHidden, err := client.SubstKeePass(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ssh alice@example.com -p secret456"
	if out != want {
		t.Errorf("SubstKeePass() = %q, want %q", out, want)
	}
	if !hasHidden {
		t.Errorf("expected hasHidden to be true due to Password field")
	}
}
