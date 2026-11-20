// Package singleinstance lets a second standalone LeanEdit hand its files to
// the one that is already running.
//
// The running instance listens on a loopback port and writes the port and a
// random token to leanedit.lock in the configuration folder (mode 0600). A
// second instance reads the file, connects, sends the token and the absolute
// file paths as one JSON line, waits for the answer and exits. A lock file
// nobody answers for (the instance is gone) is stale and is replaced.
package singleinstance

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// LockFile is the name of the lock file in the configuration folder.
const LockFile = "leanedit.lock"

const (
	dialTimeout = time.Second
	ioTimeout   = 5 * time.Second
	maxMessage  = 1 << 20
)

type lockInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type request struct {
	Token string   `json:"token"`
	Files []string `json:"files"`
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Server is the running instance's end of the lock.
type Server struct {
	dir   string
	ln    net.Listener
	token string

	closeOnce sync.Once
}

// Listen makes this process the running instance in dir: handle gets the
// absolute paths of every forwarded request on the server's goroutine (none
// when a second instance was started without files, meaning "show yourself").
// Any lock file from an earlier instance is replaced.
func Listen(dir string, handle func(files []string)) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := writeLock(dir, lockInfo{Port: port, Token: token}); err != nil {
		_ = ln.Close()
		return nil, err
	}
	s := &Server{dir: dir, ln: ln, token: token}
	go s.serve(handle)
	return s, nil
}

// Close stops listening and removes the lock file, unless a later instance
// has replaced it already.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.ln.Close()
		if info, ok := readLock(s.dir); ok && info.Token == s.token {
			_ = os.Remove(filepath.Join(s.dir, LockFile))
		}
	})
	return err
}

func (s *Server) serve(handle func(files []string)) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // closed
		}
		go s.handleConn(conn, handle)
	}
}

func (s *Server) handleConn(conn net.Conn, handle func(files []string)) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	var req request
	if err := json.NewDecoder(io.LimitReader(conn, maxMessage)).Decode(&req); err != nil {
		writeReply(conn, reply{Error: "bad request"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.token)) != 1 {
		writeReply(conn, reply{Error: "wrong token"})
		return
	}
	handle(req.Files)
	writeReply(conn, reply{OK: true})
}

func writeReply(w io.Writer, r reply) {
	b, _ := json.Marshal(r)
	_, _ = w.Write(append(b, '\n'))
}

// Forward hands files (made absolute here) to the running instance in dir.
// It reports true when the instance has them; false means that no instance
// answered and the caller should start its own.
func Forward(dir string, files []string) (bool, error) {
	info, ok := readLock(dir)
	if !ok {
		return false, nil
	}
	abs := make([]string, 0, len(files))
	for _, f := range files {
		p, err := filepath.Abs(f)
		if err != nil {
			return false, err
		}
		abs = append(abs, p)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(info.Port)), dialTimeout)
	if err != nil {
		return false, nil // stale lock file
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	b, err := json.Marshal(request{Token: info.Token, Files: abs})
	if err != nil {
		return false, err
	}
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return false, nil
	}
	var r reply
	if err := json.NewDecoder(conn).Decode(&r); err != nil {
		return false, nil
	}
	return r.OK, nil
}

// readLock returns the lock file's content; false when there is none or it
// is not readable.
func readLock(dir string) (lockInfo, bool) {
	var info lockInfo
	b, err := os.ReadFile(filepath.Join(dir, LockFile))
	if err != nil {
		return info, false
	}
	if err := json.Unmarshal(b, &info); err != nil || info.Port <= 0 || info.Token == "" {
		return info, false
	}
	return info, true
}

// writeLock replaces the lock file; it is created with mode 0600 so that only
// the user can read the token.
func writeLock(dir string, info lockInfo) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, LockFile)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(info)
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", LockFile, err)
	}
	return f.Close()
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
