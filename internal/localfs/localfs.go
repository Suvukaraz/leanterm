// Package localfs implements the file operations of the file browser shown next to
// local shell tabs. Everything uses the Go standard library directly; no shell is
// ever involved, and all paths must be absolute.
package localfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Suvukaraz/leanterm/internal/platform"
	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

// FileEntry and VFSInfo are shared with the SFTP browser so the frontend uses one model.
type (
	FileEntry = sftpsvc.FileEntry
	VFSInfo   = sftpsvc.VFSInfo
)

// CopyOptions mirror the [sftp_browser] transfer preferences that apply locally.
type CopyOptions struct {
	PreserveTimestamps  bool
	PreservePermissions bool
	// Overwrite replaces existing targets; otherwise an existing target is an error.
	Overwrite bool
}

const maxDepth = 64

// cleanAbs rejects relative paths and returns the cleaned absolute path.
func cleanAbs(p string) (string, error) {
	if p == "" || !filepath.IsAbs(p) {
		return "", fmt.Errorf("invalid path %q: an absolute path is required", p)
	}
	return filepath.Clean(p), nil
}

// friendly turns low-level errors into messages that name the path and the reason.
func friendly(err error) error {
	if err == nil {
		return nil
	}
	p := ""
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		p = pe.Path
	case errors.As(err, &le):
		p = le.New
	}
	if p == "" {
		return err
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("permission denied: %s", p)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("does not exist: %s", p)
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf("already exists: %s", p)
	case errors.Is(err, syscall.ENOTEMPTY):
		return fmt.Errorf("folder is not empty: %s", p)
	case errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("not a folder: %s", p)
	case errors.Is(err, syscall.EISDIR):
		return fmt.Errorf("is a folder: %s", p)
	case errors.Is(err, syscall.EROFS):
		return fmt.Errorf("read-only file system: %s", p)
	case errors.Is(err, syscall.ENOSPC):
		return fmt.Errorf("no space left on device: %s", p)
	}
	return err
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = friendly(e).Error()
	}
	return errors.New(strings.Join(msgs, "\n"))
}

// entryFor describes p from its Lstat info; symlinks report whether they point to a
// folder (so they can be entered) and whether the target is missing.
func entryFor(p string, fi fs.FileInfo) FileEntry {
	e := FileEntry{
		Name:        fi.Name(),
		Path:        p,
		Size:        fi.Size(),
		Mode:        uint32(fi.Mode()),
		ModTime:     fi.ModTime(),
		IsDir:       fi.IsDir(),
		Permissions: fi.Mode().String(),
	}
	e.Owner, e.Group = ownerOf(fi)
	if fi.Mode()&fs.ModeSymlink != 0 {
		e.IsLink = true
		if target, err := os.Readlink(p); err == nil {
			e.LinkTarget = target
		}
		if tfi, err := os.Stat(p); err == nil {
			e.IsDir = tfi.IsDir()
			if !e.IsDir {
				e.Size = tfi.Size()
			}
		} else {
			e.LinkBroken = true
		}
	}
	return e
}

// List returns the entries of dir (hidden files only when showHidden is set).
func List(dir string, showHidden bool) ([]FileEntry, error) {
	dir, err := cleanAbs(dir)
	if err != nil {
		return nil, err
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, friendly(err)
	}
	result := make([]FileEntry, 0, len(des))
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			continue // removed while listing
		}
		if !showHidden && isHidden(de.Name(), fi) {
			continue
		}
		result = append(result, entryFor(filepath.Join(dir, de.Name()), fi))
	}
	return result, nil
}

// Stat describes a single path (symlinks are not followed).
func Stat(p string) (FileEntry, error) {
	p, err := cleanAbs(p)
	if err != nil {
		return FileEntry{}, err
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return FileEntry{}, friendly(err)
	}
	return entryFor(p, fi), nil
}

// Mkdir creates a folder (and its parents when parents is set).
func Mkdir(p string, parents bool) error {
	p, err := cleanAbs(p)
	if err != nil {
		return err
	}
	if parents {
		return friendly(os.MkdirAll(p, 0o755))
	}
	return friendly(os.Mkdir(p, 0o755))
}

// CreateFile creates an empty file; an existing file is never truncated.
func CreateFile(p string) error {
	p, err := cleanAbs(p)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return friendly(err)
	}
	return f.Close()
}

// Rename renames or moves from to to; an existing target is not replaced.
func Rename(from, to string) error {
	from, err := cleanAbs(from)
	if err != nil {
		return err
	}
	to, err = cleanAbs(to)
	if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("already exists: %s", to)
	}
	return friendly(os.Rename(from, to))
}

// isRoot reports whether p is a file system root: "/" or a drive like `C:\`.
func isRoot(p string) bool {
	vol := filepath.VolumeName(p)
	return p == "/" || (vol != "" && strings.TrimRight(p[len(vol):], `\/`) == "")
}

