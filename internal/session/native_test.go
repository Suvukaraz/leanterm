package session

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/events"
)

// A native GUI drives a local shell with Subscribe, WriteBytes and Resize and
// follows it through the event bus, without the web GUI's base64 events.
func TestNativeGUIDrivesLocalShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	store, err := config.NewStore(config.StoreOptions{ConfigDir: filepath.Join(home, "cfg")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}
	cmd, want := "echo native-$((40+2))\r", "native-42"
	if runtime.GOOS == "windows" {
		store.Config().LocalShell.Binary = "cmd.exe"
		cmd, want = "echo native-%OS%\r", "native-Windows_NT"
	} else {
		store.Config().LocalShell.Binary = "/bin/sh"
	}

	bus := events.New()
	var mu sync.Mutex
	var out strings.Builder
	var states, webPackets []string
	bus.On(events.SessionState, func(_ string, data any) {
		mu.Lock()
		defer mu.Unlock()
		if m, ok := data.(map[string]any); ok {
			states = append(states, m["state"].(string))
		}
	})
	bus.OnPrefix(events.TermDataPrefix, func(name string, _ any) {
		mu.Lock()
		defer mu.Unlock()
		webPackets = append(webPackets, name)
	})

	sm := NewSessionManager(store, nil, bus)
	tmpID, err := sm.OpenLocalShell("tab")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sm.Close(tmpID) })
	unsub, err := sm.Subscribe(tmpID, func(p []byte) {
		mu.Lock()
		out.Write(p)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()
	// Output before Subscribe may have gone out for the web GUI already.
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	webPackets = nil
	mu.Unlock()

	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			mu.Lock()
			ok := cond()
			mu.Unlock()
			if ok {
				return
			}
			if time.Now().After(deadline) {
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("no %s; output %q, states %v", what, out.String(), states)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	// The greeting was written before Subscribe: it comes from the replay.
	waitFor("greeting", func() bool { return strings.Contains(out.String(), "CONNECTING WITH") })
	waitFor("connected state", func() bool {
		for _, s := range states {
			if s == "connected" {
				return true
			}
		}
		return false
	})
	if err := sm.Resize(tmpID, 100, 30); err != nil {
		t.Fatal(err)
	}
	if err := sm.WriteBytes(tmpID, []byte(cmd)); err != nil {
		t.Fatal(err)
	}
	waitFor(want, func() bool { return strings.Count(out.String(), want) >= 1 })

	time.Sleep(100 * time.Millisecond) // a pending web batch would flush by now
	mu.Lock()
	defer mu.Unlock()
	if len(webPackets) > 0 {
		t.Fatalf("web GUI events sent while a native GUI reads: %v", webPackets)
	}
}
