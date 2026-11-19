package services

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/editor"
	"github.com/Suvukaraz/leanterm/internal/events"
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

// EditorAppName is the editor's name, also when it runs inside LeanTerm.
const EditorAppName = "LeanEdit"

// EditorService backs the text editor (LeanEdit): one window with a tab per
// file, fed by Open from the file browsers or the command line. File contents
// travel in chunks (Load/ReadChunk, BeginSave/AppendSave/CommitSave) so large
// files do not block the bridge.
//
// Inside LeanTerm the window is an extra window of LeanTerm. Started on its own
// (leanterm --leanedit) it is the only window of a separate process: closing it
// quits that process, and its settings are written straight to config.toml.
type EditorService struct {
	mu      sync.Mutex
	host    Host
	windows EditorWindows
	store   *config.Store
	sftp    *SFTPService
	emitter settingsEmitter
	files   *editor.Manager

	pending    []EditorFile
	unsaved    bool
	standalone bool
}

func NewEditorService(store *config.Store, sftp *SFTPService, emitter settingsEmitter) *EditorService {
	return &EditorService{store: store, sftp: sftp, emitter: emitter, files: editor.NewManager()}
}

// SetGUI gives the service the GUI it runs in: dialogs and its windows.
func (es *EditorService) SetGUI(host Host, windows EditorWindows) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.host = host
	es.windows = windows
}

// SetStandalone switches the service to LeanEdit's own process (see the type).
func (es *EditorService) SetStandalone(standalone bool) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.standalone = standalone
}

// Standalone reports whether LeanEdit runs as its own process: closing the
// editor window then quits.
func (es *EditorService) Standalone() bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.standalone
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
	windows := es.windows
	es.mu.Unlock()
	if windows == nil {
		return errors.New("application not ready")
	}
	if err := windows.ShowEditor(); err != nil {
		return err
	}
	if notify && es.emitter != nil {
		es.emitter.Emit(events.EditorPending, nil)
	}
	return nil
}

// WindowClosed is called by the GUI when the editor window has closed: queued
// files and the unsaved state belong to that window.
func (es *EditorService) WindowClosed() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.unsaved = false
	es.pending = nil
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

// NewWindow starts another LeanEdit with its own window (File → New Window),
// also when LeanEdit runs as a single instance, and from LeanTerm's editor.
func (es *EditorService) NewWindow() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"--leanedit", "--new-window"}
	if es.store != nil {
		args = append(args, "--config-dir="+es.store.ConfigDir())
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reaps the process when that window is closed
	return nil
}

// CloseWindow closes the editor window without asking again (the window has
// already dealt with unsaved changes).
func (es *EditorService) CloseWindow() {
	es.mu.Lock()
	windows := es.windows
	es.mu.Unlock()
	if windows != nil {
		windows.CloseEditor()
	}
}

// ShowPreferences opens LeanEdit's own settings window (only the editor
// settings) or raises it when it is already open.
func (es *EditorService) ShowPreferences() error {
	es.mu.Lock()
	windows := es.windows
	es.mu.Unlock()
	if windows == nil {
		return errors.New("application not ready")
	}
	return windows.ShowEditorPrefs()
}

// ClosePreferences closes the settings window (OK / Cancel).
func (es *EditorService) ClosePreferences() {
	es.mu.Lock()
	windows := es.windows
	es.mu.Unlock()
	if windows != nil {
		windows.CloseEditorPrefs()
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
		es.emitter.Emit(events.SettingsChanged, nil)
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
	windows, unsaved := es.windows, es.unsaved
	es.mu.Unlock()
	return unsaved && windows != nil && windows.EditorShown()
}

// ChooseOpenFile shows a file chooser on the editor window ("" = cancelled).
func (es *EditorService) ChooseOpenFile(startDir string) (string, error) {
	es.mu.Lock()
	host := es.host
	es.mu.Unlock()
	if host == nil {
		return "", errors.New("application not ready")
	}
	return host.ChooseFile(FileDialog{Title: "Open File", Directory: startDirectory(startDir), ShowHidden: true, Window: EditorWindowName})
}

// ChooseSaveFile shows a "save as" dialog on the editor window ("" = cancelled).
// The dialog itself asks before replacing an existing file.
func (es *EditorService) ChooseSaveFile(suggested string) (string, error) {
	es.mu.Lock()
	host := es.host
	es.mu.Unlock()
	if host == nil {
		return "", errors.New("application not ready")
	}
	dir, name := "", suggested
	if suggested != "" && filepath.IsAbs(suggested) {
		dir, name = filepath.Dir(suggested), filepath.Base(suggested)
	}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	return host.ChooseFile(FileDialog{Title: "Save As", Directory: dir, Filename: name, Save: true, Window: EditorWindowName})
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
