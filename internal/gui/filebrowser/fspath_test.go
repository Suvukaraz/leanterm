package filebrowser

import "testing"

// Ported from frontend/src/lib/fspath.test.ts.

func TestPosixPaths(t *testing.T) {
	eq := func(got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	eq(NormalizePath("home/u/", false), "/home/u")
	eq(NormalizePath("/a//b", false), "/a/b")
	eq(NormalizePath("/", false), "/")
	eq(JoinPath("/", "etc", false), "/etc")
	eq(JoinPath("/etc", "hosts", false), "/etc/hosts")
	p, _ := ParentPath("/etc/ssh", false)
	eq(p, "/etc")
	p, _ = ParentPath("/etc", false)
	eq(p, "/")
	if _, ok := ParentPath("/", false); ok {
		t.Error("parent of /")
	}
	if !IsRootPath("/", false) {
		t.Error("/ is the root")
	}
}

func TestWindowsPaths(t *testing.T) {
	for p, want := range map[string]bool{`C:\Users\me`: true, "d:/data": true, `\\server\share`: true, Drives: true, "/home/me": false} {
		if IsWindowsPath(p) != want {
			t.Errorf("IsWindowsPath(%q) = %v", p, !want)
		}
	}
	for in, want := range map[string]string{
		"c:":                  `C:\`,
		"C:/Users/me/":        `C:\Users\me`,
		`C:\\Users\\me`:       `C:\Users\me`,
		`\\server\share`:      `\\server\share\`,
		`\\server\share\dir\`: `\\server\share\dir`,
		"":                    Drives,
		"this pc":             Drives,
	} {
		if got := NormalizePath(in, true); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		`C:\Users\me`:        `C:\Users`,
		`C:\Users`:           `C:\`,
		`C:\`:                Drives,
		`\\server\share\dir`: `\\server\share`,
		`\\server\share\`:    Drives,
	} {
		if got, ok := ParentPath(in, true); !ok || got != want {
			t.Errorf("ParentPath(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := ParentPath(Drives, true); ok {
		t.Error("parent of the drive list")
	}
	if got := JoinPath(`C:\`, "Users", true); got != `C:\Users` {
		t.Error(got)
	}
	if got := JoinPath(`C:\Users`, "me", true); got != `C:\Users\me` {
		t.Error(got)
	}
	if BaseName(`C:\Users\me\file.txt`) != "file.txt" || BaseName("/etc/hosts") != "hosts" {
		t.Error("BaseName")
	}
}
