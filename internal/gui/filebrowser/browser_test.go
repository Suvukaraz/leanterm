package filebrowser

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/sftpsvc"
)

// fakeBackend is a file system of folders with file names.
type fakeBackend struct {
	kind   Kind
	cwd    string
	home   string
	dirs   map[string][]string
	mu     sync.Mutex
	calls  []string
	follow bool
}

func (f *fakeBackend) rec(s string) { f.mu.Lock(); f.calls = append(f.calls, s); f.mu.Unlock() }

func (f *fakeBackend) Kind() Kind                        { return f.kind }
func (f *fakeBackend) Open(tmpID string) (string, error) { return "id-" + tmpID, nil }
func (f *fakeBackend) Getwd(string) (string, error)      { return f.cwd, nil }
func (f *fakeBackend) Home(string) (string, error)       { return f.home, nil }
func (f *fakeBackend) List(_ string, p string, hidden bool) ([]sftpsvc.FileEntry, error) {
	f.rec("list " + p)
	names, ok := f.dirs[p]
	if !ok {
		return nil, errors.New("no such folder: " + p)
	}
	var out []sftpsvc.FileEntry
	for _, n := range names {
		if strings.HasPrefix(n, ".") && !hidden {
			continue
		}
		out = append(out, sftpsvc.FileEntry{Name: n, Path: JoinPath(p, n, false), IsDir: strings.HasSuffix(n, "/")})
	}
	return out, nil
}
func (f *fakeBackend) StatVFS(string, string) (sftpsvc.VFSInfo, error) {
	return sftpsvc.VFSInfo{FreeSpace: 1 << 30}, nil
}
func (f *fakeBackend) Mkdir(_, p string) error      { f.rec("mkdir " + p); return nil }
func (f *fakeBackend) CreateFile(_, p string) error { f.rec("create " + p); return nil }
func (f *fakeBackend) Remove(_ string, ps []string) error {
	f.rec("rm " + strings.Join(ps, ","))
	return nil
}
func (f *fakeBackend) Rename(_, from, to string) error      { f.rec("mv " + from + " " + to); return nil }
func (f *fakeBackend) OpenWithDefaultApp(_, p string) error { f.rec("open " + p); return nil }
func (f *fakeBackend) Chmod(_ string, ps []string, m uint32, r bool, s string) error {
	return nil
}
func (f *fakeBackend) SetFollow(_ string, on bool) { f.follow = on }
func (f *fakeBackend) Close(string)                {}

func newBrowser(kind Kind) (*Browser, *fakeBackend, chan struct{}) {
	fb := &fakeBackend{kind: kind, cwd: "/home/me/project", home: "/home/me", dirs: map[string][]string{
		"/home/me/project": {"b.txt", ".hidden", "src/"},
		"/home/me":         {"project/"},
		"/home":            {"me/"},
		"/srv":             {"data/"},
	}}
	changed := make(chan struct{}, 100)
	b := &Browser{TmpID: "t1", Backend: fb, Changed: func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}}
	return b, fb, changed
}

// waitFor waits until the state satisfies ok.
func waitFor(t *testing.T, b *Browser, ok func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s := b.Snapshot()
		if ok(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("state %+v", s)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestBrowserOpensAndNavigates(t *testing.T) {
	b, _, _ := newBrowser(KindLocal)
	b.Open(false, false)
	s := waitFor(t, b, func(s State) bool { return len(s.Entries) > 0 })
	if s.ID != "id-t1" || s.Start != "/home/me/project" || len(s.Entries) != 2 || s.VFS == nil {
		t.Fatalf("opened %+v", s)
	}
	b.Up()
	waitFor(t, b, func(s State) bool { return s.Current == "/home/me" })
	b.Navigate("/srv/")
	waitFor(t, b, func(s State) bool { return s.Current == "/srv" && !s.Loading })
	b.Back()
	s = waitFor(t, b, func(s State) bool { return s.Current == "/home/me" && !s.Loading })
	if len(s.History) != 3 || s.HistoryIdx != 1 {
		t.Fatalf("history %v at %d", s.History, s.HistoryIdx)
	}
	b.Forward()
	waitFor(t, b, func(s State) bool { return s.Current == "/srv" && !s.Loading })
	// A missing folder reports an error and keeps the folder.
	b.Navigate("/nope")
	s = waitFor(t, b, func(s State) bool { return s.Err != "" })
	if s.Current != "/srv" {
		t.Fatalf("current %q after an error", s.Current)
	}
	b.Home()
	waitFor(t, b, func(s State) bool { return s.Current == "/home/me" && !s.Loading })
}

func TestBrowserHiddenFollowAndActions(t *testing.T) {
	b, fb, _ := newBrowser(KindLocal)
	b.Open(false, false)
	waitFor(t, b, func(s State) bool { return len(s.Entries) == 2 })
	b.ToggleHidden()
	waitFor(t, b, func(s State) bool { return len(s.Entries) == 3 })
	b.Navigate("/home")
	waitFor(t, b, func(s State) bool { return s.Current == "/home" && !s.Loading })
	// Following a local shell goes to its folder at once.
	b.SetFollow(true)
	waitFor(t, b, func(s State) bool { return s.Current == "/home/me/project" && s.Follow })
	if !fb.follow {
		t.Fatal("backend not told to follow")
	}
	b.TerminalFolder("/srv")
	waitFor(t, b, func(s State) bool { return s.Current == "/srv" && !s.Loading })

	b.CreateFolder("new ")
	b.Rename("/srv/data", "old")
	waitFor(t, b, func(State) bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return strings.Contains(strings.Join(fb.calls, "|"), "mv /srv/data /srv/old")
	})
	fb.mu.Lock()
	calls := strings.Join(fb.calls, "|")
	fb.mu.Unlock()
	if !strings.Contains(calls, "mkdir /srv/new") {
		t.Fatalf("calls %s", calls)
	}
}

func TestConflictsFromListing(t *testing.T) {
	b, _, _ := newBrowser(KindSFTP)
	b.Open(false, false)
	waitFor(t, b, func(s State) bool { return len(s.Entries) > 0 })
	got := b.Conflicts([]string{"/tmp/b.txt", "/tmp/c.txt"}, "/home/me/project")
	if strings.Join(got, ",") != "b.txt" {
		t.Fatalf("conflicts %v", got)
	}
}

func TestSorted(t *testing.T) {
	es := []sftpsvc.FileEntry{
		{Name: "file10", Size: 5}, {Name: "File2", Size: 50}, {Name: "zdir", IsDir: true}, {Name: "adir", IsDir: true},
	}
	names := func(es []sftpsvc.FileEntry) string {
		var s []string
		for _, e := range es {
			s = append(s, e.Name)
		}
		return strings.Join(s, " ")
	}
	if got := names(Sorted(es, SortName, true)); got != "adir zdir File2 file10" {
		t.Errorf("by name: %s", got)
	}
	if got := names(Sorted(es, SortSize, false)); got != "zdir adir File2 file10" && got != "adir zdir File2 file10" {
		t.Errorf("by size desc: %s", got)
	}
	if FormatBytes(1536) != "1.5 KB" || FormatBytes(10) != "10 B" {
		t.Error("FormatBytes")
	}
}
