//go:build !linux && !darwin && !freebsd && !windows

package localfs

import "errors"

// StatVFS is not available on this platform.
func StatVFS(p string) (VFSInfo, error) {
	return VFSInfo{}, errors.New("free space is not available on this platform")
}
