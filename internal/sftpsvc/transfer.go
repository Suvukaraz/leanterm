package sftpsvc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/sftp"
)

// TransferOptions mirror the [sftp_browser] transfer preferences.
type TransferOptions struct {
	PreserveTimestamps  bool
	PreservePermissions bool
	UsePartFiles        bool
	// Parallel is the maximum number of files copied at the same time (1-8),
	// counted across all transfers of all sessions.
	Parallel int
}

func DefaultTransferOptions() TransferOptions {
	return TransferOptions{PreserveTimestamps: true, PreservePermissions: true, UsePartFiles: true, Parallel: 3}
}

const (
	maxTransferDepth = 64
	maxParallel      = 8
)

func clampParallel(n int) int {
	if n < 1 {
		return 1
	}
	if n > maxParallel {
		return maxParallel
	}
	return n
}

// slotLimiter hands out transfer slots; the limit follows the latest setting, so
// a changed preference applies to transfers that start afterwards.
type slotLimiter struct {
	mu     sync.Mutex
	cond   *sync.Cond
	active int
	limit  int
}

func newSlotLimiter() *slotLimiter {
	l := &slotLimiter{limit: 1}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *slotLimiter) acquire(limit int) {
	l.mu.Lock()
	if limit != l.limit {
		l.limit = limit
		l.cond.Broadcast()
	}
	for l.active >= l.limit {
		l.cond.Wait()
	}
	l.active++
	l.mu.Unlock()
}

func (l *slotLimiter) release() {
	l.mu.Lock()
	l.active--
	l.cond.Broadcast()
	l.mu.Unlock()
}

// fileJob is one file of a transfer; folders are created while planning.
type fileJob struct {
	src, dst string
	fi       os.FileInfo
}

// dirMeta remembers a created folder whose permissions/times are applied after
// all files are written (writing files would change its modification time).
type dirMeta struct {
	path string
	fi   os.FileInfo
}

// errList collects errors of concurrently running jobs.
type errList struct {
	mu   sync.Mutex
	errs []string
}

func (e *errList) add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	e.errs = append(e.errs, err.Error())
	e.mu.Unlock()
}

// runJobs copies the files with up to opts.Parallel workers, each holding one of
// the manager's transfer slots while it copies.
func (m *SFTPManager) runJobs(jobs []fileJob, opts TransferOptions, copyFile func(fileJob) error, errs *errList) {
	limit := clampParallel(opts.Parallel)
	workers := limit
	if len(jobs) < workers {
		workers = len(jobs)
	}
	ch := make(chan fileJob)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range ch {
				m.slots.acquire(limit)
				errs.add(copyFile(job))
				m.slots.release()
			}
		}()
	}
	for _, job := range jobs {
		ch <- job
	}
	close(ch)
	wg.Wait()
}

// DownloadWithOptions downloads files and folders. Errors of single files are
// collected; the remaining files are still transferred.
func (m *SFTPManager) DownloadWithOptions(sftpID string, remotePaths []string, localDir string, opts TransferOptions) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}
	if localDir == "" {
		return errors.New("no local target folder")
	}
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}
	errs := &errList{}
	var jobs []fileJob
	var dirs []dirMeta
	for _, rp := range remotePaths {
		errs.add(planDownload(client, rp, filepath.Join(localDir, path.Base(rp)), &jobs, &dirs, 0))
	}
	m.runJobs(jobs, opts, func(j fileJob) error { return downloadFile(client, j, opts) }, errs)
	// Deepest folders first, so setting a parent's time is not undone by a child.
	for i := len(dirs) - 1; i >= 0; i-- {
		applyLocalMeta(dirs[i].path, dirs[i].fi, opts)
	}
	return joinErrors(errs.errs)
}

