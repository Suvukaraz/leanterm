package main

import (
	"log"
	"sync"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/leanedit"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// editorWindows is the native GUI's side of LeanEdit: its window runs its
// own Gio event loop next to the main window's.
type editorWindows struct {
	u *ui
	// standalone: LeanEdit is the only window; closing it quits.
	standalone bool
	onExit     func()

	mu      sync.Mutex
	w       *app.Window
	ed      *leanedit.App
	closing bool
	shapers map[string]*text.Shaper
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
	st := e.u.core.Store.LoadUIState().EditorWindow
	width, height := st.Width, st.Height
	if width < 400 || height < 300 {
		width, height = 900, 640
	}
	w.Option(app.Title("LeanEdit"), app.Size(unit.Dp(width), unit.Dp(height)), app.MinSize(400, 300))
	ed := leanedit.New(e.u.editorSvc)
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

// ShowEditorPrefs opens LeanEdit's settings (stage 7c: in the editor window).
func (e *editorWindows) ShowEditorPrefs() error {
	e.mu.Lock()
	ed, w := e.ed, e.w
	e.mu.Unlock()
	if ed == nil {
		return nil
	}
	ed.ShowPreferences()
	w.Invalidate()
	return nil
}

func (e *editorWindows) CloseEditorPrefs() {}

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
	scheme := e.u.core.Store.Config().General.ColorScheme
	th := gtk.New(scheme)
	var ops op.Ops
	var size [2]int
	for {
		switch ev := w.Event().(type) {
		case app.DestroyEvent:
			if ev.Err != nil {
				log.Print("LeanEdit window: ", ev.Err)
			}
			st := e.u.core.Store.LoadUIState()
			if size[0] > 0 {
				st.EditorWindow.Width, st.EditorWindow.Height = size[0], size[1]
				_ = e.u.core.Store.SaveUIState(st)
			}
			e.mu.Lock()
			e.w, e.ed = nil, nil
			e.mu.Unlock()
			e.u.editorSvc.WindowClosed()
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
			if s := e.u.core.Store.Config().General.ColorScheme; s != scheme {
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
