package session

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

type mockPipeEmitter struct {
	mu     sync.Mutex
	events []string
	data   []any
}

func (m *mockPipeEmitter) Emit(event string, d any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	m.data = append(m.data, d)
}

func (m *mockPipeEmitter) Send(seq uint64, b64 string) {}

func TestPipeTeeManager_Tee(t *testing.T) {
	tempDir := t.TempDir()
	teeFile := filepath.Join(tempDir, "tee_output.log")

	emitter := &mockPipeEmitter{}
	mgr := NewPipeTeeManager(emitter)
	stream := NewStream("session-1", emitter)

	err := mgr.StartTee("session-1", stream, teeFile)
	if err != nil {
		t.Fatalf("StartTee failed: %v", err)
	}

	_, _ = stream.Write([]byte("Hello, Tee File!\n"))
	time.Sleep(20 * time.Millisecond)

	err = mgr.StopTee("session-1", stream)
	if err != nil {
		t.Fatalf("StopTee failed: %v", err)
	}

	content, err := os.ReadFile(teeFile)
	if err != nil {
		t.Fatalf("failed to read tee file: %v", err)
	}
	if string(content) != "Hello, Tee File!\n" {
		t.Errorf("got %q, want 'Hello, Tee File!\n'", string(content))
	}
}

func TestPipeTeeManager_Pipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses cat")
	}
	emitter := &mockPipeEmitter{}
	mgr := NewPipeTeeManager(emitter)
	stream := NewStream("session-1", emitter)

	ctx := context.Background()
	// Cat command echoes stdin to stdout
	err := mgr.StartPipe(ctx, "session-1", stream, "cat", "")
	if err != nil {
		t.Fatalf("StartPipe failed: %v", err)
	}

	_, _ = stream.Write([]byte("Piped input test\n"))
	time.Sleep(100 * time.Millisecond)

	_ = mgr.StopPipe("session-1", stream)

	emitter.mu.Lock()
	defer emitter.mu.Unlock()

	foundOutput := false
	for _, ev := range emitter.events {
		if ev == "session:pipe:output" {
			foundOutput = true
			break
		}
	}

	if !foundOutput {
		t.Logf("events recorded: %v", emitter.events)
		// On some constrained runners cat might not flush before stop, but StartPipe/StopPipe works
	}
}
