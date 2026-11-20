// Command leanterm-native is the Gio frontend of LeanTerm: the full main
// window from the web GUI, drawn with the gtk widget set (internal/gui/gtk),
// on top of the same backend (internal/gui/appcore) the web GUI uses.
//
// Layout like MainWindow.svelte: the connection tree on the left (or right),
// the terminal notebook next to it, the button bar, the status bar and the
// Info tab of the web GUI with the welcome page and the connection's notes
// and statistics.
package main

import (
	"flag"
	"github.com/Suvukaraz/leanterm/internal/gui/filedialog"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/appcore"
	"github.com/Suvukaraz/leanterm/internal/gui/dialogs"
	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/Suvukaraz/leanterm/internal/session"
)

func main() {
	configDir := flag.String("config-dir", "", "configuration folder (default: LeanTerm's)")
	readOnly := flag.Bool("readonly", false, "read-only configuration (nothing is saved)")
	noBackup := flag.Bool("no-backup", false, "do not keep backups of the configuration")
	password := flag.String("password", "", "master password for secrets stored with one")
	startShell := flag.Bool("start-shell", false, "open a local shell at startup")
	startUUID := flag.String("start-uuid", "", "comma-separated saved connections (id or name, id:cluster) to open")
	engine := flag.String("engine", "", "terminal engine for connections: ghostty or vt (default: "+term.Default()+")")
	editFile := flag.String("edit", "", "open this file in LeanEdit at start (testing)")
	quitAfter := flag.Duration("quit-after", 0, "self-test: quit after this time")
	stats := flag.Bool("stats", false, "log the frames drawn per second")
	leanEdit := flag.Bool("leanedit", false, "run only LeanEdit (files as arguments)")
	flag.Parse()
	if *leanEdit {
		runLeanEdit(*configDir, flag.Args())
		return
	}

	core, err := appcore.Open(appcore.Options{ConfigDir: *configDir, MasterPassword: *password, ReadOnly: *readOnly, NoBackup: *noBackup})
	if err != nil {
		log.Fatal(err)
	}
	cfg := core.Store.Config()

	w := new(app.Window)
	title := "LeanTerm"
	if *readOnly {
		title = "LeanTerm - READ ONLY MODE"
	}
	uiState := core.Store.LoadUIState()
	width, height := uiState.MainWindow.Width, uiState.MainWindow.Height
	if width < 640 || height < 420 {
		width, height = 1200, 760
	}
	w.Option(app.Title(title), app.Size(unit.Dp(width), unit.Dp(height)), app.MinSize(640, 420))

	u := &ui{
		w:         w,
		core:      core,
		opened:    make(chan session.SessionInfo, 16),
		closed:    make(chan string, 16),
		engine:    *engine,
		stats:     *stats,
		drops:     make(chan dnd.Drop, 4),
		readOnly:  *readOnly,
		splits:    map[string]*splitState{},
		newData:   map[string]bool{},
		frameSize: image.Point{X: width, Y: height},
	}
	u.panel = gtk.NewPaned()
	// General.ColorScheme decides the palette; SettingsChanged (event) switches it live.
	u.th = gtk.New(cfg.General.ColorScheme)
	u.prompter = dialogs.NewPrompter(w.Invalidate)
	u.configSvc = services.NewConfigService(core.Store, core.Vault, core.Tree)
	u.configSvc.SetEmitter(core.Bus)
	u.sessionSvc = services.NewSessionService(core.Store, core.Sessions)
	sftpMgr := services.NewSFTPService(nil, core.Sessions, core.Bus)
	sftpMgr.SetStore(core.Store)
	u.editorSvc = services.NewEditorService(core.Store, sftpMgr, core.Bus)
	u.sftpSvc = sftpMgr
	u.localFS = services.NewLocalFSService(core.Sessions, core.Bus)
	u.localFS.SetStore(core.Store)
	u.filesEvents = make(chan func(), 16)
	u.watchFileEvents()
	u.clusterSvc = services.NewClusterService(core.Clusters, core.Sessions, core.Store)
	u.systemSvc = services.NewSystemService(u.host())
	u.systemSvc.SetStore(core.Store)
	u.systemSvc.SetUnlockHandler(func(pwd string) error { return core.Vault.Unlock(pwd) })
	u.setupEditDialogs(*readOnly)
	// LeanEdit runs in its own window next to the main window.
	u.editorWins = &editorWindows{store: core.Store, svc: u.editorSvc, chooseFile: u.systemSvc.ChooseFile}
	u.editorSvc.SetGUI(u.host(), u.editorWins)
	u.editorWins.watchEditorEvents(core.Bus)
	if *editFile != "" {
		if abs, err := filepath.Abs(*editFile); err == nil {
			// After the main window is up, so the editor window comes on top.
			time.AfterFunc(time.Second, func() { _ = u.editorSvc.Open(services.EditorFile{Kind: "local", Path: abs}) })
		}
	}
	// Quitting saves the configuration when "Auto save on exit" is set.
	core.Sessions.SetPrompter(u.prompter)
	core.Bus.On(events.SessionOpened, func(_ string, data any) {
		m, _ := data.(map[string]any)
		switch info := m["session"].(type) {
		case *session.SessionInfo:
			u.opened <- *info
		case session.SessionInfo:
			u.opened <- info
		}
		w.Invalidate()
	})
	core.Bus.On(events.SessionClosed, func(_ string, data any) {
		if m, ok := data.(map[string]any); ok {
			if id, ok := m["tmpID"].(string); ok {
				u.closed <- id
				w.Invalidate()
			}
		}
	})
	core.Bus.On(events.SettingsChanged, func(_ string, _ any) {
		u.settingsChanged = true
		w.Invalidate()
	})

	u.reloadSettings()
	u.loadUIState()
	u.rebuildNotebook() // the Info tab is there from the start

	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, func() {
			log.Print("self-test: quitting")
			u.quitAfter.Store(true)
			w.Invalidate()
			// A self-test must end: without a frame to quit in, report
			// where the program hangs.
			time.AfterFunc(10*time.Second, func() {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				log.Printf("self-test: no frame to quit in after 10s; goroutines:\n%s", buf[:n])
				os.Exit(3)
			})
		})
	}

	// File drops: handled in the event loop on Windows (see drop_windows.go).

	// Local shell autostart and connections marked "launch at startup".
	if cfg.LocalShell.Autostart {
		if _, err := core.Sessions.OpenLocalShell("tab"); err != nil {
			log.Print(err)
		}
	}
	for _, conn := range cfg.Connections {
		if conn.LaunchAtStartup {
			if _, err := core.Sessions.Open(session.OpenRequest{ConnIDs: []string{conn.ID}, Where: "tab"}); err != nil {
				log.Printf("%s: %v", conn.Name, err)
			}
		}
	}

	if *startShell {
		if _, err := core.Sessions.OpenLocalShell("tab"); err != nil {
			log.Print(err)
		}
	}
	for _, entry := range strings.Split(*startUUID, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		id, cluster := parts[0], ""
		if len(parts) == 2 {
			cluster = parts[1]
		}
		conn, ok := core.FindConnection(id)
		if !ok {
			log.Printf("no saved connection %q", id)
			continue
		}
		if _, err := core.Sessions.Open(session.OpenRequest{ConnIDs: []string{conn.ID}, Where: "tab", Cluster: cluster}); err != nil {
			log.Printf("%s: %v", entry, err)
		}
	}

	go func() {
		err := u.run()
		u.closeAll()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func (u *ui) closeAll() {
	u.saveUIState()
	cfg := u.core.Store.Config()
	if cfg.General.SaveOnExit {
		if err := u.configSvc.Save(); err != nil {
			log.Print("save on exit: ", err)
		}
	}
	for _, t := range u.tabs {
		t.sess.Close()
	}
	u.core.Close()
}

// exitNow ends the application without asking (the user already confirmed
// in a quit dialog, or a self-test is done).
func (u *ui) exitNow() {
	u.closeAll()
	os.Exit(0)
}

func (u *ui) host() services.Host { return u }

func (u *ui) run() error {
	var ops op.Ops
	for {
		switch e := u.w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case *app.ClosingEvent:
			// The window stays open; the quit chain decides.
			e.Abort()
			u.requestQuit()
		case app.FrameEvent:
			if u.stats {
				u.countFrame()
			}
			gtx := app.NewContext(&ops, e)
			u.layout(gtx)
			u.frameSize = e.Size
			e.Frame(gtx.Ops)
		default:
			if ev, ok := e.(app.ViewEvent); ok {
				u.maybeEnableDrop(ev)
			}
		}
	}
}

// ---- services.Host ---------------------------------------------------------

// Quit ends the application: sessions are closed and the process exits.
func (u *ui) Quit() { u.exitNow() }

// ChooseFile shows a native file chooser. The native GUI has none yet; the
// services report the error to the user.
// ChooseFile shows the system's file dialog (blocking: callers run it
// outside the GUI's event handling).
func (u *ui) ChooseFile(d services.FileDialog) (string, error) {
	return filedialog.Choose(filedialog.Options{Title: d.Title, Directory: d.Directory, Filename: d.Filename, Save: d.Save, Folder: d.Folder})
}

var _ services.Host = (*ui)(nil)

// countFrame logs the frames per second (-stats).
func (u *ui) countFrame() {
	now := time.Now()
	if u.statsStart.IsZero() {
		u.statsStart = now
	}
	u.statsFrames++
	if d := now.Sub(u.statsStart); d >= time.Second {
		log.Printf("stats: %.1f frames/s", float64(u.statsFrames)/d.Seconds())
		u.statsStart, u.statsFrames = now, 0
	}
}
