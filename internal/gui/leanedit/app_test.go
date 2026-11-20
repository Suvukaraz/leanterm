package leanedit

import (
	"errors"
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/editor"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// fakeSvc keeps files in memory.
type fakeSvc struct {
	mu      sync.Mutex
	files   map[string]string
	mod     map[string]int64
	pending []services.EditorFile
	saves   map[string]*strings.Builder
	loads   map[string][]string
	unsaved bool
	closed  bool
	nextID  int
	saveAs  string
}

func newFake() *fakeSvc {
	return &fakeSvc{files: map[string]string{}, mod: map[string]int64{}, saves: map[string]*strings.Builder{}, loads: map[string][]string{}}
}

func (f *fakeSvc) Stat(fl services.EditorFile) (editor.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.files[fl.Path]
	return editor.FileInfo{Exists: ok, Size: int64(len(s)), ModTime: f.mod[fl.Path]}, nil
}

func (f *fakeSvc) Load(fl services.EditorFile, enc string, allowBinary bool) (editor.LoadResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.files[fl.Path]
	if !ok {
		return editor.LoadResult{}, errors.New("no such file")
	}
	if strings.ContainsRune(s, 0) && !allowBinary {
		return editor.LoadResult{Binary: true}, nil
	}
	f.nextID++
	id := string(rune('a' + f.nextID))
	// Two chunks, to test the chunked reading.
	half := len(s) / 2
	f.loads[id] = []string{s[:half], s[half:]}
	le := "lf"
	if strings.Contains(s, "\r\n") {
		le = "crlf"
	}
	return editor.LoadResult{LoadID: id, Size: int64(len(s)), ModTime: f.mod[fl.Path], Encoding: "utf-8", LineEnding: le}, nil
}

func (f *fakeSvc) ReadChunk(id string) (editor.Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	parts := f.loads[id]
	c := editor.Chunk{Text: parts[0], Done: len(parts) == 1}
	f.loads[id] = parts[1:]
	return c, nil
}

func (f *fakeSvc) BeginSave() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := string(rune('A' + f.nextID))
	f.saves[id] = &strings.Builder{}
	return id
}

func (f *fakeSvc) AppendSave(id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves[id].WriteString(text)
	return nil
}

func (f *fakeSvc) CommitSave(id string, fl services.EditorFile, o editor.SaveOptions) (editor.SaveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !o.Force && o.ExpectedModTime != f.mod[fl.Path] {
		return editor.SaveResult{Conflict: true}, nil
	}
	f.files[fl.Path] = f.saves[id].String()
	f.mod[fl.Path]++
	return editor.SaveResult{Size: int64(len(f.files[fl.Path])), ModTime: f.mod[fl.Path]}, nil
}

func (f *fakeSvc) Discard(string) {}
func (f *fakeSvc) TakePending() []services.EditorFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.pending
	f.pending = nil
	return p
}
func (f *fakeSvc) ChooseOpenFile(string) (string, error) { return "", nil }
func (f *fakeSvc) ChooseSaveFile(string) (string, error) { return f.saveAs, nil }
func (f *fakeSvc) GetEditorSettings() services.EditorSettings {
	return services.EditorSettings{Editor: config.EditorConfig{TabWidth: 4, LargeFileMB: 10, HugeFileMB: 100}}
}
func (f *fakeSvc) SetUnsaved(u bool) {
	f.mu.Lock()
	f.unsaved = u
	f.mu.Unlock()
}
func (f *fakeSvc) CloseWindow() {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
}
func (f *fakeSvc) NewWindow() error       { return nil }
func (f *fakeSvc) ShowPreferences() error { return nil }

// harness draws the editor headless.
type harness struct {
	t   *testing.T
	a   *App
	r   input.Router
	ops op.Ops
	th  *gtk.Theme
	now time.Time
}

func newHarness(t *testing.T, svc *fakeSvc) *harness {
	return &harness{t: t, a: New(svc), th: gtk.New("dark"), now: time.Now()}
}

func (h *harness) frame() {
	h.ops.Reset()
	h.now = h.now.Add(200 * time.Millisecond)
	gtx := layout.Context{Ops: &h.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(900, 640)}, Source: h.r.Source(), Now: h.now}
	h.a.Layout(gtx, h.th)
	h.r.Frame(&h.ops)
}

// until draws frames until ok holds (flows run in goroutines).
func (h *harness) until(what string, ok func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		h.frame()
		time.Sleep(2 * time.Millisecond)
	}
}

// answerNext waits for a question and answers it.
func (h *harness) answerNext(title, id string) {
	h.t.Helper()
	h.until("question "+title, func() bool { return h.a.question != nil })
	if h.a.question.Title != title {
		h.t.Fatalf("question %q, want %q (%s)", h.a.question.Title, title, h.a.question.Message)
	}
	h.a.answer(id)
}

