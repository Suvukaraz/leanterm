package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/filedialog"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/leanedit"
	"github.com/Suvukaraz/leanterm/internal/gui/prefs"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// editorWindows is the native GUI's side of LeanEdit: its window runs its
// own Gio event loop next to the main window's.
type editorWindows struct {
	store      *config.Store
	svc        *services.EditorService
	chooseFile func(title, dir string, folder bool) (string, error)
	// standalone: LeanEdit is the only window; closing it quits.
	standalone bool
	onExit     func()

	mu      sync.Mutex
	w       *app.Window
	ed      *leanedit.App
	closing bool
	shapers map[string]*text.Shaper
	prefsW  *app.Window
}

var _ services.EditorWindows = (*editorWindows)(nil)

// ShowEditor creates the window or raises it.
func (e *editorWindows) ShowEditor() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.w != nil {
		e.w.Perform(system.ActionRaise)
		e.ed.TakePending()
		return nil
	}
	w := new(app.Window)
	st := e.store.LoadUIState().EditorWindow
	width, height := st.Width, st.Height
	if width < 400 || height < 300 {
		width, height = 900, 640
	}
	w.Option(app.Title("LeanEdit"), app.Size(unit.Dp(width), unit.Dp(height)), app.MinSize(400, 300))
	ed := leanedit.New(e.svc)
	ed.Invalidate = w.Invalidate
	ed.SetTitle = func(s string) { w.Option(app.Title(s)) }
	ed.Shaper = e.shaper
	e.w, e.ed, e.closing = w, ed, false
	go e.run(w, ed)
	return nil
}

// CloseEditor closes the window without asking again.
func (e *editorWindows) CloseEditor() {
	e.mu.Lock()
	w := e.w
	e.closing = true
	e.mu.Unlock()
	if w != nil {
		w.Perform(system.ActionClose)
	}
}

// EditorShown reports whether the window is open.
func (e *editorWindows) EditorShown() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.w != nil
}

// ShowEditorPrefs opens LeanEdit's settings window or raises it.
func (e *editorWindows) ShowEditorPrefs() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.prefsW != nil {
		e.prefsW.Perform(system.ActionRaise)
		return nil
	}
	w := new(app.Window)
	w.Option(app.Title(services.EditorAppName+" Settings"), app.Size(560, 620), app.MinSize(420, 360))
	p := &prefs.EditorPrefs{
		Load:  e.svc.GetEditorSettings,
		Save:  e.svc.SaveEditorSettings,
		Close: func() { w.Perform(system.ActionClose) },
	}
	p.F().ChooseFile = func(title, dir string, folder bool) (string, error) {
		return e.chooseFile(title, dir, folder)
	}
	p.F().Changed = w.Invalidate
	e.prefsW = w
	go func() {
		scheme := e.store.Config().General.ColorScheme
		th := gtk.New(scheme)
		var ops op.Ops
		for {
			switch ev := w.Event().(type) {
			case app.DestroyEvent:
				e.mu.Lock()
				e.prefsW = nil
				e.mu.Unlock()
				return
			case app.FrameEvent:
				if s := e.store.Config().General.ColorScheme; s != scheme {
					scheme = s
					th.SetScheme(s)
				}
				gtx := app.NewContext(&ops, ev)
				p.Layout(gtx, th)
				ev.Frame(gtx.Ops)
			}
		}
	}()
	return nil
}

// CloseEditorPrefs closes the settings window.
func (e *editorWindows) CloseEditorPrefs() {
	e.mu.Lock()
	w := e.prefsW
	e.mu.Unlock()
	if w != nil {
		w.Perform(system.ActionClose)
	}
}

// pending opens queued files (events.EditorPending).
func (e *editorWindows) pending() {
	e.mu.Lock()
	ed := e.ed
	e.mu.Unlock()
	if ed != nil {
		ed.TakePending()
	}
}

func (e *editorWindows) settingsChanged() {
	e.mu.Lock()
	ed, w := e.ed, e.w
	e.mu.Unlock()
	if ed != nil {
		ed.SettingsChanged()
		w.Invalidate()
	}
}