// planDownload creates the local folders and lists the files to copy.
func planDownload(client *sftp.Client, remote, local string, jobs *[]fileJob, dirs *[]dirMeta, depth int) error {
	if depth > maxTransferDepth {
		return fmt.Errorf("%s: folder nesting too deep", remote)
	}
	fi, err := client.Stat(remote)
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	if !fi.IsDir() {
		*jobs = append(*jobs, fileJob{src: remote, dst: local, fi: fi})
		return nil
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		return err
	}
	*dirs = append(*dirs, dirMeta{path: local, fi: fi})
	entries, err := client.ReadDir(remote)
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	var errs []string
	for _, e := range entries {
		if err := planDownload(client, path.Join(remote, e.Name()), filepath.Join(local, e.Name()), jobs, dirs, depth+1); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return joinErrors(errs)
}

func downloadFile(client *sftp.Client, j fileJob, opts TransferOptions) error {
	remote, local := j.src, j.dst
	src, err := client.Open(remote)
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	defer src.Close()

	target := local
	if opts.UsePartFiles {
		target = local + ".part"
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	// *sftp.File implements WriterTo with concurrent reads.
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(target)
		return fmt.Errorf("%s: %w", remote, err)
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if target != local {
		if err := os.Rename(target, local); err != nil {
			return err
		}
	}
	applyLocalMeta(local, j.fi, opts)
	return nil
}

func applyLocalMeta(local string, fi os.FileInfo, opts TransferOptions) {
	if opts.PreservePermissions {
		_ = os.Chmod(local, fi.Mode().Perm())
	}
	if opts.PreserveTimestamps {
		_ = os.Chtimes(local, fi.ModTime(), fi.ModTime())
	}
}

// UploadWithOptions uploads local files and folders into remoteDir.
func (m *SFTPManager) UploadWithOptions(sftpID string, localPaths []string, remoteDir string, opts TransferOptions) error {
	client, err := m.getClient(sftpID)
	if err != nil {
		return err
	}
	errs := &errList{}
	var jobs []fileJob
	var dirs []dirMeta
	for _, lp := range localPaths {
		errs.add(planUpload(client, lp, path.Join(remoteDir, filepath.Base(lp)), &jobs, &dirs, 0))
	}
	m.runJobs(jobs, opts, func(j fileJob) error { return uploadFile(client, j, opts) }, errs)
	for i := len(dirs) - 1; i >= 0; i-- {
		applyRemoteMeta(client, dirs[i].path, dirs[i].fi, opts)
	}
	return joinErrors(errs.errs)
}

// planUpload creates the remote folders and lists the files to copy.
func planUpload(client *sftp.Client, local, remote string, jobs *[]fileJob, dirs *[]dirMeta, depth int) error {
	if depth > maxTransferDepth {
		return fmt.Errorf("%s: folder nesting too deep", local)
	}
	fi, err := os.Stat(local)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		*jobs = append(*jobs, fileJob{src: local, dst: remote, fi: fi})
		return nil
	}
	if err := client.MkdirAll(remote); err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	*dirs = append(*dirs, dirMeta{path: remote, fi: fi})
	entries, err := os.ReadDir(local)
	if err != nil {
		return err
	}
	var errs []string
	for _, e := range entries {
		if err := planUpload(client, filepath.Join(local, e.Name()), path.Join(remote, e.Name()), jobs, dirs, depth+1); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return joinErrors(errs)
}

func uploadFile(client *sftp.Client, j fileJob, opts TransferOptions) error {
	local, remote := j.src, j.dst
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()

	target := remote
	if opts.UsePartFiles {
		target = remote + ".part"
	}
	dst, err := client.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	// *sftp.File implements ReaderFrom with concurrent writes.
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = client.Remove(target)
		return fmt.Errorf("%s: %w", remote, err)
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if target != remote {
		if err := client.PosixRename(target, remote); err != nil {
			_ = client.Remove(remote)
			if err := client.Rename(target, remote); err != nil {
				return fmt.Errorf("%s: %w", remote, err)
			}
		}
	}
	applyRemoteMeta(client, remote, j.fi, opts)
	return nil
}

func applyRemoteMeta(client *sftp.Client, remote string, fi os.FileInfo, opts TransferOptions) {
	if opts.PreservePermissions {
		_ = client.Chmod(remote, fi.Mode().Perm())
	}
	if opts.PreserveTimestamps {
		_ = client.Chtimes(remote, fi.ModTime(), fi.ModTime())
	}
}

func joinErrors(errs []string) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) > 5 {
		errs = append(errs[:5], fmt.Sprintf("... and %d more", len(errs)-5))
	}
	return errors.New(strings.Join(errs, "\n"))
}

// DefaultDownloadDir resolves the configured download folder: explicit setting,
// else the XDG download dir, else ~/Downloads, else the home directory.
func DefaultDownloadDir(configured string) string {
	home, _ := os.UserHomeDir()
	if configured != "" {
		if strings.HasPrefix(configured, "~/") && home != "" {
			return filepath.Join(home, configured[2:])
		}
		return configured
	}
	if xdg := xdgDownloadDir(home); xdg != "" {
		return xdg
	}
	if home == "" {
		return os.TempDir()
	}
	return filepath.Join(home, "Downloads")
}

func xdgDownloadDir(home string) string {
	if home == "" {
		return ""
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	data, err := os.ReadFile(filepath.Join(cfg, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR=") {
			continue
		}
		v := strings.Trim(strings.TrimPrefix(line, "XDG_DOWNLOAD_DIR="), `"`)
		return strings.ReplaceAll(v, "$HOME", home)
	}
	return ""
}