func TestOpenEditSaveClose(t *testing.T) {
	svc := newFake()
	svc.files["/w/main.go"] = "package main\n\nfunc main() {}\n"
	svc.pending = []services.EditorFile{{Kind: "local", Path: "/w/main.go"}}
	h := newHarness(t, svc)
	h.until("the file to load", func() bool { t := h.a.Active(); return t != nil && t.ready })
	tab := h.a.Active()
	if tab.Title != "main.go" || tab.LanguageID != "go" || tab.Text() != svc.files["/w/main.go"] || tab.Dirty {
		t.Fatalf("loaded: %q %q dirty=%v", tab.Title, tab.LanguageID, tab.Dirty)
	}

	// Opening it again shows the same tab.
	h.a.OpenFile(services.EditorFile{Kind: "local", Path: "/w/main.go"})
	h.frame()
	h.frame()
	if len(h.a.tabs) != 1 {
		t.Fatalf("%d tabs", len(h.a.tabs))
	}

	tab.ed.SetCaret(0, 0)
	tab.ed.Insert("// x\n")
	h.a.markDirty(tab)
	if !tab.Dirty || !svc.unsaved {
		t.Fatal("not dirty after an edit")
	}
	h.a.Save()
	h.until("the save", func() bool { return !tab.Dirty })
	if svc.files["/w/main.go"] != "// x\npackage main\n\nfunc main() {}\n" || !strings.HasPrefix(h.a.status, "Saved ") {
		t.Fatalf("saved %q, status %q", svc.files["/w/main.go"], h.a.status)
	}

	// Another program changes the file: saving asks.
	svc.mu.Lock()
	svc.mod["/w/main.go"] = 99
	svc.mu.Unlock()
	tab.ed.Insert("y")
	h.a.markDirty(tab)
	h.a.Save()
	h.answerNext("File changed on disk", "overwrite")
	h.until("the forced save", func() bool { return !tab.Dirty })

	// Closing a changed document asks; Discard closes it.
	tab.ed.Insert("z")
	h.a.markDirty(tab)
	h.a.closeActive()
	h.answerNext("Close Document", "discard")
	h.until("the tab to close", func() bool { return len(h.a.tabs) == 0 })
	if svc.unsaved {
		t.Error("still unsaved")
	}
}

func TestBinaryAndMissingFiles(t *testing.T) {
	svc := newFake()
	svc.files["/w/bin"] = "ab\x00cd"
	h := newHarness(t, svc)
	h.frame()
	h.a.OpenFile(services.EditorFile{Kind: "local", Path: "/w/nothing"})
	h.answerNext("LeanEdit", "ok")
	h.a.OpenFile(services.EditorFile{Kind: "local", Path: "/w/bin"})
	h.answerNext("Binary file", "cancel")
	h.until("the tab to go", func() bool { return len(h.a.tabs) == 0 })
	h.a.OpenFile(services.EditorFile{Kind: "local", Path: "/w/bin"})
	h.answerNext("Binary file", "open")
	h.until("the binary file", func() bool { t := h.a.Active(); return t != nil && t.ready })
}

func TestCloseWindowAsksForUnsaved(t *testing.T) {
	svc := newFake()
	svc.saveAs = "/w/new.txt"
	h := newHarness(t, svc)
	h.frame()
	h.a.NewDocument()
	tab := h.a.Active()
	tab.ed.Insert("hello\r\n")
	h.a.markDirty(tab)
	h.a.RequestClose()
	h.answerNext("Close Editor", "save")
	h.until("the window to close", func() bool { svc.mu.Lock(); defer svc.mu.Unlock(); return svc.closed })
	if svc.files["/w/new.txt"] != "hello\n" || tab.Title != "new.txt" {
		t.Errorf("saved as: %q, title %q", svc.files["/w/new.txt"], tab.Title)
	}
}

func TestDiskChangeOnFocus(t *testing.T) {
	svc := newFake()
	svc.files["/w/a.txt"] = "one"
	svc.pending = []services.EditorFile{{Kind: "local", Path: "/w/a.txt"}}
	h := newHarness(t, svc)
	h.until("load", func() bool { t := h.a.Active(); return t != nil && t.ready })
	svc.mu.Lock()
	svc.files["/w/a.txt"] = "two!"
	svc.mod["/w/a.txt"] = 5
	svc.mu.Unlock()
	h.a.lastDisk = time.Time{}
	h.a.Focused()
	h.answerNext("File changed on disk", "reload")
	h.until("reload", func() bool { return h.a.Active().Text() == "two!" && !h.a.checking })
	// Ignored once, it stays quiet until the file changes again.
	svc.mu.Lock()
	svc.mod["/w/a.txt"] = 6
	svc.mu.Unlock()
	h.a.lastDisk = time.Time{}
	h.a.Focused()
	h.answerNext("File changed on disk", "ignore")
	h.until("check done", func() bool { return !h.a.checking })
	h.a.lastDisk = time.Time{}
	h.a.Focused()
	h.until("check done", func() bool { return !h.a.checking })
	if h.a.question != nil {
		t.Error("asked again about an ignored change")
	}
}

