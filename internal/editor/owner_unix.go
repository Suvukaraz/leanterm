//go:build !windows

package editor

import (
	"io/fs"
	"os"
	"syscall"
)

// keepOwner gives tmp the owner of the original file. It reports false when that
// is not possible, so the caller overwrites the original in place instead.
func keepOwner(tmp string, orig fs.FileInfo) bool {
	st, ok := orig.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	if int(st.Uid) == os.Geteuid() && int(st.Gid) == os.Getegid() {
		return true
	}
	return os.Chown(tmp, int(st.Uid), int(st.Gid)) == nil
}
