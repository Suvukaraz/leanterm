package localfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(entries []FileEntry) map[string]FileEntry {
	m := map[string]FileEntry{}
	for _, e := range entries {
		m[e.Name] = e
	}
	return m
}

func TestRelativePathsRejected(t *testing.T) {
	if _, err := List("relative/dir", false); err == nil {
		t.Fatal("expected error for relative path")
	}
	if err := Remove([]string{"x"}); err == nil {
		t.Fatal("expected error for relative path")
	}
	if err := Remove([]string{"/"}); err == nil {
		t.Fatal("expected refusal to delete /")
	}
}

func TestListHiddenAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.txt"), "a")
	write(t, filepath.Join(dir, ".hidden"), "h")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink("sub", filepath.Join(dir, "link-dir"))
	_ = os.Symlink("missing", filepath.Join(dir, "broken"))

	entries, err := List(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	m := names(entries)
	if _, ok := m[".hidden"]; ok {
		t.Error("hidden file listed without showHidden")
	}
	if e := m["link-dir"]; !e.IsLink || !e.IsDir || e.LinkTarget != "sub" {
		t.Errorf("link-dir = %+v, want link to folder", e)
	}
	if e := m["broken"]; !e.IsLink || !e.LinkBroken {
		t.Errorf("broken = %+v, want broken link", e)
	}

	entries, _ = List(dir, true)
	if _, ok := names(entries)[".hidden"]; !ok {
		t.Error("hidden file missing with showHidden")
	}
}

func TestCreateRenameRemove(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "new.txt")
	if err := CreateFile(f); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(f); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second CreateFile: %v, want 'already exists'", err)
	}
	if err := Mkdir(filepath.Join(dir, "x", "y"), true); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "other.txt"), "o")
	if err := Rename(f, filepath.Join(dir, "other.txt")); err == nil {
		t.Fatal("rename must not replace an existing file")
	}
	if err := Rename(f, filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}

	// Removing a symlink to a folder keeps the folder's contents.
	target := filepath.Join(dir, "x")
	link := filepath.Join(dir, "x-link")
	_ = os.Symlink(target, link)
	if err := Remove([]string{link}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "y")); err != nil {
		t.Fatal("symlink removal deleted the target")
	}
	if err := Remove([]string{target}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("folder not removed")
	}
}

func TestCopyAndConflicts(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	write(t, filepath.Join(src, "f.txt"), "new")
	if err := os.Mkdir(filepath.Join(src, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(src, "d", "inner.txt"), "inner")
	_ = os.Symlink("inner.txt", filepath.Join(src, "d", "link"))
	write(t, filepath.Join(dst, "f.txt"), "old")

	paths := []string{filepath.Join(src, "f.txt"), filepath.Join(src, "d")}
	conflicts, err := Conflicts(paths, dst)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "f.txt" {
		t.Fatalf("Conflicts = %v, %v; want [f.txt]", conflicts, err)
	}

	err = Copy(paths, dst, CopyOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Copy without overwrite: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "f.txt")); string(b) != "old" {
		t.Fatal("existing file replaced without overwrite")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "d", "inner.txt")); string(b) != "inner" {
		t.Fatal("folder not copied")
	}
	if target, err := os.Readlink(filepath.Join(dst, "d", "link")); err != nil || target != "inner.txt" {
		t.Fatalf("symlink not copied as link: %q %v", target, err)
	}

	if err := Copy(paths, dst, CopyOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "f.txt")); string(b) != "new" {
		t.Fatal("overwrite did not replace the file")
	}

	if err := Copy([]string{src}, filepath.Join(src, "d"), CopyOptions{}); err == nil {
		t.Fatal("copying a folder into itself must fail")
	}
}

func TestMove(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	write(t, filepath.Join(src, "m.txt"), "m")
	if err := Move([]string{filepath.Join(src, "m.txt")}, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "m.txt")); !os.IsNotExist(err) {
		t.Fatal("source still exists after move")
	}

	// Moving a folder onto an existing folder merges with overwrite.
	_ = os.MkdirAll(filepath.Join(src, "d"), 0o755)
	_ = os.MkdirAll(filepath.Join(dst, "d"), 0o755)
	write(t, filepath.Join(src, "d", "a"), "a")
	write(t, filepath.Join(dst, "d", "b"), "b")
	if err := Move([]string{filepath.Join(src, "d")}, dst, CopyOptions{}); err == nil {
		t.Fatal("move onto existing folder needs overwrite")
	}
	if err := Move([]string{filepath.Join(src, "d")}, dst, CopyOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(dst, "d", n)); err != nil {
			t.Fatalf("%s missing after merge: %v", n, err)
		}
	}
}

func TestChmodRecursiveScope(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	_ = os.Mkdir(sub, 0o755)
	f := filepath.Join(sub, "f")
	write(t, f, "x")
	outside := filepath.Join(t.TempDir(), "outside")
	write(t, outside, "o")
	_ = os.Symlink(outside, filepath.Join(sub, "link"))

	if err := Chmod([]string{dir}, 0o700, true, "files"); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o700 {
		t.Errorf("file mode = %v, want 0700", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(sub); fi.Mode().Perm() != 0o755 {
		t.Errorf("folder changed with scope files: %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(outside); fi.Mode().Perm() != 0o644 {
		t.Errorf("symlink target changed: %v", fi.Mode().Perm())
	}
}

func TestPermissionDeniedMessage(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	_ = os.Mkdir(locked, 0o000)
	defer os.Chmod(locked, 0o755)
	_, err := List(locked, false)
	if err == nil || !strings.HasPrefix(err.Error(), "permission denied: ") {
		t.Fatalf("List(locked) = %v, want 'permission denied: ...'", err)
	}
}

func TestProcessCwd(t *testing.T) {
	want, _ := os.Getwd()
	got, err := ProcessCwd(os.Getpid())
	if err != nil || got != want {
		t.Fatalf("ProcessCwd = %q, %v; want %q", got, err, want)
	}
	if _, err := ProcessCwd(0); err == nil {
		t.Fatal("expected error for pid 0")
	}
}

func TestStatVFSAndHome(t *testing.T) {
	info, err := StatVFS(t.TempDir())
	if err != nil || info.TotalSpace == 0 || info.FreeSpace > info.TotalSpace {
		t.Fatalf("StatVFS = %+v, %v", info, err)
	}
	if _, err := StatVFS("relative"); err == nil {
		t.Fatal("expected error for relative path")
	}
	t.Setenv("HOME", "/home/someone/")
	if h, _ := Home(); h != "/home/someone" {
		t.Fatalf("Home = %q", h)
	}
}
