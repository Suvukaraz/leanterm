package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

func TestStripEscapeSequences(t *testing.T) {
	colored := []byte("\x1b[31;1mERROR:\x1b[0m Connection failed\x07\r\n")
	cleaned := StripEscapeSequences(colored)
	expected := "ERROR: Connection failed\r\n"
	if string(cleaned) != expected {
		t.Errorf("got %q, want %q", string(cleaned), expected)
	}
}

func TestSessionLogger_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	opts := config.SessionLogsConfig{
		Enabled:           true,
		Folder:            tmpDir,
		Pattern:           "<NAME>_test.log",
		StripControlChars: true,
		Timestamps:        true,
	}

	logger, err := NewSessionLogger(opts, vars.SubstContext{Name: "myconn"}, tmpDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	logger.Write([]byte("\x1b[32mFirst line\x1b[0m\nSecond line\n"))
	_ = logger.Close()

	logFile := filepath.Join(tmpDir, "myconn_test.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "First line") || !strings.Contains(s, "Second line") {
		t.Errorf("missing lines in log: %s", s)
	}
	if strings.Contains(s, "\x1b[32m") {
		t.Errorf("ANSI sequences not stripped: %s", s)
	}
	// Check for timestamp format: [YYYY-MM-DD HH:MM:SS]
	if !strings.Contains(s, "[20") {
		t.Errorf("expected timestamp bracket in log: %s", s)
	}
}

func TestSessionLogger_Rotation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 old logs
	for i := 1; i <= 3; i++ {
		p := filepath.Join(tmpDir, "srv_log_"+string(rune('0'+i))+".log")
		_ = os.WriteFile(p, []byte("log"), 0600)
		time.Sleep(10 * time.Millisecond)
	}

	// Rotate with keep = 2
	rotateLogs(tmpDir, "srv", 2)

	entries, _ := os.ReadDir(tmpDir)
	if len(entries) > 2 {
		t.Errorf("expected at most 2 log files remaining, got %d", len(entries))
	}
}
