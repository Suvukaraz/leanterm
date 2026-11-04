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

const (
	editorWindowName      = "editor"
	editorPrefsWindowName = "editor-prefs"
	// EditorAppName is the editor's name, also when it runs inside LeanTerm.
	EditorAppName = "LeanEdit"
)

// EditorService backs the text editor (LeanEdit): one window with a tab per
// file, fed by Open from the file browsers or the command line. File contents
// travel in chunks (Load/ReadChunk, BeginSave/AppendSave/CommitSave) so large
// files do not block the bridge.
//
// Inside LeanTerm the window is an extra window of LeanTerm. Started on its own
// (leanterm --leanedit) it is the only window of a separate process: closing it
// quits that process, and its settings are written straight to config.toml.
type EditorService struct {
	mu         sync.Mutex
	app        *application.App
	store      *config.Store
	sftp       *SFTPService
	emitter    settingsEmitter
	isQuitting func() bool
	files      *editor.Manager

	window      *application.WebviewWindow
	prefsWindow *application.WebviewWindow
	allowClose  bool
	pending     []EditorFile
	unsaved     bool
	standalone  bool
	// windowSetup is called for every window the service creates (LeanEdit sets
	// its own icon on Windows with it).
	windowSetup func(*application.WebviewWindow)
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

// SetStandalone switches the service to LeanEdit's own process (see the type).
func (es *EditorService) SetStandalone(standalone bool) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.standalone = standalone
}

// SetWindowSetup registers a function that is called for every new window.
func (es *EditorService) SetWindowSetup(setup func(*application.WebviewWindow)) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.windowSetup = setup
}

func (es *EditorService) setupWindow(win *application.WebviewWindow) {
	es.mu.Lock()
	setup := es.windowSetup
	es.mu.Unlock()
	if setup != nil {
		setup(win)
	}
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
		Title:            EditorAppName,
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
		standalone, prefs := es.standalone, es.prefsWindow
		es.mu.Unlock()

		// LeanEdit on its own: the editor window is the application, so closing
		// it also closes an open preferences window and quits.
		if standalone {
			if prefs != nil {
				prefs.Close()
			}
			go app.Quit()
		}
	})

	es.mu.Lock()
	es.window = win
	es.mu.Unlock()
	es.setupWindow(win)
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

// ShowPreferences opens LeanEdit's own preferences window (only the editor
// settings) or raises it when it is already open.
func (es *EditorService) ShowPreferences() error {
	es.mu.Lock()
	app, win := es.app, es.prefsWindow
	es.mu.Unlock()
	if app == nil {
		return errors.New("application not ready")
	}
	if win != nil {
		win.Show()
		win.UnMinimise()
		win.Focus()
		return nil
	}

	scheme := ""
	if es.store != nil {
		scheme = es.store.Config().General.ColorScheme
	}
	bg := application.NewRGB(0x2b, 0x2b, 0x2b)
	if scheme == "light" {
		bg = application.NewRGB(0xf0, 0xf0, 0xf0)
	}
	win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             editorPrefsWindowName,
		Title:            EditorAppName + " Preferences",
		Width:            560,
		Height:           620,
		MinWidth:         420,
		MinHeight:        360,
		URL:              "/#/editor-prefs",
		BackgroundColour: bg,
		InitialPosition:  application.WindowCentered,
	})
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		es.mu.Lock()
		if es.prefsWindow == win {
			es.prefsWindow = nil
		}
		es.mu.Unlock()
	})
	es.mu.Lock()
	es.prefsWindow = win
	es.mu.Unlock()
	es.setupWindow(win)
	return nil
}

// ClosePreferences closes the preferences window (OK / Cancel).
func (es *EditorService) ClosePreferences() {
	es.mu.Lock()
	win := es.prefsWindow
	es.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// EditorSettings is what the editor and its preferences window need from the
// configuration.
type EditorSettings struct {
	Editor      config.EditorConfig `json:"editor"`
	ColorScheme string              `json:"colorScheme"`
	IconTheme   string              `json:"iconTheme"`
	// Standalone is true in LeanEdit's own process (no LeanTerm file browsers).
	Standalone bool `json:"standalone"`
}

// GetEditorSettings returns the current editor settings.
func (es *EditorService) GetEditorSettings() EditorSettings {
	es.mu.Lock()
	standalone := es.standalone
	es.mu.Unlock()
	out := EditorSettings{Standalone: standalone}
	if es.store != nil {
		cfg := es.store.Config()
		out.Editor = cfg.Editor
		out.ColorScheme = cfg.General.ColorScheme
		out.IconTheme = cfg.General.Theme
	}
	return out
}

// SaveEditorSettings stores the editor settings and tells all windows
// ("settings:changed"). Invalid values are rejected with a readable error.
func (es *EditorService) SaveEditorSettings(ed config.EditorConfig) error {
	if es.store == nil {
		return errors.New("no configuration")
	}
	es.mu.Lock()
	standalone := es.standalone
	es.mu.Unlock()

	if standalone {
		// Only the [editor] section of the file on disk changes; LeanTerm (if it
		// runs) reloads the file.
		if err := es.store.UpdateOnDisk(func(cfg *config.Config) { cfg.Editor = ed }); err != nil {
			return err
		}
	} else {
		cfg := es.store.Config()
		before := cfg.Editor
		cfg.Editor = ed
		if res := config.Validate(cfg); res.HasErrors() {
			cfg.Editor = before
			return res.Error()
		}
		es.store.MarkDirty()
	}
	if es.emitter != nil {
		es.emitter.Emit("settings:changed", nil)
	}
	return nil
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
