package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Suvukaraz/leanterm/internal/app"
	"github.com/Suvukaraz/leanterm/internal/cluster"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/keepass"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/Suvukaraz/leanterm/internal/session"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowBackground avoids a white flash before the frontend has painted.
func windowBackground(scheme string) application.RGBA {
	if scheme == "light" {
		return application.NewRGB(0xf0, 0xf0, 0xf0)
	}
	return application.NewRGB(0x2b, 0x2b, 0x2b)
}

type wailsEmitter struct {
	app *application.App
}

func (we *wailsEmitter) Emit(event string, data any) {
	if we.app != nil && we.app.Event != nil {
		we.app.Event.Emit(event, data)
	}
}

func handleCLICommands(flags *app.CLIFlags, sessionMgr *session.SessionManager, cfgStore *config.Store, emitter *wailsEmitter) {
	if flags == nil {
		return
	}

	// 1. Start shell
	if flags.StartShell {
		_, _ = sessionMgr.OpenLocalShell("tab")
	}

	// 2. Start UUIDs
	for _, spec := range flags.StartUUIDs {
		parts := strings.SplitN(spec, ":", 2)
		connID := parts[0]
		cluster := ""
		if len(parts) == 2 {
			cluster = parts[1]
		}
		_, _ = sessionMgr.Open(session.OpenRequest{
			ConnIDs: []string{connID},
			Where:   "tab",
			Cluster: cluster,
		})
	}

	// 3. Quick connect
	if flags.QuickConn {
		emitter.Emit("ui:open:quick-connect", nil)
	}

	// 4. Edit UUID
	if flags.EditUUID != "" {
		emitter.Emit("ui:open:edit", flags.EditUUID)
	}

	// 5. Preferences
	if flags.Preferences {
		emitter.Emit("ui:open:preferences", nil)
	}
}

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/linux/icons/leanterm-256.png
var appIcon []byte

