//go:build windows

package localfs

import (
	"io/fs"

	"golang.org/x/sys/windows"
)

// StatVFS reports total and free space of the volume holding p.
func StatVFS(p string) (VFSInfo, error) {
	p, err := cleanAbs(p)
	if err != nil {
		return VFSInfo{}, err
	}
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return VFSInfo{}, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &totalFree); err != nil {
		return VFSInfo{}, friendly(&fs.PathError{Op: "statfs", Path: p, Err: err})
	}
	return VFSInfo{TotalSpace: total, FreeSpace: free}, nil
}
