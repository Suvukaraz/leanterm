//go:build !windows

package localfs

import (
	"io/fs"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

// oNoFollow makes copyFile refuse to write through a symlink at the target.
const oNoFollow = syscall.O_NOFOLLOW

var (
	nameMu    sync.Mutex
	userNames = map[uint32]string{}
	groupName = map[uint32]string{}
)

func ownerOf(fi fs.FileInfo) (string, string) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}
	nameMu.Lock()
	defer nameMu.Unlock()
	u, ok := userNames[st.Uid]
	if !ok {
		u = strconv.FormatUint(uint64(st.Uid), 10)
		if usr, err := user.LookupId(u); err == nil {
			u = usr.Username
		}
		userNames[st.Uid] = u
	}
	g, ok := groupName[st.Gid]
	if !ok {
		g = strconv.FormatUint(uint64(st.Gid), 10)
		if grp, err := user.LookupGroupId(g); err == nil {
			g = grp.Name
		}
		groupName[st.Gid] = g
	}
	return u, g
}
