// Package leanedit is LeanEdit, LeanTerm's text editor, in Gio
// (frontend/src/components/editor/EditorApp.svelte and lib/editor): tabs of
// documents edited with gvcode, loaded and saved through EditorService, a
// menu bar, tool bar and status bar.
//
// Long operations (loading, saving, questions) run as flows in goroutines,
// like the web GUI's async functions. A flow touches the editor's state only
// inside ui(func) and waits for answers with ask: both hand over to the UI
// goroutine, which runs them in Layout. So all state belongs to the UI
// goroutine.
package leanedit

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"gioui.org/text"

	"github.com/Suvukaraz/leanterm/internal/editor"
	"github.com/Suvukaraz/leanterm/internal/gui/picker"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Service is what LeanEdit needs of services.EditorService.
type Service interface {
	Stat(f services.EditorFile) (editor.FileInfo, error)
	Load(f services.EditorFile, encoding string, allowBinary bool) (editor.LoadResult, error)
	ReadChunk(loadID string) (editor.Chunk, error)
	BeginSave() string
	AppendSave(saveID, text string) error
	CommitSave(saveID string, f services.EditorFile, opts editor.SaveOptions) (editor.SaveResult, error)
	Discard(id string)
	TakePending() []services.EditorFile
	ChooseOpenFile(startDir string) (string, error)
	ChooseSaveFile(suggested string) (string, error)
	GetEditorSettings() services.EditorSettings
	SetUnsaved(unsaved bool)
	CloseWindow()
	NewWindow() error
	ShowPreferences() error
}

var _ Service = (*services.EditorService)(nil)

// App is one LeanEdit window.
type App struct {
	Svc Service
	// Shaper returns the text shaper for a font family ("" = the GUI's).
	Shaper func(family string) *text.Shaper
	// Invalidate asks for a frame (from any goroutine).
	Invalidate func()
	// SetTitle sets the window title.
	SetTitle func(string)

	tabs     []*Tab
	active   int // tab id, -1 = none
	nextID   int
	untitled int
	status   string
	ready    bool
	settings services.EditorSettings
	zoomPt   float32
	toolbar  bool
	title    string

	wrapOverride, whitespaceOverride, minimapOverride *bool

	search       searchPanel
	gotoLn       gotoPanel
	quick        quickOpen
	symPick      picker.Picker
	align        alignPrompt
	jumps, edits *history

	question *question
	writing  int
	checking bool
	lastDisk time.Time
	ignored  map[int]string

	mu    sync.Mutex
	queue []func()

	ui_ uiState
}

// New creates the window's state.
func New(svc Service) *App {
	return &App{Svc: svc, active: -1, toolbar: true, ignored: map[int]string{}}
}

// ---- the UI goroutine hand-over --------------------------------------------------

// post queues fn for the UI goroutine.
func (a *App) post(fn func()) {
	a.mu.Lock()
	a.queue = append(a.queue, fn)
	a.mu.Unlock()
	if a.Invalidate != nil {
		a.Invalidate()
	}
}

// ui runs fn on the UI goroutine and waits for it (flows only).
func (a *App) ui(fn func()) {
	done := make(chan struct{})
	a.post(func() { fn(); close(done) })
	<-done
}

// drain runs what flows queued (Layout, on the UI goroutine).
func (a *App) drain() {
	for {
		a.mu.Lock()
		q := a.queue
		a.queue = nil
		a.mu.Unlock()
		if len(q) == 0 {
			return
		}
		for _, fn := range q {
			fn()
		}
	}
}

// flow runs fn in a goroutine.
func (a *App) flow(fn func()) { go fn() }

// ---- questions (ChoiceDialog.svelte) ---------------------------------------------------

// Choice is a button of a question.
type Choice struct {
	ID, Label string
	Icon      rune
	Default   bool
}

var cancelChoice = Choice{ID: "cancel", Label: "Cancel"}

type question struct {
	Title, Message, Detail, Type string
	Choices                      []Choice
	CancelID                     string
	answer                       chan string
}

