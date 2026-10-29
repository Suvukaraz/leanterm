package editor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// FileInfo describes a file as the editor needs it. ModTime is in milliseconds
// since the epoch; together with Size it detects changes made elsewhere.
type FileInfo struct {
	Exists  bool   `json:"exists"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	Mode    uint32 `json:"mode"`
}

// FS is a file system a document lives on: the local disk or an SFTP session.
type FS interface {
	Stat(path string) (FileInfo, error)
	ReadFile(path string) ([]byte, error)
	// WriteFile replaces the file's content. Existing files keep their permissions.
	WriteFile(path string, data []byte) error
}

func infoFrom(fi fs.FileInfo) FileInfo {
	return FileInfo{
		Exists:  true,
		IsDir:   fi.IsDir(),
		Size:    fi.Size(),
		ModTime: fi.ModTime().UnixMilli(),
		Mode:    uint32(fi.Mode().Perm()),
	}
}

// LocalFS is the local disk.
type LocalFS struct{}

func (LocalFS) Stat(path string) (FileInfo, error) {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return FileInfo{}, nil
	}
	if err != nil {
		return FileInfo{}, err
	}
	return infoFrom(fi), nil
}

func (LocalFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes a temporary file next to the target and renames it over the
// original, so a failed write never leaves a half-written file. Symlinks are
// followed (the link stays a link). When the folder is not writable or the
// owner cannot be kept, the file is overwritten in place instead.
func (LocalFS) WriteFile(path string, data []byte) error {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	orig, err := os.Stat(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	mode := fs.FileMode(0o644)
	if orig != nil {
		mode = orig.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".leanterm-*")
	if err != nil {
		return os.WriteFile(target, data, mode)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if orig != nil && !keepOwner(tmpName, orig) {
		return os.WriteFile(target, data, mode)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return os.WriteFile(target, data, mode)
	}
	ok = true
	return nil
}
