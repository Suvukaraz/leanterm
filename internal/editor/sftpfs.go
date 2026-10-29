package editor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path"

	"github.com/pkg/sftp"
)

// ErrConnectionClosed means the SSH/SFTP connection of a remote file is gone.
var ErrConnectionClosed = errors.New("the connection of this file is closed")

// remoteErr turns the different "connection is gone" errors into ErrConnectionClosed.
func remoteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sftp.ErrSSHFxConnectionLost) || errors.Is(err, sftp.ErrSSHFxNoConnection) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return ErrConnectionClosed
	}
	return err
}

// SFTPFS is a remote file system reached through an SFTP session.
type SFTPFS struct {
	Client *sftp.Client
}

func (s SFTPFS) Stat(p string) (FileInfo, error) {
	fi, err := s.Client.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return FileInfo{}, nil
	}
	if err != nil {
		return FileInfo{}, remoteErr(err)
	}
	return infoFrom(fi), nil
}

func (s SFTPFS) ReadFile(p string) ([]byte, error) {
	f, err := s.Client.Open(p)
	if err != nil {
		return nil, remoteErr(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	return data, remoteErr(err)
}

// WriteFile writes a temporary file next to the target and renames it over the
// original (posix-rename). Symlinks, files whose owner would change and servers
// without posix-rename get the file overwritten in place instead.
func (s SFTPFS) WriteFile(p string, data []byte) error {
	return remoteErr(s.writeFile(p, data))
}

func (s SFTPFS) writeFile(p string, data []byte) error {
	orig, err := s.Client.Lstat(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if orig != nil && orig.Mode()&fs.ModeSymlink != 0 {
		return s.writeInPlace(p, data)
	}

	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	tmpName := path.Join(path.Dir(p), "."+path.Base(p)+".leanterm-"+hex.EncodeToString(suffix[:]))
	tmp, err := s.Client.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return s.writeInPlace(p, data)
	}
	ok := false
	defer func() {
		if !ok {
			_ = s.Client.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if orig != nil {
		mode = orig.Mode().Perm()
	}
	if err := s.Client.Chmod(tmpName, mode); err != nil {
		return err
	}
	if orig != nil && !s.sameOwner(orig, tmpName) {
		return s.writeInPlace(p, data)
	}
	if err := s.Client.PosixRename(tmpName, p); err != nil {
		return s.writeInPlace(p, data)
	}
	ok = true
	return nil
}

// sameOwner gives tmp the owner of orig if needed and reports whether they match.
func (s SFTPFS) sameOwner(orig fs.FileInfo, tmp string) bool {
	o, ok := orig.Sys().(*sftp.FileStat)
	if !ok {
		return true
	}
	fi, err := s.Client.Stat(tmp)
	if err != nil {
		return false
	}
	t, ok := fi.Sys().(*sftp.FileStat)
	if !ok {
		return true
	}
	if o.UID == t.UID && o.GID == t.GID {
		return true
	}
	return s.Client.Chown(tmp, int(o.UID), int(o.GID)) == nil
}

func (s SFTPFS) writeInPlace(p string, data []byte) error {
	f, err := s.Client.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
