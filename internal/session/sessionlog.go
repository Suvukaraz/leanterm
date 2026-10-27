package session

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\].*?(\x07|\x1b\\)|\x1b[PX^_].*?\x1b\\|\x1b[()][AB012]`)

// StripEscapeSequences removes ANSI escape sequences and non-printable control codes.
func StripEscapeSequences(input []byte) []byte {
	// 1. Remove ANSI escape sequences
	cleaned := ansiRegex.ReplaceAll(input, nil)

	// 2. Remove other control characters except \r, \n, \t
	var out bytes.Buffer
	out.Grow(len(cleaned))
	for _, b := range cleaned {
		if b == '\n' || b == '\r' || b == '\t' || b >= 0x20 {
			out.WriteByte(b)
		}
	}
	return out.Bytes()
}

// SessionLogger handles writing session output to log files with optional timestamping and ANSI stripping.
type SessionLogger struct {
	opts     config.SessionLogsConfig
	substCtx vars.SubstContext
	logFile  *os.File
	mu       sync.Mutex
	atLineStart bool
	closed   bool
}

// NewSessionLogger initializes a session logger. Returns nil if logging is not enabled.
func NewSessionLogger(opts config.SessionLogsConfig, substCtx vars.SubstContext, defaultDir string) (*SessionLogger, error) {
	if !opts.Enabled {
		return nil, nil
	}

	folder := opts.Folder
	if folder == "" {
		folder = defaultDir // <config dir>/session_logs
	}

	// Create directory if not exists
	if err := os.MkdirAll(folder, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory '%s': %w", folder, err)
	}

	// Pattern substitution
	pattern := opts.Pattern
	if pattern == "" {
		pattern = "<NAME>_<DATE_Y><DATE_M><DATE_D>_<TIME_H><TIME_M><TIME_S>.log"
	}
	substPattern := vars.Subst(pattern, substCtx).Text
	// Sanitize filename
	substPattern = filepath.Base(substPattern)
	filePath := filepath.Join(folder, substPattern)

	// Rotation: delete oldest logs beyond keep
	if opts.Keep > 0 {
		rotateLogs(folder, substCtx.Name, opts.Keep)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file '%s': %w", filePath, err)
	}

	return &SessionLogger{
		opts:        opts,
		substCtx:    substCtx,
		logFile:     f,
		atLineStart: true,
	}, nil
}

func rotateLogs(folder string, connName string, keep int) {
	if keep <= 0 || connName == "" {
		return
	}

	entries, err := os.ReadDir(folder)
	if err != nil {
		return
	}

	prefix := connName + "_"
	var matches []os.FileInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			if info, err := entry.Info(); err == nil {
				matches = append(matches, info)
			}
		}
	}

	if len(matches) < keep {
		return
	}

	// Sort oldest first
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ModTime().Before(matches[j].ModTime())
	})

	toDelete := len(matches) - keep + 1
	for i := 0; i < toDelete && i < len(matches); i++ {
		_ = os.Remove(filepath.Join(folder, matches[i].Name()))
	}
}

// Write processes incoming terminal data and writes it to the log file.
func (l *SessionLogger) Write(p []byte) {
	if l == nil || len(p) == 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.logFile == nil {
		return
	}

	data := p
	if l.opts.StripControlChars {
		data = StripEscapeSequences(data)
		if len(data) == 0 {
			return
		}
	}

	if !l.opts.Timestamps {
		_, _ = l.logFile.Write(data)
		return
	}

	// Line-by-line timestamp prefixing: [YYYY-mm-dd HH:MM:SS]
	var out bytes.Buffer
	for _, b := range data {
		if l.atLineStart {
			ts := time.Now().Format("2006-01-02 15:04:05")
			out.WriteString(fmt.Sprintf("[%s] ", ts))
			l.atLineStart = false
		}
		out.WriteByte(b)
		if b == '\n' {
			l.atLineStart = true
		}
	}

	_, _ = l.logFile.Write(out.Bytes())
}

// Close flushes and closes the underlying log file.
func (l *SessionLogger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}
	l.closed = true
	if l.logFile != nil {
		err := l.logFile.Close()
		l.logFile = nil
		return err
	}
	return nil
}

// SaveManualLog dumps the ring buffer contents to a specified file.
func SaveManualLog(filePath string, bufferData []byte, stripControl bool) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := bufferData
	if stripControl {
		data = StripEscapeSequences(data)
	}

	return os.WriteFile(filePath, data, 0644)
}
