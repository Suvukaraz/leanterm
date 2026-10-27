//go:build linux || darwin || freebsd

package localfs

import (
	"io/fs"

	"golang.org/x/sys/unix"
)

// StatVFS reports total and free space of the file system holding p.
func StatVFS(p string) (VFSInfo, error) {
	p, err := cleanAbs(p)
	if err != nil {
		return VFSInfo{}, err
	}
	var st unix.Statfs_t
	if err := unix.Statfs(p, &st); err != nil {
		return VFSInfo{}, friendly(&fs.PathError{Op: "statfs", Path: p, Err: err})
	}
	bsize := uint64(st.Bsize)
	return VFSInfo{TotalSpace: uint64(st.Blocks) * bsize, FreeSpace: uint64(st.Bavail) * bsize}, nil
}
