package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/platform"
)

// ActiveTee holds state for an active file tee on a session stream.
type ActiveTee struct {
	FilePath   string
	File       *os.File
	ListenerID int
}

// ActivePipe holds state for an active external command pipe.
type ActivePipe struct {
	Command    string
	Cmd        *exec.Cmd
	Stdin      io.WriteCloser
	ListenerID int
	Cancel     context.CancelFunc
}

// PipeTeeManager manages TEE file copies and PIPE processes for terminal sessions.
type PipeTeeManager struct {
	mu     sync.Mutex
	tees   map[string]*ActiveTee  // key: tmpID
	pipes  map[string]*ActivePipe // key: tmpID
	em     EventEmitter
}

// NewPipeTeeManager creates a new PipeTeeManager.
func NewPipeTeeManager(emitter EventEmitter) *PipeTeeManager {
	return &PipeTeeManager{
		tees:  make(map[string]*ActiveTee),
		pipes: make(map[string]*ActivePipe),
		em:    emitter,
	}
}

// StartTee mirrors incoming session stream bytes to a local file.
func (m *PipeTeeManager) StartTee(tmpID string, stream *Stream, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing tee on this session if any
	if existing, ok := m.tees[tmpID]; ok {
		stream.RemoveListener(existing.ListenerID)
		_ = existing.File.Close()
		delete(m.tees, tmpID)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tee file %s: %w", filePath, err)
	}

	listenerID := stream.AddListener(func(data []byte) {
		_, _ = f.Write(data)
	})

	m.tees[tmpID] = &ActiveTee{
		FilePath:   filePath,
		File:       f,
		ListenerID: listenerID,
	}

	if m.em != nil {
		m.em.Emit("session:tee:status", map[string]any{
			"tmpID":    tmpID,
			"filePath": filePath,
			"active":   true,
		})
	}

	return nil
}

// StopTee stops mirroring bytes to the local file for the given session.
func (m *PipeTeeManager) StopTee(tmpID string, stream *Stream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tee, ok := m.tees[tmpID]
	if !ok {
		return nil
	}

	stream.RemoveListener(tee.ListenerID)
	_ = tee.File.Close()
	delete(m.tees, tmpID)

	if m.em != nil {
		m.em.Emit("session:tee:status", map[string]any{
			"tmpID":  tmpID,
			"active": false,
		})
	}

	return nil
}

// StartPipe executes an external command, feeds the session stream to its stdin,
// and streams its stdout/stderr to "session:pipe:output" events.
func (m *PipeTeeManager) StartPipe(ctx context.Context, tmpID string, stream *Stream, command string, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing pipe if any
	if existing, ok := m.pipes[tmpID]; ok {
		stream.RemoveListener(existing.ListenerID)
		existing.Cancel()
		delete(m.pipes, tmpID)
	}

	pipeCtx, cancel := context.WithCancel(ctx)
	cmd := platform.ShellCommand(pipeCtx, command)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to open stdin pipe for command %q: %w", command, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		cancel()
		return fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		cancel()
		return fmt.Errorf("failed to start pipe command %q: %w", command, err)
	}

	listenerID := stream.AddListener(func(data []byte) {
		_, _ = stdin.Write(data)
	})

	active := &ActivePipe{
		Command:    command,
		Cmd:        cmd,
		Stdin:      stdin,
		ListenerID: listenerID,
		Cancel:     cancel,
	}
	m.pipes[tmpID] = active

	// Notify frontend that piped execution started
	if m.em != nil {
		m.em.Emit("session:pipe:started", map[string]any{
			"tmpID":   tmpID,
			"command": command,
			"prompt":  prompt,
		})
	}

	// Read output in background
	go func() {
		buf := make([]byte, 4096)
		reader := io.MultiReader(stdout, stderr)
		for {
			n, err := reader.Read(buf)
			if n > 0 && m.em != nil {
				m.em.Emit("session:pipe:output", map[string]any{
					"tmpID":   tmpID,
					"command": command,
					"data":    string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}

		_ = cmd.Wait()

		m.mu.Lock()
		if curr, ok := m.pipes[tmpID]; ok && curr == active {
			stream.RemoveListener(active.ListenerID)
			delete(m.pipes, tmpID)
		}
		m.mu.Unlock()

		if m.em != nil {
			m.em.Emit("session:pipe:stopped", map[string]any{
				"tmpID":   tmpID,
				"command": command,
			})
		}
	}()

	return nil
}

// StopPipe terminates any active pipe command for the session.
func (m *PipeTeeManager) StopPipe(tmpID string, stream *Stream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipe, ok := m.pipes[tmpID]
	if !ok {
		return nil
	}

	stream.RemoveListener(pipe.ListenerID)
	_ = pipe.Stdin.Close()
	pipe.Cancel()
	delete(m.pipes, tmpID)

	if m.em != nil {
		m.em.Emit("session:pipe:stopped", map[string]any{
			"tmpID": tmpID,
		})
	}

	return nil
}