func TestMenusAndFormat(t *testing.T) {
	if formatSize(1536) != "1.5 KB" || formatSize(20<<20) != "20 MB" || classifySize(11<<20, 10, 100) != "large" {
		t.Error("formatSize/classifySize")
	}
	svc := newFake()
	h := newHarness(t, svc)
	h.frame()
	h.a.NewDocument()
	h.frame()
	for _, m := range h.a.menus() {
		if m.label == "" {
			t.Error("menu without label")
		}
	}
	if it := findItem(h.a.menus()[0].items, "0"); it == nil || it.label != "New" {
		t.Errorf("findItem: %+v", it)
	}
	if it := findItem(h.a.menus()[5].items, "2.2.0"); it == nil {
		t.Error("submenu item not found")
	}
	for _, s := range []string{"light", "dark"} {
		h.th = gtk.New(s)
		h.frame()
	}
}

func TestCommandsOnTheEditor(t *testing.T) {
	svc := newFake()
	h := newHarness(t, svc)
	h.frame()
	h.a.NewDocument()
	tab := h.a.Active()
	tab.ed.SetText("one\ntwo\nthree\nfour")
	tab.savedText = tab.ed.Text()
	h.frame()

	// Bookmarks follow edits.
	tab.setSel(Sel{4, 4})
	tab.toggleBookmark()
	if got := tab.bookmarkLines(); len(got) != 1 || got[0] != 2 {
		t.Fatal(got)
	}
	h.a.applyEdit(tab, []Change{{0, 0, "zero\n"}}, nil)
	if got := tab.bookmarkLines(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("after insert: %v", got)
	}
	tab.setSel(Sel{0, 0})
	h.a.gotoBookmark(1)
	if n := tab.lineIndex().lineAt(tab.sel().Head).n; n != 3 {
		t.Fatalf("goto bookmark: line %d", n)
	}
	if !tab.modifiedSet()[1] || !tab.Dirty {
		t.Error("line 1 not modified")
	}

	// Undo takes the whole command back.
	tab.ed.Undo()
	if tab.ed.Text() != "one\ntwo\nthree\nfour" {
		t.Fatalf("undo: %q", tab.ed.Text())
	}

	// Case, join, comment through the menu commands.
	tab.setSel(Sel{0, 3})
	h.a.caseCmd("upper")
	if !strings.HasPrefix(tab.ed.Text(), "ONE\n") {
		t.Fatalf("upper: %q", tab.ed.Text())
	}
	tab.setSel(Sel{0, 0})
	h.a.cmd(lines(joinLines))
	if !strings.HasPrefix(tab.ed.Text(), "ONE two\n") {
		t.Fatalf("join: %q", tab.ed.Text())
	}

	// Search and replace.
	h.a.search.find.SetText("o")
	h.a.search.matchCase.Value = true
	h.a.refreshSearch(tab)
	if n := len(h.a.search.matches); n != 2 { // two, four (ONE is upper case)
		t.Fatalf("%d matches", n)
	}
	h.a.search.matchCase.Value = false
	h.a.refreshSearch(tab)
	if n := len(h.a.search.matches); n != 3 {
		t.Fatalf("%d matches ignoring case", n)
	}
	h.a.search.repl.SetText("0")
	h.a.replaceAll()
	if tab.ed.Text() != "0NE tw0\nthree\nf0ur" {
		t.Fatalf("replace all: %q", tab.ed.Text())
	}
	h.a.search.regex.Value = true
	h.a.search.find.SetText(`(t)(h)`)
	h.a.search.repl.SetText("$2$1")
	h.a.replaceAll()
	if tab.ed.Text() != "0NE tw0\nhtree\nf0ur" {
		t.Fatalf("regex replace: %q", tab.ed.Text())
	}

	if l, c, ok := parseGoto("2:3", 1, 10); !ok || l != 2 || c != 3 {
		t.Error("goto", l, c, ok)
	}
	if l, _, ok := parseGoto("+3", 4, 10); !ok || l != 7 {
		t.Error("goto +", l)
	}
	if l, _, ok := parseGoto("50%", 1, 10); !ok || l != 5 {
		t.Error("goto %", l)
	}
	if score("main.go", "mg") < 1000 || score("main.go", "ain") != 1 || score("x", "y") != -1 {
		t.Error("score")
	}
	// The panels draw.
	h.a.search.open, h.a.gotoLn.open, h.a.quick.open = true, true, true
	h.frame()
	h.frame()
}