func main() {
	// GTK 4 uses Vulkan by default. On older GPUs (e.g. Intel Ivy Bridge, Mesa "hasvk")
	// that path is incomplete and makes the whole UI sluggish; OpenGL is fast everywhere.
	// Explicitly set GSK_RENDERER / GDK_DISABLE values are respected.
	if runtime.GOOS == "linux" {
		if os.Getenv("GSK_RENDERER") == "" {
			_ = os.Setenv("GSK_RENDERER", "ngl")
		}
		if os.Getenv("GDK_DISABLE") == "" {
			_ = os.Setenv("GDK_DISABLE", "vulkan")
		}
	}

	flags := app.ParseCLI(os.Args[1:])
	if flags.Help {
		app.PrintUsage()
		return
	}

	cfgStore, err := config.NewStore(config.StoreOptions{
		ConfigDir: flags.ConfigDir,
		NoBackup:  flags.NoBackup,
		ReadOnly:  flags.ReadOnly,
	})
	if err != nil {
		log.Fatalf("failed to initialize config store: %v", err)
	}
	if err := cfgStore.Load(); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Handle CLI dump and list commands
	if flags.DumpUUID != "" {
		if err := app.DumpUUID(cfgStore.Config(), flags.DumpUUID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if flags.ListUUIDs {
		app.ListUUIDs(cfgStore.Config())
		return
	}

	vault := secrets.NewVault(secrets.VaultOptions{
		ConfigDir:      filepath.Dir(cfgStore.ConfigPath()),
		ProtectWithPwd: cfgStore.Config().General.ProtectWithPassword,
		MasterPassword: flags.Password,
		ReadOnly:       flags.ReadOnly,
		NoBackup:       flags.NoBackup,
	})
	_ = vault.Load()

	treeMgr := config.NewTreeManager(cfgStore, vault)

	emitter := &wailsEmitter{}
	clusterMgr := cluster.NewManager(cfgStore, emitter)
	sessionMgr := session.NewSessionManager(cfgStore, vault, emitter)
	sessionMgr.SetClusterManager(clusterMgr)

	kpPass := ""
	if vault != nil {
		kpPass = vault.GetGlobal("keepass_master_password")
	}
	kpClient := keepass.NewClient(keepass.Options{
		Config:         cfgStore.Config().KeePass,
		MasterPassword: kpPass,
	})
	sessionMgr.SetKeePassClient(kpClient)

	configSvc := services.NewConfigService(cfgStore, vault, treeMgr)
	sessionSvc := services.NewSessionService(cfgStore, sessionMgr)
	sftpSvc := services.NewSFTPService(nil, sessionMgr, emitter)
	sftpSvc.SetStore(cfgStore)
	localFSSvc := services.NewLocalFSService(sessionMgr, emitter)
	localFSSvc.SetStore(cfgStore)
	dialogBroker := services.NewDialogBroker(emitter)
	dialogSvc := services.NewDialogService(dialogBroker)
	sessionMgr.SetPrompter(dialogBroker)
	systemSvc := services.NewSystemService(nil)
	systemSvc.SetStore(cfgStore)
	clusterSvc := services.NewClusterService(clusterMgr, sessionMgr, cfgStore)

	var singleInstance *application.SingleInstanceOptions
	if !cfgStore.Config().General.AllowMultipleInstances {
		singleInstance = &application.SingleInstanceOptions{
			UniqueID: "org.leanterm.LeanTerm.Instance",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				secondFlags := app.ParseCLI(data.Args)
				handleCLICommands(secondFlags, sessionMgr, cfgStore, emitter)
			},
		}
	}

	wailsApp := application.New(application.Options{
		Name:        "LeanTerm",
		Description: "LeanTerm Connection Manager",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(configSvc),
			application.NewService(sessionSvc),
			application.NewService(sftpSvc),
			application.NewService(localFSSvc),
			application.NewService(dialogSvc),
			application.NewService(systemSvc),
			application.NewService(clusterSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: singleInstance,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Matches the installed org.leanterm.LeanTerm.desktop, so desktops (KDE, GNOME)
		// show its icon in the title bar and the task bar.
		Linux: application.LinuxOptions{
			ApplicationID: "org.leanterm.LeanTerm",
		},
	})
	emitter.app = wailsApp
	systemSvc.SetApp(wailsApp)

	title := "LeanTerm"
	if flags.ReadOnly {
		title = "LeanTerm - READ ONLY MODE"
	}

	uiState := cfgStore.LoadUIState()
	geom := uiState.MainWindow
	winOpts := application.WebviewWindowOptions{
		Title:            title,
		Width:            geom.Width,
		Height:           geom.Height,
		MinWidth:         640,
		MinHeight:        420,
		URL:              "/",
		BackgroundColour: windowBackground(cfgStore.Config().General.ColorScheme),
		// Files dragged from the desktop onto an element with data-file-drop-target
		// (the SFTP browser) arrive as real paths via "files:dropped".
		EnableFileDrop: true,
	}
	if geom.Width < 640 || geom.Height < 420 {
		winOpts.Width, winOpts.Height = 1200, 760
	}
	if cfgStore.Config().General.RememberMainSize && (geom.X != 0 || geom.Y != 0) {
		winOpts.InitialPosition = application.WindowXY
		winOpts.X, winOpts.Y = geom.X, geom.Y
	}
	if cfgStore.Config().General.StartMainMaximized || (cfgStore.Config().General.RememberMainSize && geom.Maximized) {
		winOpts.StartState = application.WindowStateMaximised
	}

	mainWindow := wailsApp.Window.NewWithOptions(winOpts)

	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		ctx := e.Context()
		payload := map[string]any{"files": ctx.DroppedFiles()}
		if d := ctx.DropTargetDetails(); d != nil {
			payload["attributes"] = d.Attributes
			payload["id"] = d.ElementID
		}
		emitter.Emit("files:dropped", payload)
	})

	// Remember the window geometry for the next start (merged with the panel layout
	// the frontend stores in the same file).
	// Closing the window (X) asks the frontend first, which confirms open
	// connections/transfers and then calls SystemService.Quit.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		st := cfgStore.LoadUIState()
		st.MainWindow.Maximized = mainWindow.IsMaximised()
		if !st.MainWindow.Maximized {
			st.MainWindow.Width, st.MainWindow.Height = mainWindow.Size()
			st.MainWindow.X, st.MainWindow.Y = mainWindow.Position()
		}
		_ = cfgStore.SaveUIState(st)
		if !systemSvc.IsQuitting() {
			e.Cancel()
			emitter.Emit("app:close-requested", nil)
		}
	})

	// Autostart connections and CLI flags
	go func() {
		time.Sleep(500 * time.Millisecond)

		// 1. Launch at startup
		for _, conn := range cfgStore.Config().Connections {
			if conn.LaunchAtStartup {
				_, _ = sessionMgr.Open(session.OpenRequest{
					ConnIDs: []string{conn.ID},
					Where:   "tab",
				})
			}
		}

		// 2. CLI commands
		handleCLICommands(flags, sessionMgr, cfgStore, emitter)

		// 3. Local shell autostart
		if cfgStore.Config().LocalShell.Autostart && !flags.StartShell {
			_, _ = sessionMgr.OpenLocalShell("tab")
		}
	}()

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
