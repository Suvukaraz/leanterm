package sftpsvc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// TransferOptions mirror the [sftp_browser] transfer preferences.
type TransferOptions struct {
	PreserveTimestamps  bool
	PreservePermissions bool
	UsePartFiles        bool
}

func DefaultTransferOptions() TransferOptions {
	return TransferOptions{PreserveTimestamps: true, PreservePermissions: true, UsePartFiles: true}
}

const maxTransferDepth = 64

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
	var errs []string
	for _, rp := range remotePaths {
		if err := downloadPath(client, rp, filepath.Join(localDir, path.Base(rp)), opts, 0); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return joinErrors(errs)
}

func downloadPath(client *sftp.Client, remote, local string, opts TransferOptions, depth int) error {
	if depth > maxTransferDepth {
		return fmt.Errorf("%s: folder nesting too deep", remote)
	}
	fi, err := client.Stat(remote)
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	if fi.IsDir() {
		if err := os.MkdirAll(local, 0o755); err != nil {
			return err
		}
		entries, err := client.ReadDir(remote)
		if err != nil {
			return fmt.Errorf("%s: %w", remote, err)
		}
		var errs []string
		for _, e := range entries {
			if err := downloadPath(client, path.Join(remote, e.Name()), filepath.Join(local, e.Name()), opts, depth+1); err != nil {
				errs = append(errs, err.Error())
			}
		}
		applyLocalMeta(local, fi, opts)
		return joinErrors(errs)
	}

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
	applyLocalMeta(local, fi, opts)
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
	var errs []string
	for _, lp := range localPaths {
		if err := uploadPath(client, lp, path.Join(remoteDir, filepath.Base(lp)), opts, 0); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return joinErrors(errs)
}

func uploadPath(client *sftp.Client, local, remote string, opts TransferOptions, depth int) error {
	if depth > maxTransferDepth {
		return fmt.Errorf("%s: folder nesting too deep", local)
	}
	fi, err := os.Stat(local)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := client.MkdirAll(remote); err != nil {
			return fmt.Errorf("%s: %w", remote, err)
		}
		entries, err := os.ReadDir(local)
		if err != nil {
			return err
		}
		var errs []string
		for _, e := range entries {
			if err := uploadPath(client, filepath.Join(local, e.Name()), path.Join(remote, e.Name()), opts, depth+1); err != nil {
				errs = append(errs, err.Error())
			}
		}
		applyRemoteMeta(client, remote, fi, opts)
		return joinErrors(errs)
	}

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
	applyRemoteMeta(client, remote, fi, opts)
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
