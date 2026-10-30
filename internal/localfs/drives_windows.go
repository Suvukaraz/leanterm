//go:build windows

package localfs

import (
	"io/fs"
	"syscall"

	"golang.org/x/sys/windows"
)

// Drives lists the drive letters for the browser's "This PC" level, with the
// volume label in the name ("Data (D:)").
func Drives() ([]FileEntry, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	var out []FileEntry
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		root := letter + `:\`
		name := letter + ":"
		if label := volumeLabel(root); label != "" {
			name = label + " (" + letter + ":)"
		}
		out = append(out, FileEntry{
			Name:        name,
			Path:        root,
			IsDir:       true,
			Mode:        uint32(fs.ModeDir | 0o755),
			Permissions: "drive",
		})
	}
	return out, nil
}

func volumeLabel(root string) string {
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return ""
	}
	// Removable drives without a medium would show an error dialog otherwise.
	old := windows.SetErrorMode(windows.SEM_FAILCRITICALERRORS)
	defer windows.SetErrorMode(old)
	buf := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(ptr, &buf[0], uint32(len(buf)), nil, nil, nil, nil, 0); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// isHidden: dot files and files with the Windows "hidden" attribute.
func isHidden(name string, fi fs.FileInfo) bool {
	if len(name) > 0 && name[0] == '.' {
		return true
	}
	if attrs, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return attrs.FileAttributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
	}
	return false
}
