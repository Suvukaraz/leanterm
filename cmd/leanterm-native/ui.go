package main

import (
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/appcore"
	"github.com/Suvukaraz/leanterm/internal/gui/dialogs"
	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/termview"
	"github.com/Suvukaraz/leanterm/internal/services"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// tab is a terminal tab, bound to a session of the backend.
type tab struct {
	sess   *termview.Session
	view   *termview.View
	engine string
	kind   string
	nb     *gtk.Tab
}

// splitState is an active split of two terminal tabs (splits.ts). The size
// of the first pane is kept in the paned widget itself.
type splitState struct {
	// orientation: "horizontal" = side by side, "vertical" = stacked.
	orientation string
	secondary   string
	paned       gtk.Paned
}

// panelButtons are the click targets of the command panel and the button bar.
type panelButtons struct {
	// Command panel toolbar.
	addGroup, addConn, edit, rename, delete widget.Clickable
	closeWin                                widget.Clickable
	// Bottom rows.
	connect, quickConnect, favourite widget.Clickable
	searchToggle                     widget.Clickable
	clusters, pcc                    widget.Clickable
	// Search area.
	searchPrev, searchNext, searchClose widget.Clickable
	infoTabBtn                          widget.Clickable
	// Main button bar.
	panelToggle, sftpToggle, clustersToggle widget.Clickable
	localShell, wol, prefs, lock, about     widget.Clickable
	quitBtn                                 widget.Clickable
}

// ui is the state of the main window.
type ui struct {
	w    *app.Window
	th   *gtk.Theme
	core *appcore.Core

	prompter   *dialogs.Prompter
	configSvc  *services.ConfigService
	sessionSvc *services.SessionService
	editorSvc  *services.EditorService
	systemSvc  *services.SystemService
	sftpSvc    *services.SFTPService

	// The main notebook: the Info tab first, then the terminal tabs.
	nb      gtk.Notebook
	tabs    []*tab
	splits  map[string]*splitState
	newData map[string]bool
	focus   bool // focus the terminal view on tab selection
	infoTab *gtk.Tab

	// Sessions opened and closed by the backend (any goroutine).
	opened chan session.SessionInfo
	closed chan string
	engine string

	// Drops from the desktop (files). The editor does not exist yet; the
	// status line reports them.
	drops    chan dnd.Drop
	dropMsg  string
	readOnly bool

	// The configured look, refreshed on SettingsChanged.
	cfg    *config.Config
	scheme string

	frameSize image.Point

	settingsChanged bool
	quitAfter       bool

	// Command panel (the connections sidebar).
	panel           gtk.Paned // zero value: set to NewPaned() in main
	panelVisible    bool
	panelWidth      int
	sftpVisible     bool
	clustersVisible bool
	buttons         panelButtons
	activeLeftTab   string

	// The left-hand notebook with the Connections/Favourites/History/Clusters tabs.
	leftNB   gtk.Notebook
	leftTabs []*gtk.Tab
	clusters widget.Clickable

	// Search (Ctrl+F).
	searchVisible bool
	searchQuery   widget.Editor
	searchField   widget.Enum
	searchLast    string

	// The tree and its helpers.
	tree     gtk.Tree
	selected []string
	rawTree  *config.TreeNode
	byID     map[string]*config.TreeNode

	// Favourites/history/clusters flat lists: clicked rows, star buttons.
	favRows  []widget.Clickable
	favStars []widget.Clickable
	histRows []widget.Clickable

	// Info tab.
	notes        widget.Editor
	notesNode    string
	notesLast    string
	notesDirty   bool
	notesChanged time.Time
	statsReset   widget.Clickable
	shotsOpen    widget.Clickable

	// Dialogs (see dialogs.go).
	quit     quitDialog
	addRen   addRenameDialog
	del      deleteDialog
	about    aboutDialog
	keys     keybindingsDialog
	lock     lockOverlay
	tabMenu  gtk.Menu
	treeMenu gtk.Menu
	// The tab/tree the context menu was opened for, and the internal
	// clipboard for the tree's copy/cut/paste (like the web GUI's).
	tabMenuTarget  string
	treeMenuTarget string
	clipboardIDs   []string
	clipboardCut   bool
	manageClusters widget.Clickable

	// The window-wide pointer tag (the context menus open at the pointer).
	rootTag int
	lastPtr image.Point
}
