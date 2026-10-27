package session

import (
	"strings"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
)

func TestPromptMatcherLoginFlow(t *testing.T) {
	var sentText []string
	var statuses []string
	connected := false

	opts := PromptMatcherOptions{
		AuthType:   "password",
		Username:   "admin",
		Password:   "secret123",
		LineEnding: "\r\n",
		Prompts:    config.PromptsConfig{},
		TimeoutSec: 5,
		OnSend: func(text string) {
			sentText = append(sentText, text)
		},
		OnStatus: func(code string) {
			statuses = append(statuses, code)
		},
		OnConnected: func(note string) {
			connected = true
		},
	}

	pm := NewPromptMatcher(opts)
	defer pm.Stop()

	// 1. Feed username prompt: "Ubuntu 22.04 LTS\r\nlogin: "
	pm.FeedOutput([]byte("Ubuntu 22.04 LTS\r\nlogin: "))

	if len(sentText) != 1 || sentText[0] != "admin\r\n" {
		t.Fatalf("expected admin sent, got: %v", sentText)
	}
	if len(statuses) != 1 || statuses[0] != "LOGIN:admin" {
		t.Fatalf("expected LOGIN:admin status, got: %v", statuses)
	}

	// 2. Feed password prompt: "Password: "
	pm.FeedOutput([]byte("Password: "))

	if len(sentText) != 2 || sentText[1] != "secret123\r\n" {
		t.Fatalf("expected password sent, got: %v", sentText)
	}
	if len(statuses) != 2 || statuses[1] != "PASSWORD:Sent (not shown)" {
		t.Fatalf("expected PASSWORD:Sent status, got: %v", statuses)
	}

	// 3. Feed command prompt: "admin@server:~$ "
	pm.FeedOutput([]byte("admin@server:~$ "))

	if !connected {
		t.Fatal("expected connected to be true")
	}
}

func TestPromptMatcherEmptyUser(t *testing.T) {
	var errCode string
	opts := PromptMatcherOptions{
		AuthType: "password",
		Username: "", // empty user
		Password: "password",
		OnError: func(code string) {
			errCode = code
		},
	}

	pm := NewPromptMatcher(opts)
	defer pm.Stop()

	pm.FeedOutput([]byte("login: "))

	if !strings.Contains(errCode, "Empty username is not valid") {
		t.Fatalf("expected empty username error, got: %s", errCode)
	}
}

func TestPromptMatcherRejectedPassword(t *testing.T) {
	var statuses []string

	opts := PromptMatcherOptions{
		AuthType: "password",
		Username: "admin",
		Password: "wrong_password",
		OnStatus: func(code string) {
			statuses = append(statuses, code)
		},
	}

	pm := NewPromptMatcher(opts)
	defer pm.Stop()

	// Step 1: User prompt
	pm.FeedOutput([]byte("login: "))

	// Step 2: Password prompt
	pm.FeedOutput([]byte("Password: "))

	// Step 3: Rejected, second password prompt
	pm.FeedOutput([]byte("\r\nLogin incorrect\r\nPassword: "))

	foundRejected := false
	for _, st := range statuses {
		if strings.Contains(st, "was rejected") {
			foundRejected = true
			break
		}
	}
	if !foundRejected {
		t.Fatalf("expected rejected status, got statuses: %v", statuses)
	}
}

func TestPromptMatcherTimeout(t *testing.T) {
	noteCh := make(chan string, 1)
	opts := PromptMatcherOptions{
		AuthType:   "password",
		Username:   "admin",
		Password:   "secret",
		TimeoutSec: 1, // 1 second timeout
		OnConnected: func(note string) {
			select {
			case noteCh <- note:
			default:
			}
		},
	}

	pm := NewPromptMatcher(opts)
	defer pm.Stop()

	select {
	case note := <-noteCh:
		if note != "no prompt detected" {
			t.Fatalf("expected 'no prompt detected', got: %s", note)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("matcher did not report a connection after the timeout")
	}
}
