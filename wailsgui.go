package main

import (
	"errors"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"
)

const editorPrefsWindowName = "editor-prefs"

// wailsGUI is the web GUI's side of the services: native dialogs, quitting and
// LeanEdit's windows (services.Host and services.EditorWindows).
type wailsGUI struct {
	app        *application.App
	store      *config.Store
	editor     *services.EditorService
	isQuitting func() bool
	// windowSetup is called for every editor window (LeanEdit sets its own
	// icon on Windows with it).
	windowSetup func(*application.WebviewWindow)

	mu         sync.Mutex
	editorWin  *application.WebviewWindow
	prefsWin   *application.WebviewWindow
	allowClose bool
}

var (
	_ services.Host          = (*wailsGUI)(nil)
	_ services.EditorWindows = (*wailsGUI)(nil)
)

// windowBackground avoids a white flash before the frontend has painted.
func windowBackground(scheme string) application.RGBA {
	if scheme == "light" {
		return application.NewRGB(0xf0, 0xf0, 0xf0)
	}
	return application.NewRGB(0x2b, 0x2b, 0x2b)
}

func (g *wailsGUI) colorScheme() string {
	if g.store == nil {
		return ""
	}
	return g.store.Config().General.ColorScheme
}

// ---- services.Host ----------------------------------------------------------

func (g *wailsGUI) Quit() { g.app.Quit() }

func (g *wailsGUI) ChooseFile(d services.FileDialog) (string, error) {
	var win application.Window
	if d.Window == services.EditorWindowName {
		g.mu.Lock()
		if g.editorWin != nil {
			win = g.editorWin
		}
		g.mu.Unlock()
	} else if cur := g.app.Window.Current(); cur != nil {
		win = cur
	}
	if d.Save {
		dlg := g.app.Dialog.SaveFile().SetMessage(d.Title).CanCreateDirectories(true)
		if d.Filename != "" {
			dlg.SetFilename(d.Filename)
		}
		if d.Directory != "" {
			dlg.SetDirectory(d.Directory)
		}
		if win != nil {
			dlg.AttachToWindow(win)
		}
		return dlg.PromptForSingleSelection()
	}
	dlg := g.app.Dialog.OpenFile().
		SetTitle(d.Title).
		CanChooseFiles(!d.Folder).
		CanChooseDirectories(d.Folder).
		CanCreateDirectories(true).
		ShowHiddenFiles(d.ShowHidden)
	if d.Directory != "" {
		dlg.SetDirectory(d.Directory)
	}
	if win != nil && d.Window == services.EditorWindowName {
		dlg.AttachToWindow(win)
	}
	return dlg.PromptForSingleSelection()
}

// ---- services.EditorWindows -------------------------------------------------

func (g *wailsGUI) setupWindow(win *application.WebviewWindow) {
	if g.windowSetup != nil {
		g.windowSetup(win)
	}
}

func (g *wailsGUI) EditorShown() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.editorWin != nil
}

func (g *wailsGUI) ShowEditor() error {
	if g.app == nil {
		return errors.New("application not ready")
	}
	g.mu.Lock()
	win := g.editorWin
	g.mu.Unlock()
	if win != nil {
		win.Show()
		win.UnMinimise()
		win.Focus()
		return nil
	}
	g.createEditorWindow()
	return nil
}

func (g *wailsGUI) createEditorWindow() {
	geom := config.UIState{}.EditorWindow
	if g.store != nil {
		geom = g.store.LoadUIState().EditorWindow
	}
	opts := application.WebviewWindowOptions{
		Name:             services.EditorWindowName,
		Title:            services.EditorAppName,
		Width:            900,
		Height:           640,
		MinWidth:         480,
		MinHeight:        320,
		URL:              "/#/editor",
		BackgroundColour: windowBackground(g.colorScheme()),
	}
	if geom.Width >= 480 && geom.Height >= 320 {
		opts.Width, opts.Height = geom.Width, geom.Height
	}
	if geom.Maximized {
		opts.StartState = application.WindowStateMaximised
	}

	win := g.app.Window.NewWithOptions(opts)
	// The window checks its files for changes on disk whenever it gets the focus.
	win.OnWindowEvent(wailsevents.Common.WindowFocus, func(*application.WindowEvent) {
		win.EmitEvent("editor:focused")
	})
	win.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
		g.mu.Lock()
		quitting := g.isQuitting != nil && g.isQuitting()
		allowed := g.allowClose || quitting
		g.mu.Unlock()

		if !allowed {
			// The window asks about unsaved changes first and then calls CloseWindow.
			e.Cancel()
			win.EmitEvent("editor:close-requested")
			return
		}
		g.saveEditorGeometry(win)
		g.mu.Lock()
		g.editorWin = nil
		g.allowClose = false
		prefs := g.prefsWin
		g.mu.Unlock()
		g.editor.WindowClosed()

		// LeanEdit on its own: the editor window is the application, so closing
		// it also closes an open preferences window and quits.
		if g.editor.Standalone() {
			if prefs != nil {
				prefs.Close()
			}
			go g.app.Quit()
		}
	})

	g.mu.Lock()
	g.editorWin = win
	g.mu.Unlock()
	g.setupWindow(win)
}

func (g *wailsGUI) saveEditorGeometry(win *application.WebviewWindow) {
	if g.store == nil {
		return
	}
	st := g.store.LoadUIState()
	st.EditorWindow.Maximized = win.IsMaximised()
	if !st.EditorWindow.Maximized {
		st.EditorWindow.Width, st.EditorWindow.Height = win.Size()
	}
	_ = g.store.SaveUIState(st)
}

func (g *wailsGUI) CloseEditor() {
	g.mu.Lock()
	win := g.editorWin
	g.allowClose = true
	g.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

func (g *wailsGUI) ShowEditorPrefs() error {
	if g.app == nil {
		return errors.New("application not ready")
	}
	g.mu.Lock()
	win := g.prefsWin
	g.mu.Unlock()
	if win != nil {
		win.Show()
		win.UnMinimise()
		win.Focus()
		return nil
	}
	win = g.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             editorPrefsWindowName,
		Title:            services.EditorAppName + " Settings",
		Width:            560,
		Height:           620,
		MinWidth:         420,
		MinHeight:        360,
		URL:              "/#/editor-prefs",
		BackgroundColour: windowBackground(g.colorScheme()),
		InitialPosition:  application.WindowCentered,
	})
	win.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
		g.mu.Lock()
		if g.prefsWin == win {
			g.prefsWin = nil
		}
		g.mu.Unlock()
	})
	g.mu.Lock()
	g.prefsWin = win
	g.mu.Unlock()
	g.setupWindow(win)
	return nil
}

func (g *wailsGUI) CloseEditorPrefs() {
	g.mu.Lock()
	win := g.prefsWin
	g.mu.Unlock()
	if win != nil {
		win.Close()
	}
}
