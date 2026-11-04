package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Suvukaraz/leanterm/internal/app"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed build/linux/icons/leanedit-256.png
var leanEditIcon []byte

// leanEditAppID is LeanEdit's application id: GTK application id / desktop file
// name on Linux, AppUserModelID on Windows (own task bar entry and icon).
const leanEditAppID = "org.leanterm.LeanEdit"

// runLeanEdit starts only the text editor as an application of its own: no
// main window, no sessions, clusters, KeePass or secrets. It never joins a
// running LeanTerm, so it works independently of it.
//
// With "single instance" in its settings (the default) the first LeanEdit owns
// the instance: later starts (desktop, "Open with", command line) hand their
// files over and quit, and each file opens as a tab there. Otherwise, or with
// --new-window, every start opens its own window.
func runLeanEdit(flags *app.CLIFlags, cfgStore *config.Store) {
	app.SetAppUserModelID(leanEditAppID)

	emitter := &wailsEmitter{}
	systemSvc := services.NewSystemService(nil)
	systemSvc.SetStore(cfgStore)
	editorSvc := services.NewEditorService(cfgStore, nil, emitter)
	editorSvc.SetStandalone(true)
	editorSvc.SetWindowSetup(func(w *application.WebviewWindow) {
		w.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
			app.SetWindowIcon(w.NativeWindow(), leanEditIcon)
		})
	})
	// Settings saved by LeanTerm or another LeanEdit window apply right away.
	cfgStore.OnReload = func() { emitter.Emit("settings:changed", nil) }

	var singleInstance *application.SingleInstanceOptions
	if cfgStore.Config().Editor.SingleInstance && !flags.NewWindow && app.SingleInstancePossible() {
		// When this start only hands its files over, the running LeanEdit may
		// take the focus (Windows only lets the process just started do that).
		app.AllowForegroundForOthers()
		singleInstance = &application.SingleInstanceOptions{
			UniqueID: app.LeanEditInstanceID(leanEditAppID, cfgStore.ConfigDir()),
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				args := data.Args
				if len(args) > 0 {
					args = args[1:] // the program itself
				}
				openLeanEditFiles(editorSvc, app.ParseCLI(args), data.WorkingDir)
			},
		}
	}

	wailsApp := application.New(application.Options{
		Name:        services.EditorAppName,
		Description: "LeanEdit text editor (part of LeanTerm)",
		Icon:        leanEditIcon,
		Services: []application.Service{
			application.NewService(systemSvc),
			application.NewService(editorSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ApplicationID: leanEditAppID,
		},
		SingleInstance: singleInstance,
	})
	emitter.app = wailsApp
	systemSvc.SetApp(wailsApp)
	editorSvc.SetApp(wailsApp, systemSvc.IsQuitting)

	workDir, _ := os.Getwd()
	openLeanEditFiles(editorSvc, flags, workDir)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// openLeanEditFiles opens the files of a (first or later) start as tabs, or
// just shows and raises the window when there are none.
func openLeanEditFiles(editorSvc *services.EditorService, flags *app.CLIFlags, workDir string) {
	files := append(append([]string{}, flags.EditFiles...), flags.Files...)
	opened := false
	for _, f := range files {
		if !filepath.IsAbs(f) && workDir != "" {
			f = filepath.Join(workDir, f)
		}
		if err := editorSvc.Open(services.EditorFile{Kind: "local", Path: f}); err != nil {
			fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", f, err)
			continue
		}
		opened = true
	}
	if !opened {
		if err := editorSvc.ShowWindow(); err != nil {
			log.Printf("cannot open the editor window: %v", err)
		}
	}
}
