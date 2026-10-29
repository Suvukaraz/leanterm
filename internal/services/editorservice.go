package services

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/editor"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// EditorFile identifies a file opened in the editor window.
type EditorFile struct {
	// Kind is "local" or "sftp".
	Kind string `json:"kind"`
	// SFTPID is the SFTP session of a remote file (SFTPService.OpenForSession).
	SFTPID string `json:"sftpId,omitempty"`
	// ConnID is the saved connection of a remote file: after a reconnect (new
	// session) the editor continues with the new session of the same connection.
	ConnID string `json:"connId,omitempty"`
	// Host labels remote files (tab tooltip, status bar).
	Host string `json:"host,omitempty"`
	Path string `json:"path"`
}

const editorWindowName = "editor"

// EditorService backs the text editor: one extra window with a tab per file,
// fed by Open from the file browsers. File contents travel in chunks
// (Load/ReadChunk, BeginSave/AppendSave/CommitSave) so large files do not
// block the bridge.
type EditorService struct {
	mu         sync.Mutex
	app        *application.App
	store      *config.Store
	sftp       *SFTPService
	emitter    settingsEmitter
	isQuitting func() bool
	files      *editor.Manager

	window     *application.WebviewWindow
	allowClose bool
	pending    []EditorFile
	unsaved    bool
}

func NewEditorService(store *config.Store, sftp *SFTPService, emitter settingsEmitter) *EditorService {
	return &EditorService{store: store, sftp: sftp, emitter: emitter, files: editor.NewManager()}
}

// SetApp gives the service the application (for creating the window and dialogs).
// isQuitting reports whether LeanTerm is shutting down (closing is not intercepted then).
func (es *EditorService) SetApp(app *application.App, isQuitting func() bool) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.app = app
	es.isQuitting = isQuitting
}

// ---- Window -----------------------------------------------------------------

// Open queues a file for the editor window, creating or raising the window.
// The window picks queued files up with TakePending ("editor:pending").
func (es *EditorService) Open(f EditorFile) error {
	if f.Kind != "local" && f.Kind != "sftp" {
		return errors.New("unknown file kind")
	}
	if f.Path == "" {
		return errors.New("no file given")
	}
	es.mu.Lock()
	es.pending = append(es.pending, f)
	es.mu.Unlock()
	return es.showWindow(true)
}

// ShowWindow opens the editor window (empty if no files are open) and raises it.
func (es *EditorService) ShowWindow() error {
	return es.showWindow(false)
}

func (es *EditorService) showWindow(notify bool) error {
	es.mu.Lock()
	app := es.app
	win := es.window
	es.mu.Unlock()
	if app == nil {
		return errors.New("application not ready")
	}
	if win == nil {
		win = es.createWindow(app)
	} else {
		win.Show()
		win.UnMinimise()
		win.Focus()
	}
	if notify && es.emitter != nil {
		es.emitter.Emit("editor:pending", nil)
	}
	return nil
}

func (es *EditorService) createWindow(app *application.App) *application.WebviewWindow {
	geom := config.UIState{}.EditorWindow
	scheme := ""
	if es.store != nil {
		geom = es.store.LoadUIState().EditorWindow
		scheme = es.store.Config().General.ColorScheme
	}
	bg := application.NewRGB(0x2b, 0x2b, 0x2b)
	if scheme == "light" {
		bg = application.NewRGB(0xf0, 0xf0, 0xf0)
	}
	opts := application.WebviewWindowOptions{
		Name:             editorWindowName,
		Title:            "LeanTerm Editor",
		Width:            900,
		Height:           640,
		MinWidth:         480,
		MinHeight:        320,
		URL:              "/#/editor",
		BackgroundColour: bg,
	}
	if geom.Width >= 480 && geom.Height >= 320 {
		opts.Width, opts.Height = geom.Width, geom.Height
	}
	if geom.Maximized {
		opts.StartState = application.WindowStateMaximised
	}

	win := app.Window.NewWithOptions(opts)
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		es.mu.Lock()
		quitting := es.isQuitting != nil && es.isQuitting()
		allowed := es.allowClose || quitting
		es.mu.Unlock()

		if !allowed {
			// The window asks about unsaved changes first and then calls CloseWindow.
			e.Cancel()
			win.EmitEvent("editor:close-requested")
			return
		}
		es.saveGeometry(win)
		es.mu.Lock()
		es.window = nil
		es.allowClose = false
		es.unsaved = false
		es.pending = nil
		es.mu.Unlock()
	})

	es.mu.Lock()
	es.window = win
	es.mu.Unlock()
	return win
}

func (es *EditorService) saveGeometry(win *application.WebviewWindow) {
	if es.store == nil {
		return
	}
	st := es.store.LoadUIState()
	st.EditorWindow.Maximized = win.IsMaximised()
	if !st.EditorWindow.Maximized {
		st.EditorWindow.Width, st.EditorWindow.Height = win.Size()
	}
	_ = es.store.SaveUIState(st)
}

