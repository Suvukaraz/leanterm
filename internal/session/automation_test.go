package session

import (
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/config"
)

type nopEmitter struct{}

func (nopEmitter) Emit(string, any) {}

func TestAutomationRunsExpectAndWritesSessionLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	t.Setenv("SSH_AUTH_SOCK", "")

	addr, _ := testSSHServer(t, "pw")
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	store, err := config.NewStore(config.StoreOptions{ConfigDir: filepath.Join(home, "cfg")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := store.Config()
	cfg.General.AutoAcceptHostKeys = true
	cfg.Connections = append(cfg.Connections, config.Connection{
		ID: "auto1", Name: "auto", Title: "auto", Method: "ssh", Host: host, Port: port,
		Auth: config.ConnectionAuth{Type: "password", User: "alice", Exclusive: true},
		Expect: []config.ExpectEntry{
			{ID: "e1", Enabled: true, Pattern: "welcome", Send: "automated-reply", AppendReturn: true, OnMatch: "next", OnFail: "stop"},
		},
		SessionLogs: config.SessionLogsConfig{Enabled: true, Pattern: "<NAME>.log", Keep: 0},
	})

	sm := NewSessionManager(store, nil, nopEmitter{})
	sm.SetPrompter(&fakePrompter{reply: func(req PromptRequest) PromptResponse {
		return PromptResponse{Choice: "ok", Values: map[string]string{"password": "pw"}}
	}})

	ids, err := sm.Open(OpenRequest{ConnIDs: []string{"auto1"}})
	if err != nil || len(ids) != 1 {
		t.Fatalf("open: %v %v", ids, err)
	}
	tmpID := ids[0]
	t.Cleanup(func() { _ = sm.Close(tmpID) })

	// The expect rule answers the server's "welcome" banner; the echo server
	// sends our reply back, so it shows up in the terminal output.
	waitFor(t, func() bool {
		replay, _, err := sm.Attach(tmpID)
		if err != nil {
			return false
		}
		data, _ := base64.StdEncoding.DecodeString(replay)
		return strings.Contains(string(data), "automated-reply")
	})

	logPath := filepath.Join(home, "cfg", "session_logs", "auto.log")
	waitFor(t, func() bool {
		data, err := os.ReadFile(logPath)
		return err == nil && strings.Contains(string(data), "welcome")
	})
}