// Remove deletes the given files and folders (folders recursively). Symlinks are
// removed themselves, never their targets.
func Remove(paths []string) error {
	var errs []error
	for _, p := range paths {
		p, err := cleanAbs(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if isRoot(p) {
			errs = append(errs, fmt.Errorf("refusing to delete %s", p))
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

// Chmod sets the permission bits of paths. With recursive set, folder contents are
// changed too, limited to "files", "dirs" or "all" by scope. Symlinks inside folders
// are skipped (chmod would change their targets).
func Chmod(paths []string, mode uint32, recursive bool, scope string) error {
	perm := fs.FileMode(mode) & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	applies := func(isDir bool) bool {
		switch scope {
		case "dirs":
			return isDir
		case "files":
			return !isDir
		}
		return true
	}
	var errs []error
	for _, p := range paths {
		p, err := cleanAbs(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if applies(fi.IsDir()) {
			if err := os.Chmod(p, perm); err != nil {
				errs = append(errs, err)
			}
		}
		if !recursive || !fi.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(p, func(sub string, d fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			if sub == p || d.Type()&fs.ModeSymlink != 0 {
				return nil
			}
			if applies(d.IsDir()) {
				if err := os.Chmod(sub, perm); err != nil {
					errs = append(errs, err)
				}
			}
			return nil
		})
		if walkErr != nil {
			errs = append(errs, walkErr)
		}
	}
	return joinErrors(errs)
}

// Conflicts returns the base names of srcPaths that already exist in dstDir.
func Conflicts(srcPaths []string, dstDir string) ([]string, error) {
	dstDir, err := cleanAbs(dstDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, src := range srcPaths {
		name := filepath.Base(src)
		if _, err := os.Lstat(filepath.Join(dstDir, name)); err == nil {
			names = append(names, name)
		}
	}
	return names, nil
}

// checkTarget validates copying/moving src into dstDir and returns the target path.
func checkTarget(src, dstDir string) (string, string, error) {
	src, err := cleanAbs(src)
	if err != nil {
		return "", "", err
	}
	dst := filepath.Join(dstDir, filepath.Base(src))
	if dst == src {
		return "", "", fmt.Errorf("source and target are the same: %s", src)
	}
	if dstDir == src || strings.HasPrefix(dstDir, src+string(filepath.Separator)) {
		return "", "", fmt.Errorf("cannot copy a folder into itself: %s", src)
	}
	return src, dst, nil
}

// Copy copies files and folders into dstDir. Symlinks are copied as links; device
// files, sockets and pipes are skipped with an error. Errors of single entries are
// collected and the remaining entries are still copied.
func Copy(srcPaths []string, dstDir string, opts CopyOptions) error {
	dstDir, err := cleanAbs(dstDir)
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range srcPaths {
		src, dst, err := checkTarget(s, dstDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, copyPath(src, dst, opts, 0)...)
	}
	return joinErrors(errs)
}

func copyPath(src, dst string, opts CopyOptions, depth int) []error {
	if depth > maxDepth {
		return []error{fmt.Errorf("folder nesting too deep: %s", src)}
	}
	fi, err := os.Lstat(src)
	if err != nil {
		return []error{err}
	}
	existing, statErr := os.Lstat(dst)
	exists := statErr == nil
	if exists && !opts.Overwrite {
		return []error{fmt.Errorf("already exists: %s", dst)}
	}

	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return []error{err}
		}
		if exists {
			if err := os.RemoveAll(dst); err != nil {
				return []error{err}
			}
		}
		return errOrNil(os.Symlink(target, dst))

	case fi.IsDir():
		if exists && !existing.IsDir() {
			if err := os.Remove(dst); err != nil {
				return []error{err}
			}
			exists = false
		}
		if !exists {
			if err := os.Mkdir(dst, 0o755); err != nil {
				return []error{err}
			}
		}
		des, err := os.ReadDir(src)
		if err != nil {
			return []error{err}
		}
		var errs []error
		for _, de := range des {
			errs = append(errs, copyPath(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name()), opts, depth+1)...)
		}
		applyMeta(dst, fi, opts)
		return errs

	case fi.Mode().IsRegular():
		if exists && (existing.IsDir() || existing.Mode()&fs.ModeSymlink != 0) {
			if err := os.RemoveAll(dst); err != nil {
				return []error{err}
			}
		}
		if err := copyFile(src, dst); err != nil {
			return []error{err}
		}
		applyMeta(dst, fi, opts)
		return nil
	}
	return []error{fmt.Errorf("skipped special file: %s", src)}
}

func errOrNil(err error) []error {
	if err != nil {
		return []error{err}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// O_NOFOLLOW: never write through a symlink that appeared at the target.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|oNoFollow, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func applyMeta(dst string, fi fs.FileInfo, opts CopyOptions) {
	if opts.PreservePermissions {
		_ = os.Chmod(dst, fi.Mode().Perm())
	}
	if opts.PreserveTimestamps {
		_ = os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	}
}

// Move moves files and folders into dstDir. A rename is used where possible; across
// file systems, or when a folder is merged into an existing one, the entry is copied
// and the source removed afterwards.
func Move(srcPaths []string, dstDir string, opts CopyOptions) error {
	dstDir, err := cleanAbs(dstDir)
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range srcPaths {
		src, dst, err := checkTarget(s, dstDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		existing, statErr := os.Lstat(dst)
		if statErr == nil {
			if !opts.Overwrite {
				errs = append(errs, fmt.Errorf("already exists: %s", dst))
				continue
			}
			srcFi, err := os.Lstat(src)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			// Folder onto folder is merged below; anything else replaces the target.
			if !(srcFi.IsDir() && existing.IsDir()) {
				if err := os.RemoveAll(dst); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		}
		if err := os.Rename(src, dst); err == nil {
			continue
		}
		copyOpts := opts
		copyOpts.PreservePermissions, copyOpts.PreserveTimestamps = true, true
		if cerrs := copyPath(src, dst, copyOpts, 0); len(cerrs) > 0 {
			errs = append(errs, cerrs...)
			continue // keep the source when the copy was incomplete
		}
		if err := os.RemoveAll(src); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

// OpenDefault opens p with the desktop's default application (xdg-open, started
// directly without a shell).
func OpenDefault(p string) error {
	p, err := cleanAbs(p)
	if err != nil {
		return err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return friendly(err)
	}
	if fi.IsDir() {
		return fmt.Errorf("is a folder: %s", p)
	}
	if err := platform.OpenFile(p); err != nil {
		return fmt.Errorf("no application could open '%s': %v", filepath.Base(p), err)
	}
	return nil
}