// TakePending returns and clears the files queued by Open.
func (es *EditorService) TakePending() []EditorFile {
	es.mu.Lock()
	defer es.mu.Unlock()
	out := es.pending
	es.pending = nil
	if out == nil {
		out = []EditorFile{}
	}
	return out
}

// CloseWindow closes the editor window without asking again (the window has
// already dealt with unsaved changes).
func (es *EditorService) CloseWindow() {
	es.mu.Lock()
	win := es.window
	es.allowClose = true
	es.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// ShowPreferences raises the main window and opens Preferences there on the
// Editor page ("editor:configure").
func (es *EditorService) ShowPreferences() {
	es.mu.Lock()
	app := es.app
	es.mu.Unlock()
	if app != nil {
		if w, ok := app.Window.GetByName("main"); ok {
			w.Show()
			w.UnMinimise()
			w.Focus()
		}
	}
	if es.emitter != nil {
		es.emitter.Emit("editor:configure", nil)
	}
}

// SetUnsaved is called by the editor window whenever its unsaved state changes.
func (es *EditorService) SetUnsaved(unsaved bool) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.unsaved = unsaved
}

// HasUnsaved reports whether the editor window has unsaved changes (asked by
// the main window before LeanTerm quits).
func (es *EditorService) HasUnsaved() bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.window != nil && es.unsaved
}

// ChooseOpenFile shows a file chooser on the editor window ("" = cancelled).
func (es *EditorService) ChooseOpenFile(startDir string) (string, error) {
	es.mu.Lock()
	app, win := es.app, es.window
	es.mu.Unlock()
	if app == nil {
		return "", errors.New("application not ready")
	}
	dlg := app.Dialog.OpenFile().SetTitle("Open File").CanChooseFiles(true).ShowHiddenFiles(true)
	if dir := startDirectory(startDir); dir != "" {
		dlg.SetDirectory(dir)
	}
	if win != nil {
		dlg.AttachToWindow(win)
	}
	return dlg.PromptForSingleSelection()
}

// ChooseSaveFile shows a "save as" dialog on the editor window ("" = cancelled).
// The dialog itself asks before replacing an existing file.
func (es *EditorService) ChooseSaveFile(suggested string) (string, error) {
	es.mu.Lock()
	app, win := es.app, es.window
	es.mu.Unlock()
	if app == nil {
		return "", errors.New("application not ready")
	}
	dlg := app.Dialog.SaveFile().SetMessage("Save As").CanCreateDirectories(true)
	dir, name := "", suggested
	if suggested != "" && filepath.IsAbs(suggested) {
		dir, name = filepath.Dir(suggested), filepath.Base(suggested)
	}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if dir != "" {
		dlg.SetDirectory(dir)
	}
	if name != "" {
		dlg.SetFilename(name)
	}
	if win != nil {
		dlg.AttachToWindow(win)
	}
	return dlg.PromptForSingleSelection()
}

// ---- Files ------------------------------------------------------------------

func (es *EditorService) fsFor(f EditorFile) (editor.FS, error) {
	switch f.Kind {
	case "local":
		return editor.LocalFS{}, nil
	case "sftp":
		if es.sftp == nil {
			return nil, errors.New("SFTP is not available")
		}
		return es.sftp.editorFS(f.SFTPID, f.ConnID)
	}
	return nil, errors.New("unknown file kind")
}

// Stat returns size and modification time (Exists=false when the file is missing).
func (es *EditorService) Stat(f EditorFile) (editor.FileInfo, error) {
	fsys, err := es.fsFor(f)
	if err != nil {
		return editor.FileInfo{}, err
	}
	return fsys.Stat(f.Path)
}

// Load reads a file; encoding "" detects it. Binary-looking files are only
// loaded with allowBinary. The text is then fetched with ReadChunk.
func (es *EditorService) Load(f EditorFile, encoding string, allowBinary bool) (editor.LoadResult, error) {
	fsys, err := es.fsFor(f)
	if err != nil {
		return editor.LoadResult{}, err
	}
	return es.files.Load(fsys, f.Path, encoding, allowBinary)
}

// ReadChunk returns the next part of a loaded text.
func (es *EditorService) ReadChunk(loadID string) (editor.Chunk, error) {
	return es.files.ReadChunk(loadID)
}

// BeginSave starts a save; the text follows with AppendSave.
func (es *EditorService) BeginSave() string {
	return es.files.BeginSave()
}

// AppendSave adds the next part of the text of a save.
func (es *EditorService) AppendSave(saveID, text string) error {
	return es.files.AppendSave(saveID, text)
}

// CommitSave writes the collected text (see editor.SaveOptions for conflicts).
func (es *EditorService) CommitSave(saveID string, f EditorFile, opts editor.SaveOptions) (editor.SaveResult, error) {
	fsys, err := es.fsFor(f)
	if err != nil {
		es.files.Discard(saveID)
		return editor.SaveResult{}, err
	}
	return es.files.CommitSave(saveID, fsys, f.Path, opts)
}

// Discard drops an unfinished load or save.
func (es *EditorService) Discard(id string) {
	es.files.Discard(id)
}
