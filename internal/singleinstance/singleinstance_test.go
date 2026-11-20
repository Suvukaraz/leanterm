package singleinstance

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// collector records what a running instance was handed.
type collector struct {
	mu    sync.Mutex
	calls [][]string
}

func (c *collector) handle(files []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, append([]string(nil), files...))
}

func (c *collector) got() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]string(nil), c.calls...)
}

func TestForwardWithoutInstance(t *testing.T) {
	dir := t.TempDir()
	ok, err := Forward(dir, []string{"a.txt"})
	if err != nil || ok {
		t.Fatalf("Forward without a lock file = %v, %v; want false, nil", ok, err)
	}
}

func TestForwardDeliversAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	var c collector
	srv, err := Listen(dir, c.handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ok, err := Forward(dir, []string{"sub/a.txt", "/abs/b.txt"})
	if err != nil || !ok {
		t.Fatalf("Forward = %v, %v; want true, nil", ok, err)
	}
	calls := c.got()
	if len(calls) != 1 || len(calls[0]) != 2 {
		t.Fatalf("handler calls = %v; want one call with two files", calls)
	}
	if !filepath.IsAbs(calls[0][0]) || filepath.Base(calls[0][0]) != "a.txt" {
		t.Errorf("first file = %q; want an absolute path to a.txt", calls[0][0])
	}
	want, _ := filepath.Abs("/abs/b.txt")
	if calls[0][1] != want {
		t.Errorf("second file = %q; want %q", calls[0][1], want)
	}
}

func TestForwardWithoutFilesRaisesWindow(t *testing.T) {
	dir := t.TempDir()
	var c collector
	srv, err := Listen(dir, c.handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if ok, err := Forward(dir, nil); err != nil || !ok {
		t.Fatalf("Forward(no files) = %v, %v; want true, nil", ok, err)
	}
	if calls := c.got(); len(calls) != 1 || len(calls[0]) != 0 {
		t.Fatalf("handler calls = %v; want one call without files", calls)
	}
}

func TestLockFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	dir := t.TempDir()
	srv, err := Listen(dir, func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	fi, err := os.Stat(filepath.Join(dir, LockFile))
	if err != nil {
		t.Fatal(err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("lock file mode = %v; want 0600", mode)
	}
}

func TestStaleLockIsReplaced(t *testing.T) {
	dir := t.TempDir()
	// A port nobody listens on: the instance that wrote the file is gone.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	stale, _ := json.Marshal(lockInfo{Port: port, Token: "old"})
	if err := os.WriteFile(filepath.Join(dir, LockFile), stale, 0o600); err != nil {
		t.Fatal(err)
	}

	if ok, err := Forward(dir, []string{"a.txt"}); err != nil || ok {
		t.Fatalf("Forward with a stale lock = %v, %v; want false, nil", ok, err)
	}

	var c collector
	srv, err := Listen(dir, c.handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if ok, err := Forward(dir, []string{"a.txt"}); err != nil || !ok {
		t.Fatalf("Forward after replacing the lock = %v, %v; want true, nil", ok, err)
	}
}

func TestWrongTokenIsRefused(t *testing.T) {
	dir := t.TempDir()
	var c collector
	srv, err := Listen(dir, c.handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Another program on the port, or a lock file from elsewhere.
	info, ok := readLock(dir)
	if !ok {
		t.Fatal("no lock file after Listen")
	}
	info.Token = "forged"
	b, _ := json.Marshal(info)
	if err := os.WriteFile(filepath.Join(dir, LockFile), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := Forward(dir, []string{"a.txt"}); err != nil || ok {
		t.Fatalf("Forward with a wrong token = %v, %v; want false, nil", ok, err)
	}
	if calls := c.got(); len(calls) != 0 {
		t.Fatalf("handler got calls with a wrong token: %v", calls)
	}
}

func TestCloseRemovesLock(t *testing.T) {
	dir := t.TempDir()
	srv, err := Listen(dir, func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, LockFile)); !os.IsNotExist(err) {
		t.Fatalf("lock file after Close: %v; want it removed", err)
	}
	if ok, _ := Forward(dir, nil); ok {
		t.Fatal("Forward reached a closed instance")
	}
}

func TestCloseKeepsReplacedLock(t *testing.T) {
	dir := t.TempDir()
	first, err := Listen(dir, func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Listen(dir, func([]string) {})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	// The first instance exits later than the second one took over.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if ok, err := Forward(dir, nil); err != nil || !ok {
		t.Fatalf("Forward to the second instance = %v, %v; want true, nil", ok, err)
	}
}

func TestConcurrentForwards(t *testing.T) {
	dir := t.TempDir()
	var c collector
	srv, err := Listen(dir, c.handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := Forward(dir, []string{"f" + strconv.Itoa(i) + ".txt"})
			if err != nil || !ok {
				t.Errorf("Forward %d = %v, %v", i, ok, err)
			}
		}(i)
	}
	wg.Wait()
	if n := len(c.got()); n != 8 {
		t.Fatalf("handler calls = %d; want 8", n)
	}
}
