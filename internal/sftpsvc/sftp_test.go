package sftpsvc

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewSFTPManager(t *testing.T) {
	mgr := NewSFTPManager()
	if mgr == nil {
		t.Fatal("expected non-nil SFTPManager")
	}

	// Calling OpenForSession with nil sshClient should fail
	_, err := mgr.OpenForSession("test-tmp-1", nil)
	if err == nil {
		t.Fatal("expected error when sshClient is nil")
	}

	// Calling CloseForSession on nonexistent session should succeed (no-op)
	if err := mgr.CloseForSession("nonexistent"); err != nil {
		t.Fatalf("unexpected error closing nonexistent session: %v", err)
	}

	// Operations on nonexistent sftpID should fail gracefully
	if _, err := mgr.Getwd("invalid-id"); err == nil {
		t.Error("expected error for Getwd on invalid-id")
	}

	if _, err := mgr.List("invalid-id", "/", false); err == nil {
		t.Error("expected error for List on invalid-id")
	}

	if _, err := mgr.Stat("invalid-id", "/file"); err == nil {
		t.Error("expected error for Stat on invalid-id")
	}

	if _, err := mgr.StatVFS("invalid-id", "/"); err == nil {
		t.Error("expected error for StatVFS on invalid-id")
	}

	if err := mgr.Mkdir("invalid-id", "/dir", true); err == nil {
		t.Error("expected error for Mkdir on invalid-id")
	}

	if err := mgr.CreateFile("invalid-id", "/file"); err == nil {
		t.Error("expected error for CreateFile on invalid-id")
	}

	if err := mgr.Remove("invalid-id", []string{"/file"}, false); err == nil {
		t.Error("expected error for Remove on invalid-id")
	}

	if err := mgr.Rename("invalid-id", "/a", "/b"); err == nil {
		t.Error("expected error for Rename on invalid-id")
	}

	if err := mgr.Chmod("invalid-id", []string{"/a"}, 0755, false, "all"); err == nil {
		t.Error("expected error for Chmod on invalid-id")
	}

	if err := mgr.Download("invalid-id", []string{"/a"}, "/tmp"); err == nil {
		t.Error("expected error for Download on invalid-id")
	}

	if err := mgr.Upload("invalid-id", []string{"/tmp/test"}, "/"); err == nil {
		t.Error("expected error for Upload on invalid-id")
	}
}

func TestWatchForEditsDebouncesAndSeesRenameSaves(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(local, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := 0
	watchForEdits(dir, local, func() {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	time.Sleep(50 * time.Millisecond)

	// Several writes of one save, then an editor-style save via temp file + rename.
	for i := 0; i < 5; i++ {
		_ = os.WriteFile(local, []byte("v2"), 0o644)
	}
	tmp := filepath.Join(dir, ".notes.txt.swp")
	_ = os.WriteFile(tmp, []byte("v3"), 0o644)
	_ = os.Rename(tmp, local)

	time.Sleep(1500 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected exactly one save notification, got %d", calls)
	}
}
