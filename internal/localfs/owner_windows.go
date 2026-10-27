//go:build windows

package localfs

import "io/fs"

// Windows has no O_NOFOLLOW; copyPath already removes symlinks at the target.
const oNoFollow = 0

// ownerOf is not reported on Windows.
func ownerOf(fi fs.FileInfo) (string, string) {
	return "", ""
}
