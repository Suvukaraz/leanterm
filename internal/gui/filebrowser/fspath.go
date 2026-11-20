// Package filebrowser is the file browser next to a terminal
// (frontend/src/components/sftp): SFTP for SSH tabs, the local file system
// for local shells, with the same panel.
package filebrowser

import (
	"regexp"
	"strings"
)

// Path handling (frontend/src/lib/fspath.ts). SFTP and local Linux/macOS
// paths are POSIX; the local browser on Windows uses drive paths (C:\Users)
// and UNC paths (\\server\share), with a virtual top level for the drives.

// Drives is the virtual folder above the drive roots on Windows.
const Drives = "This PC"

var (
	reDrive     = regexp.MustCompile(`^[A-Za-z]:([\\/]|$)`)
	reUNC       = regexp.MustCompile(`^\\\\[^\\]`)
	reDriveRoot = regexp.MustCompile(`^[A-Za-z]:\\?$`)
	reUNCRoot   = regexp.MustCompile(`^\\\\[^\\]+\\[^\\]+\\?$`)
	reBareDrive = regexp.MustCompile(`^[A-Za-z]:$`)
	reSlashes   = regexp.MustCompile(`/{2,}`)
	reBacks     = regexp.MustCompile(`\\{2,}`)
)

// IsWindowsPath reports whether p is a Windows path (drive, UNC or the
// drive list).
func IsWindowsPath(p string) bool {
	return p == Drives || reDrive.MatchString(p) || reUNC.MatchString(p)
}

// NormalizePath cleans a typed or joined path: POSIX "/a/b", Windows
// "C:\a\b" (drive roots keep their "\").
func NormalizePath(p string, win bool) string {
	s := strings.TrimSpace(p)
	if !win {
		if !strings.HasPrefix(s, "/") {
			s = "/" + s
		}
		if len(s) > 1 {
			s = strings.TrimRight(s, "/")
			if s == "" {
				s = "/"
			}
		}
		return reSlashes.ReplaceAllString(s, "/")
	}
	if s == "" || strings.EqualFold(s, Drives) {
		return Drives
	}
	s = strings.ReplaceAll(s, "/", `\`)
	unc := strings.HasPrefix(s, `\\`)
	if unc {
		s = `\\` + reBacks.ReplaceAllString(s[2:], `\`)
	} else {
		s = reBacks.ReplaceAllString(s, `\`)
	}
	if len(s) >= 2 && s[1] == ':' && isLetter(s[0]) {
		s = strings.ToUpper(s[:1]) + s[1:]
		if len(s) == 2 {
			return s + `\`
		}
	}
	if IsRootPath(s, true) {
		if strings.HasSuffix(s, `\`) {
			return s
		}
		return s + `\`
	}
	return strings.TrimRight(s, `\`)
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

// RootPath is the top of the tree: "/" or the drive list.
func RootPath(win bool) string {
	if win {
		return Drives
	}
	return "/"
}

// IsRootPath reports "/", a drive root, a UNC share root or the drive list.
func IsRootPath(p string, win bool) bool {
	if !win {
		return p == "/" || p == ""
	}
	return p == Drives || p == "" || reDriveRoot.MatchString(p) || reUNCRoot.MatchString(p)
}

// ParentPath is the parent folder; ok is false at the top. Drive and share
// roots go up to the drive list.
func ParentPath(p string, win bool) (string, bool) {
	if !win {
		if p == "/" || p == "" {
			return "", false
		}
		var parts []string
		for _, s := range strings.Split(p, "/") {
			if s != "" {
				parts = append(parts, s)
			}
		}
		return "/" + strings.Join(parts[:max(0, len(parts)-1)], "/"), true
	}
	if p == Drives || p == "" {
		return "", false
	}
	if IsRootPath(p, true) {
		return Drives, true
	}
	t := strings.TrimRight(p, `\`)
	parent := t[:max(0, strings.LastIndex(t, `\`))]
	if reBareDrive.MatchString(parent) {
		return parent + `\`, true
	}
	return parent, true
}

// JoinPath appends name to dir with the right separator.
func JoinPath(dir, name string, win bool) string {
	if !win {
		if dir == "/" || dir == "" {
			return "/" + name
		}
		return dir + "/" + name
	}
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

// BaseName is the last path segment (either separator).
func BaseName(p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return p
	}
	return parts[len(parts)-1]
}
