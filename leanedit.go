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
// running LeanTerm (no single instance), so it works independently of it and
// several LeanEdit windows can run at the same time.
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
	})
	emitter.app = wailsApp
	systemSvc.SetApp(wailsApp)
	editorSvc.SetApp(wailsApp, systemSvc.IsQuitting)

	workDir, _ := os.Getwd()
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
			log.Fatalf("cannot open the editor window: %v", err)
		}
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