// shaper shapes the editor font (called on the editor's goroutine).
func (e *editorWindows) shaper(family string) *text.Shaper {
	if family == "" {
		return nil
	}
	if sh, ok := e.shapers[family]; ok {
		return sh
	}
	sh := familyShaper(family)
	if e.shapers == nil {
		e.shapers = map[string]*text.Shaper{}
	}
	e.shapers[family] = sh
	return sh
}

func (e *editorWindows) run(w *app.Window, ed *leanedit.App) {
	scheme := e.store.Config().General.ColorScheme
	th := gtk.New(scheme)
	var ops op.Ops
	var size [2]int
	for {
		switch ev := w.Event().(type) {
		case app.DestroyEvent:
			if ev.Err != nil {
				log.Print("LeanEdit window: ", ev.Err)
			}
			st := e.store.LoadUIState()
			if size[0] > 0 {
				st.EditorWindow.Width, st.EditorWindow.Height = size[0], size[1]
				_ = e.store.SaveUIState(st)
			}
			e.mu.Lock()
			e.w, e.ed = nil, nil
			e.mu.Unlock()
			e.svc.WindowClosed()
			if e.standalone && e.onExit != nil {
				e.onExit()
			}
			return
		case *app.ClosingEvent:
			e.mu.Lock()
			closing := e.closing
			e.mu.Unlock()
			if !closing {
				// The editor asks about unsaved changes and closes itself.
				ev.Abort()
				ed.RequestClose()
				w.Invalidate()
			}
		case app.ConfigEvent:
			if ev.Config.Focused {
				ed.Focused()
			}
		case app.FrameEvent:
			if s := e.store.Config().General.ColorScheme; s != scheme {
				scheme = s
				th.SetScheme(s)
			}
			gtx := app.NewContext(&ops, ev)
			size = [2]int{int(float32(ev.Size.X) / gtx.Metric.PxPerDp), int(float32(ev.Size.Y) / gtx.Metric.PxPerDp)}
			ed.Layout(gtx, th)
			ev.Frame(gtx.Ops)
		}
	}
}

// watchEditorEvents connects the event bus to the editor window.
func (e *editorWindows) watchEditorEvents(bus *events.Bus) {
	bus.On(events.EditorPending, func(string, any) { e.pending() })
	bus.On(events.SettingsChanged, func(string, any) { e.settingsChanged() })
}

// leanEditHost is the services.Host of LeanEdit on its own: file dialogs
// and quitting.
type leanEditHost struct{}

func (leanEditHost) ChooseFile(d services.FileDialog) (string, error) {
	return filedialog.Choose(filedialog.Options{Title: d.Title, Directory: d.Directory, Filename: d.Filename, Save: d.Save, Folder: d.Folder})
}
func (leanEditHost) Quit() { os.Exit(0) }

// runLeanEdit starts only the editor (leanterm-native -leanedit [files]):
// no main window and no sessions; closing the window quits. Its settings
// go straight to config.toml (EditorService standalone).
func runLeanEdit(configDir string, files []string) {
	store, err := config.NewStore(config.StoreOptions{ConfigDir: configDir, NoBackup: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Load(); err != nil {
		log.Fatal(err)
	}
	bus := events.New()
	svc := services.NewEditorService(store, nil, bus)
	svc.SetStandalone(true)
	// Settings saved by LeanTerm or another LeanEdit apply right away.
	store.OnReload = func() { bus.Emit(events.SettingsChanged, nil) }
	host := leanEditHost{}
	ew := &editorWindows{store: store, svc: svc, standalone: true, onExit: func() { os.Exit(0) },
		chooseFile: func(title, dir string, folder bool) (string, error) {
			return host.ChooseFile(services.FileDialog{Title: title, Directory: dir, Folder: folder})
		}}
	svc.SetGUI(host, ew)
	ew.watchEditorEvents(bus)
	go func() {
		opened := false
		for _, f := range files {
			if abs, err := filepath.Abs(f); err == nil {
				if err := svc.Open(services.EditorFile{Kind: "local", Path: abs}); err == nil {
					opened = true
				}
			}
		}
		if !opened {
			if err := svc.ShowWindow(); err != nil {
				log.Fatal(err)
			}
		}
	}()
	app.Main()
}
