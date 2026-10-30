//go:build !windows

package localfs

import (
	"io/fs"
	"strings"
)

// Drives lists the file system roots; on Unix that is "/" only.
func Drives() ([]FileEntry, error) {
	return []FileEntry{{Name: "/", Path: "/", IsDir: true, Mode: uint32(fs.ModeDir | 0o755), Permissions: "drive"}}, nil
}

func isHidden(name string, _ fs.FileInfo) bool {
	return strings.HasPrefix(name, ".")
}