// ask shows a question and waits for the answer (flows only).
func (a *App) ask(q question) string {
	q.answer = make(chan string, 1)
	if q.CancelID == "" {
		q.CancelID = "cancel"
	}
	a.ui(func() { a.question = &q })
	return <-q.answer
}

func (a *App) showError(message string, err error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	a.ask(question{Title: "LeanEdit", Type: "error", Message: message, Detail: detail,
		Choices: []Choice{{ID: "ok", Label: "OK", Default: true}}, CancelID: "ok"})
}

// answer gives the open question its answer (UI goroutine).
func (a *App) answer(id string) {
	if q := a.question; q != nil {
		a.question = nil
		q.answer <- id
	}
}

// ---- tabs -------------------------------------------------------------------------------

func (a *App) tab(id int) *Tab {
	for _, t := range a.tabs {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Active is the shown tab (nil = none).
func (a *App) Active() *Tab { return a.tab(a.active) }

// Tabs are the open documents.
func (a *App) Tabs() []*Tab { return a.tabs }

func (a *App) activate(id int) {
	if a.tab(id) != nil {
		a.active = id
		a.ui_.focusEditor = true
	}
}

func (a *App) addTab(file *services.EditorFile, title string) *Tab {
	t := newTab(a.nextID, file, title)
	t.invalidate = func() {
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}
	a.nextID++
	a.tabs = append(a.tabs, t)
	a.activate(t.ID)
	return t
}

func (a *App) closeTabNow(id int) {
	idx := -1
	for i, t := range a.tabs {
		if t.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return
	}
	a.tabs[idx].closeFolds()
	a.tabs[idx].closeCompletion()
	a.tabs = append(a.tabs[:idx:idx], a.tabs[idx+1:]...)
	delete(a.ignored, id)
	a.forgetTab(id)
	if a.active == id {
		a.active = -1
		if len(a.tabs) > 0 {
			a.activate(a.tabs[min(idx, len(a.tabs)-1)].ID)
		}
	}
	a.updateUnsaved()
}

func (a *App) anyDirty() bool {
	for _, t := range a.tabs {
		if t.Dirty {
			return true
		}
	}
	return false
}

// updateUnsaved tells the service (the main window asks before quitting)
// and sets the window title.
func (a *App) updateUnsaved() {
	a.Svc.SetUnsaved(a.anyDirty())
	title := "LeanEdit"
	if t := a.Active(); t != nil {
		title = t.Title
		if t.Dirty {
			title += " *"
		}
		title += " — LeanEdit"
	}
	if title != a.title && a.SetTitle != nil {
		a.title = title
		a.SetTitle(title)
	}
}

// ---- files.ts -------------------------------------------------------------------------------

func fileKey(f services.EditorFile) string { return f.Kind + ":" + f.SFTPID + ":" + f.Path }

func displayPath(f services.EditorFile) string {
	if f.Kind == "sftp" {
		h := f.Host
		if h == "" {
			h = "remote"
		}
		return h + ":" + f.Path
	}
	return f.Path
}

func baseName(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

func classifySize(bytes int64, largeMB, hugeMB int) string {
	mb := float64(bytes) / (1024 * 1024)
	switch {
	case mb > float64(hugeMB):
		return "huge"
	case mb > float64(largeMB):
		return "large"
	}
	return "normal"
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v, i := float64(bytes), -1
	for {
		v /= 1024
		i++
		if v < 1024 || i == len(units)-1 {
			break
		}
	}
	if v < 10 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}

// loaded is a file read through the service.
type loaded struct {
	text       string
	size       int64
	modTime    int64
	encoding   string
	lineEnding string
	mixed      bool
}

// loadFile reads a file in chunks; nil when it looks binary (and
// allowBinary is false). progress gets the characters read so far.
func (a *App) loadFile(f services.EditorFile, encoding string, allowBinary bool, progress func(int)) (*loaded, error) {
	res, err := a.Svc.Load(f, encoding, allowBinary)
	if err != nil {
		return nil, err
	}
	if res.Binary {
		return nil, nil
	}
	var sb strings.Builder
	for {
		ch, err := a.Svc.ReadChunk(res.LoadID)
		if err != nil {
			a.Svc.Discard(res.LoadID)
			return nil, err
		}
		sb.WriteString(ch.Text)
		if progress != nil {
			progress(sb.Len())
		}
		if ch.Done {
			break
		}
	}
	le := "lf"
	if res.LineEnding == "crlf" {
		le = "crlf"
	}
	return &loaded{text: sb.String(), size: res.Size, modTime: res.ModTime, encoding: res.Encoding, lineEnding: le, mixed: res.MixedLineEndings}, nil
}

// saveChunk is the size of a save chunk (bytes; cut at rune boundaries).
const saveChunk = 2 << 20

func (a *App) saveFile(f services.EditorFile, text string, opts editor.SaveOptions) (editor.SaveResult, error) {
	id := a.Svc.BeginSave()
	for len(text) > 0 {
		n := min(len(text), saveChunk)
		for n < len(text) && n > 0 && !utf8Start(text[n]) {
			n--
		}
		if err := a.Svc.AppendSave(id, text[:n]); err != nil {
			a.Svc.Discard(id)
			return editor.SaveResult{}, err
		}
		text = text[n:]
	}
	return a.Svc.CommitSave(id, f, opts)
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// ---- opening --------------------------------------------------------------------------------

// NewDocument opens an empty "Untitled n" tab.
func (a *App) NewDocument() {
	a.untitled++
	t := a.addTab(nil, fmt.Sprintf("Untitled %d", a.untitled))
	t.setText("")
	a.updateUnsaved()
}

// OpenFile opens a file in a tab (or shows the tab that has it).
func (a *App) OpenFile(f services.EditorFile) { a.flow(func() { a.openFile(f) }) }

func (a *App) openFile(f services.EditorFile) {
	key := fileKey(f)
	found := false
	a.ui(func() {
		for _, t := range a.tabs {
			if t.File != nil && fileKey(*t.File) == key {
				a.activate(t.ID)
				found = true
			}
		}
	})
	if found {
		return
	}
	info, err := a.Svc.Stat(f)
	switch {
	case err != nil:
		a.showError(displayPath(f)+" cannot be opened.", err)
		return
	case !info.Exists:
		a.showError(displayPath(f)+" does not exist.", nil)
		return
	case info.IsDir:
		a.showError(displayPath(f)+" is a folder.", nil)
		return
	}
	var largeMB, hugeMB int
	a.ui(func() { largeMB, hugeMB = a.settings.Editor.LargeFileMB, a.settings.Editor.HugeFileMB })
	if largeMB <= 0 {
		largeMB = 10
	}
	if hugeMB < largeMB {
		hugeMB = 100
	}
	class := classifySize(info.Size, largeMB, hugeMB)
	name := baseName(f.Path)
	switch class {
	case "large":
		if a.ask(question{Title: "Large file", Type: "warning", Message: fmt.Sprintf("%s is large (%s).", name, formatSize(info.Size)),
			Detail:  "Opening it may take a moment and use a lot of memory.",
			Choices: []Choice{{ID: "open", Label: "Open", Icon: iconOpen, Default: true}, cancelChoice}}) != "open" {
			return
		}
	case "huge":
		if a.ask(question{Title: "Very large file", Type: "warning", Message: fmt.Sprintf("%s is very large (%s).", name, formatSize(info.Size)),
			Detail:  "Opening it needs a lot of memory and can take a long time; LeanEdit may become slow.",
			Choices: []Choice{{ID: "continue", Label: "Continue"}, {ID: "cancel", Label: "Cancel", Default: true}}}) != "continue" {
			return
		}
		if a.ask(question{Title: "Very large file", Type: "warning", Message: fmt.Sprintf("Really open %s (%s)?", name, formatSize(info.Size)),
			Detail:  "Syntax highlighting stays off for this file; you can turn it on in View → Syntax Highlighting.",
			Choices: []Choice{{ID: "open", Label: "Open anyway", Icon: iconOpen}, {ID: "cancel", Label: "Cancel", Default: true}}}) != "open" {
			return
		}
	}
	var id int
	a.ui(func() {
		ff := f
		t := a.addTab(&ff, name)
		t.Highlight = class != "huge"
		t.Loading = "Loading…"
		id = t.ID
	})
	a.loadInto(id, "", false)
}

// loadInto (re)loads a tab's file, replacing its document.
func (a *App) loadInto(id int, encoding string, allowBinary bool) bool {
	var f services.EditorFile
	ok := false
	a.ui(func() {
		if t := a.tab(id); t != nil && t.File != nil {
			f, ok = *t.File, true
			t.Loading = "Loading…"
		}
	})
	if !ok {
		return false
	}
	closeIfEmpty := func() {
		a.ui(func() {
			if t := a.tab(id); t != nil {
				t.Loading = ""
				if !t.ready {
					a.closeTabNow(id)
				}
			}
		})
	}
	res, err := a.loadFile(f, encoding, allowBinary, func(n int) {
		if n > 1<<20 {
			a.post(func() {
				if t := a.tab(id); t != nil {
					t.Loading = "Loading… " + formatSize(int64(n))
				}
			})
		}
	})
	if err != nil {
		a.ui(func() {
			if t := a.tab(id); t != nil {
				t.Loading = ""
			}
		})
		a.showError(displayPath(f)+" could not be read.", err)
		closeIfEmpty()
		return false
	}
	if res == nil {
		if a.ask(question{Title: "Binary file", Type: "warning", Message: baseName(f.Path) + " looks like a binary file.",
			Detail:  "Editing and saving it as text can damage it.",
			Choices: []Choice{{ID: "open", Label: "Open anyway"}, {ID: "cancel", Label: "Cancel", Default: true}}}) == "open" {
			return a.loadInto(id, encoding, true)
		}
		closeIfEmpty()
		return false
	}
	done := false
	a.ui(func() {
		t := a.tab(id)
		if t == nil {
			return
		}
		if t.LanguageID == "" {
			first := res.text
			if len(first) > 200 {
				first = first[:200]
			}
			first, _, _ = strings.Cut(first, "\n")
			t.LanguageID = detectLanguage(f.Path, first)
		}
		t.lexer = lexerFor(t.LanguageID)
		// A reload is a new document: locations and bookmarks go.
		a.forgetTab(id)
		t.bookmarks = nil
		t.setText(res.text)
		t.indentDetected, t.indentSoft, t.indentWidth = detectIndent(res.text)
		t.Encoding, t.savedEncoding = res.encoding, res.encoding
		t.LineEnding, t.savedLineEnding = res.lineEnding, res.lineEnding
		t.Mixed = res.mixed
		t.modTime, t.size = res.modTime, res.size
		t.Loading = ""
		a.status = ""
		if res.mixed {
			target := "Unix (LF)"
			if res.lineEnding == "crlf" {
				target = "Windows (CRLF)"
			}
			a.status = baseName(f.Path) + " has mixed line endings; saving converts them to " + target + "."
		}
		t.Dirty = t.computeDirty()
		a.updateUnsaved()
		done = true
	})
	return done
}

// ChooseAndOpen asks for a file and opens it.
func (a *App) ChooseAndOpen() {
	start := ""
	if t := a.Active(); t != nil && t.File != nil && t.File.Kind == "local" {
		start = path.Dir(strings.ReplaceAll(t.File.Path, `\`, "/"))
	}
	a.flow(func() {
		p, err := a.Svc.ChooseOpenFile(start)
		if err != nil {
			a.showError("The file could not be opened.", err)
			return
		}
		if p != "" {
			a.openFile(services.EditorFile{Kind: "local", Path: p})
		}
	})
}

// TakePending opens the files queued by EditorService.Open.
func (a *App) TakePending() {
	a.flow(func() {
		for _, f := range a.Svc.TakePending() {
			a.openFile(f)
		}
	})
}

// ---- saving ---------------------------------------------------------------------------------

// snapshot is what a save needs of a tab, taken on the UI goroutine.
type snapshot struct {
	file                 *services.EditorFile
	title, text          string
	encoding, lineEnding string
	modTime, size        int64
}

func (a *App) snap(id int) (snapshot, bool) {
	var s snapshot
	ok := false
	a.ui(func() {
		if t := a.tab(id); t != nil && t.ready {
			ed := a.settings.Editor
			if ch := saveCleanup(t.ed.Text(), ed.RemoveTrailingSpaces, t.modifiedSet(), ed.NewlineAtEOF); len(ch) > 0 {
				a.applyEdit(t, ch, nil)
			}
			var f *services.EditorFile
			if t.File != nil {
				ff := *t.File
				f = &ff
			}
			s = snapshot{f, t.Title, t.ed.Text(), t.Encoding, t.LineEnding, t.modTime, t.size}
			ok = true
		}
	})
	return s, ok
}

func (a *App) setWriting(d int) { a.ui(func() { a.writing += d }) }

// saveTab saves a tab; false when it was not saved (cancelled or failed).
func (a *App) saveTab(id int, force bool) bool {
	a.setWriting(1)
	defer a.setWriting(-1)
	s, ok := a.snap(id)
	if !ok {
		return false
	}
	if s.file == nil {
		return a.saveTabAs(id)
	}
	a.ui(func() { a.status = "Saving " + s.title + "…" })
	res, err := a.saveFile(*s.file, s.text, editor.SaveOptions{Encoding: s.encoding, LineEnding: s.lineEnding,
		ExpectedModTime: s.modTime, ExpectedSize: s.size, Force: force})
	a.ui(func() { a.status = "" })
	if err != nil {
		if s.file.Kind == "sftp" && strings.Contains(strings.ToLower(err.Error()), "connection of this file is closed") {
			host := s.file.Host
			if host == "" {
				host = "the server"
			}
			if a.ask(question{Title: "Connection closed", Type: "warning", Message: "The connection to " + host + " is closed.",
				Detail:  "Connect to it again in LeanTerm and save again, or save a local copy now.",
				Choices: []Choice{{ID: "local", Label: "Save Local Copy…", Icon: iconSaveAs}, {ID: "cancel", Label: "Cancel", Default: true}}}) == "local" {
				return a.saveTabAs(id)
			}
			return false
		}
		a.showError(s.title+" could not be saved.", err)
		return false
	}
	if res.Conflict {
		switch a.ask(question{Title: "File changed on disk", Type: "warning", Message: s.title + " was changed by another program since it was opened.",
			Detail: "Overwrite it with your version, or reload it and lose your changes?",
			Choices: []Choice{{ID: "overwrite", Label: "Overwrite", Icon: iconSave}, {ID: "reload", Label: "Reload", Icon: iconRefresh},
				{ID: "cancel", Label: "Cancel", Default: true}}}) {
		case "overwrite":
			return a.saveTab(id, true)
		case "reload":
			a.loadInto(id, "", true)
		}
		return false
	}
	a.ui(func() {
		t := a.tab(id)
		if t == nil {
			return
		}
		t.savedText = s.text
		t.savedEncoding, t.savedLineEnding = s.encoding, s.lineEnding
		t.Mixed = false
		t.modTime, t.size = res.ModTime, res.Size
		t.Dirty = t.computeDirty()
		a.status = "Saved " + displayPath(*s.file)
		a.updateUnsaved()
	})
	return true
}

func (a *App) saveTabAs(id int) bool {
	a.setWriting(1)
	defer a.setWriting(-1)
	s, ok := a.snap(id)
	if !ok {
		return false
	}
	suggested := s.title + ".txt"
	if s.file != nil {
		suggested = baseName(s.file.Path)
		if s.file.Kind == "local" {
			suggested = s.file.Path
		}
	}
	p, err := a.Svc.ChooseSaveFile(suggested)
	if err != nil {
		a.showError("The file could not be saved.", err)
		return false
	}
	if p == "" {
		return false
	}
	file := services.EditorFile{Kind: "local", Path: p}
	// The native dialog has already asked before replacing a file.
	res, err := a.saveFile(file, s.text, editor.SaveOptions{Encoding: s.encoding, LineEnding: s.lineEnding, Force: true})
	if err != nil {
		a.showError(p+" could not be saved.", err)
		return false
	}
	a.ui(func() {
		t := a.tab(id)
		if t == nil {
			return
		}
		first, _, _ := strings.Cut(s.text, "\n")
		if lang := detectLanguage(p, first); lang != "" {
			t.LanguageID = lang
		}
		t.lexer = lexerFor(t.LanguageID)
		t.retoken = true
		f := file
		t.File, t.Title = &f, baseName(p)
		t.savedText = s.text
		t.savedEncoding, t.savedLineEnding = s.encoding, s.lineEnding
		t.Mixed = false
		t.modTime, t.size = res.ModTime, res.Size
		t.Dirty = t.computeDirty()
		a.status = "Saved " + p
		a.updateUnsaved()
	})
	return true
}

func (a *App) saveAll() bool {
	var ids []int
	a.ui(func() {
		for _, t := range a.tabs {
			if t.Dirty {
				ids = append(ids, t.ID)
			}
		}
	})
	for _, id := range ids {
		a.ui(func() { a.activate(id) })
		if !a.saveTab(id, false) {
			return false
		}
	}
	return true
}

func (a *App) reloadTab(id int, encoding string) {
	var title string
	var dirty, hasFile bool
	a.ui(func() {
		if t := a.tab(id); t != nil {
			title, dirty, hasFile = t.Title, t.Dirty, t.File != nil
		}
	})
	if !hasFile {
		return
	}
	if dirty && a.ask(question{Title: "Reload", Type: "warning", Message: "Discard the changes to " + title + " and reload it?",
		Choices: []Choice{{ID: "reload", Label: "Reload", Icon: iconRefresh}, {ID: "cancel", Label: "Cancel", Default: true}}}) != "reload" {
		return
	}
	a.loadInto(id, encoding, true)
}

// ---- changes on disk ---------------------------------------------------------------------

// busy: a load, save or question is in progress (UI goroutine).
func (a *App) busy() bool {
	if !a.ready || a.question != nil || a.writing > 0 {
		return true
	}
	for _, t := range a.tabs {
		if t.Loading != "" {
			return true
		}
	}
	return false
}

// Focused checks the open files for changes by other programs (the window
// got the focus), like Kate.
func (a *App) Focused() {
	if a.checking || a.busy() || time.Since(a.lastDisk) < time.Second {
		return
	}
	a.lastDisk, a.checking = time.Now(), true
	type item struct {
		id   int
		file services.EditorFile
	}
	var items []item
	for _, t := range a.tabs {
		if t.File != nil && t.ready && t.Loading == "" {
			items = append(items, item{t.ID, *t.File})
		}
	}
	a.flow(func() {
		defer a.ui(func() { a.checking = false })
		for _, it := range items {
			info, err := a.Svc.Stat(it.file)
			if err != nil {
				continue // e.g. the SFTP connection of a remote file is closed
			}
			var title string
			var dirty, gone, same, ignored, busy bool
			state := fmt.Sprintf("%d:%d", info.ModTime, info.Size)
			a.ui(func() {
				busy = a.busy()
				t := a.tab(it.id)
				if t == nil || t.File == nil {
					gone = true
					return
				}
				title, dirty = t.Title, t.Dirty
				same = info.ModTime == t.modTime && info.Size == t.size
				if same {
					delete(a.ignored, it.id)
				}
				if !info.Exists {
					ignored = a.ignored[it.id] == "deleted"
				} else {
					ignored = a.ignored[it.id] == state
				}
			})
			if busy {
				return // a save, load or dialog started meanwhile; next focus
			}
			if gone || ignored || (info.Exists && same) {
				continue
			}
			a.ui(func() { a.activate(it.id) })
			if !info.Exists {
				if a.ask(question{Title: "File deleted on disk", Type: "warning", Message: title + " was deleted or moved by another program.",
					Detail:  "Keep it open (saving writes it again) or close the tab?",
					Choices: []Choice{{ID: "keep", Label: "Keep Open", Default: true}, {ID: "close", Label: "Close Tab", Icon: iconClose}}}) == "close" {
					a.closeTab(it.id)
				} else {
					a.ui(func() { a.ignored[it.id] = "deleted" })
				}
				continue
			}
			detail := "Reload it to see the new content?"
			if dirty {
				detail = "Reloading discards your unsaved changes in LeanEdit."
			}
			if a.ask(question{Title: "File changed on disk", Type: "warning", Message: title + " was changed by another program.", Detail: detail,
				Choices: []Choice{{ID: "reload", Label: "Reload", Icon: iconRefresh, Default: !dirty}, {ID: "ignore", Label: "Ignore", Default: dirty}}}) == "reload" {
				a.ui(func() { delete(a.ignored, it.id) })
				a.loadInto(it.id, "", true)
			} else {
				a.ui(func() { a.ignored[it.id] = state })
			}
		}
	})
}

// ---- closing ---------------------------------------------------------------------------------

func (a *App) closeTab(id int) bool {
	var title string
	var dirty bool
	a.ui(func() {
		if t := a.tab(id); t != nil {
			title, dirty = t.Title, t.Dirty
			if dirty {
				a.activate(id)
			}
		}
	})
	if dirty {
		switch a.ask(question{Title: "Close Document", Type: "warning", Message: "Save the changes to " + title + " before closing?",
			Choices: []Choice{{ID: "save", Label: "Save", Icon: iconSave, Default: true}, {ID: "discard", Label: "Discard", Icon: iconTrash}, cancelChoice}}) {
		case "cancel":
			return false
		case "save":
			if !a.saveTab(id, false) {
				return false
			}
		}
	}
	a.ui(func() { a.closeTabNow(id) })
	return true
}

func (a *App) closeAll() bool {
	var ids []int
	a.ui(func() {
		for _, t := range a.tabs {
			ids = append(ids, t.ID)
		}
	})
	for _, id := range ids {
		if !a.closeTab(id) {
			return false
		}
	}
	return true
}

// RequestClose asks about unsaved changes and closes the window (Quit
// Editor, the window's close button).
func (a *App) RequestClose() {
	var dirty []string
	for _, t := range a.tabs {
		if t.Dirty {
			dirty = append(dirty, t.Title)
		}
	}
	a.flow(func() {
		if len(dirty) > 0 {
			msg := "Save the changes to " + dirty[0] + " before closing?"
			detail, save := "", "Save"
			if len(dirty) > 1 {
				msg = fmt.Sprintf("%d documents have unsaved changes. Save them before closing?", len(dirty))
				detail, save = strings.Join(dirty, "\n"), "Save All"
			}
			switch a.ask(question{Title: "Close Editor", Type: "warning", Message: msg, Detail: detail,
				Choices: []Choice{{ID: "save", Label: save, Icon: iconSave, Default: true}, {ID: "discard", Label: "Discard", Icon: iconTrash}, cancelChoice}}) {
			case "cancel":
				return
			case "save":
				if !a.saveAll() {
					return
				}
			}
		}
		a.Svc.SetUnsaved(false)
		a.Svc.CloseWindow()
	})
}

// ---- small helpers -----------------------------------------------------------------------------

func ftoa(f float32) string { return fmt.Sprintf("%g", f) }
func itoa(i int) string     { return fmt.Sprint(i) }
func btoa(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// lexerFor is the highlighter of a language id (nil = plain text).
func lexerFor(id string) lexer { return chromaLexer(id) }

// SettingsChanged reloads the editor settings (settings:changed; any goroutine).
func (a *App) SettingsChanged() {
	a.post(func() { a.settings = a.Svc.GetEditorSettings() })
}
