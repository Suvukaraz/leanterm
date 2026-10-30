package sftpsvc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

type pipeRWC struct {
	io.Reader
	io.WriteCloser
}

// newLocalSFTP connects a client to an in-process SFTP server on the local file system.
func newLocalSFTP(t *testing.T, mgr *SFTPManager) string {
	t.Helper()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	srv, err := sftp.NewServer(pipeRWC{sr, sw})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve() }()
	client, err := sftp.NewClientPipe(cr, cw)
	if err != nil {
		t.Fatal(err)
	}
	// Closing the pipes first lets both ends shut down without waiting on each other.
	t.Cleanup(func() { cw.Close(); sw.Close(); client.Close(); srv.Close() })
	mgr.mu.Lock()
	mgr.clients["test"] = &SFTPClientInstance{client: client}
	mgr.mu.Unlock()
	return "test"
}

func TestSlotLimiterHonoursLimit(t *testing.T) {
	l := newSlotLimiter()
	var active, peak int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.acquire(3)
			n := atomic.AddInt32(&active, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			l.release()
		}()
	}
	wg.Wait()
	if peak != 3 {
		t.Fatalf("peak concurrency = %d, want 3", peak)
	}
}

func TestParallelUploadDownloadRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("checks Unix permission bits")
	}
	mgr := NewSFTPManager()
	id := newLocalSFTP(t, mgr)

	src := filepath.Join(t.TempDir(), "project")
	for i := 0; i < 12; i++ {
		dir := filepath.Join(src, fmt.Sprintf("d%d", i%3), "sub")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte(fmt.Sprintf("content %d", i)), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	_ = os.Chtimes(filepath.Join(src, "d0"), old, old)

	opts := DefaultTransferOptions()
	opts.Parallel = 4
	remote := t.TempDir()
	if err := mgr.UploadWithOptions(id, []string{src}, remote, opts); err != nil {
		t.Fatal(err)
	}
	back := t.TempDir()
	if err := mgr.DownloadWithOptions(id, []string{filepath.Join(remote, "project")}, back, opts); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		rel := filepath.Join("project", fmt.Sprintf("d%d", i%3), "sub", fmt.Sprintf("f%d.txt", i))
		b, err := os.ReadFile(filepath.Join(back, rel))
		if err != nil || string(b) != fmt.Sprintf("content %d", i) {
			t.Fatalf("%s = %q, %v", rel, b, err)
		}
		if fi, _ := os.Stat(filepath.Join(back, rel)); fi.Mode().Perm() != 0o640 {
			t.Errorf("%s mode = %v, want 0640", rel, fi.Mode().Perm())
		}
		if _, err := os.Stat(filepath.Join(remote, rel+".part")); !os.IsNotExist(err) {
			t.Errorf("leftover part file for %s", rel)
		}
	}
	// Folder times are applied after the files were written.
	if fi, _ := os.Stat(filepath.Join(back, "project", "d0")); !fi.ModTime().Equal(old) {
		t.Errorf("folder time = %v, want %v", fi.ModTime(), old)
	}
	if mgr.slots.active != 0 {
		t.Fatalf("slots still held: %d", mgr.slots.active)
	}
}

func TestTransferErrorsKeepGoing(t *testing.T) {
	mgr := NewSFTPManager()
	id := newLocalSFTP(t, mgr)
	src := t.TempDir()
	good := filepath.Join(src, "good.txt")
	_ = os.WriteFile(good, []byte("ok"), 0o644)
	remote := t.TempDir()
	err := mgr.UploadWithOptions(id, []string{filepath.Join(src, "missing.txt"), good}, remote, DefaultTransferOptions())
	if err == nil {
		t.Fatal("expected an error for the missing file")
	}
	if b, _ := os.ReadFile(filepath.Join(remote, "good.txt")); string(b) != "ok" {
		t.Fatal("remaining file was not uploaded")
	}
}

func TestClampParallel(t *testing.T) {
	for in, want := range map[int]int{0: 1, -3: 1, 1: 1, 5: 5, 8: 8, 20: 8} {
		if got := clampParallel(in); got != want {
			t.Errorf("clampParallel(%d) = %d, want %d", in, got, want)
		}
	}
}
