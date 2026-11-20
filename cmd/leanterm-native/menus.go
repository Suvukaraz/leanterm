package main

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- window-wide pointer position ---------------------------------------------

// trackPointer reads where the pointer is, in window coordinates (the
// context menus open there). It runs first in a frame, before the handlers
// that open menus; globalPointer registers its area.
func (u *ui) trackPointer(gtx layout.Context) {
	for {
		e, ok := gtx.Event(pointer.Filter{Target: &u.rootTag, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag})
		if !ok {
			break
		}
		if pe, ok := e.(pointer.Event); ok {
			u.lastPtr = pe.Position.Round()
		}
	}
}

// globalPointer registers a window-wide area on top of everything that
// passes all events on: the other areas are siblings, not its parents, so
// only such an area sees the pointer everywhere.
func (u *ui) globalPointer(gtx layout.Context) layout.Dimensions {
	pass := pointer.PassOp{}.Push(gtx.Ops)
	rs := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &u.rootTag)
	rs.Pop()
	pass.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// ---- buttons -------------------------------------------------------------------

// handleClicks processes the clicks of the simple buttons (their own Layout
// finds nothing left to process afterwards).
func (u *ui) handleClicks(gtx layout.Context) {
	b := &u.buttons
	clicked := func(c *widget.Clickable) bool { return c.Clicked(gtx) }

	if clicked(&b.panelToggle) {
		u.panelVisible = !u.panelVisible
		u.noteUIStateSoon()
	}
	if clicked(&b.sftpToggle) {
		u.sftpVisible = !u.sftpVisible
		// The tabs follow (TerminalTab.svelte's $sftpVisible effect).
		for _, t := range u.tabs {
			if t.kind == "local" || t.hasSFTP {
				t.sftpOpen = u.sftpVisible
			}
		}
		u.noteUIStateSoon()
	}
	if clicked(&b.clustersToggle) {
		u.clustersVisible = !u.clustersVisible
		u.noteUIStateSoon()
	}
	if clicked(&b.localShell) {
		if _, err := u.sessionSvc.OpenLocalShell("tab"); err != nil {
			u.dropMsg = err.Error()
		}
	}
	if clicked(&b.wol) {
		u.dropMsg = "The Wake On LAN window is not implemented yet"
	}
	if clicked(&b.prefs) {
		u.dropMsg = "The settings dialog is not implemented yet"
	}
	if clicked(&b.lock) {
		u.toggleLock()
	}
	if clicked(&b.about) {
		u.about.open = true
	}
	if clicked(&b.closeWin) {
		u.requestQuit()
	}
	if clicked(&b.quitBtn) {
		u.requestQuit()
	}
	if clicked(&b.searchToggle) {
		u.searchVisible = !u.searchVisible
		u.noteUIStateSoon()
	}
	if clicked(&b.searchClose) {
		u.searchVisible = false
		u.noteUIStateSoon()
	}
	if clicked(&b.searchPrev) {
		u.searchTree(true)
	}
	if clicked(&b.searchNext) {
		u.searchTree(false)
	}
	if clicked(&b.connect) {
		u.connectSelected()
	}
	if clicked(&b.quickConnect) {
		u.dropMsg = "The quick connect window is not implemented yet"
	}
	if clicked(&b.favourite) {
		u.toggleFavourite()
	}
	if clicked(&b.infoTabBtn) {
		u.nb.Selected = 0
		u.focus = true
	}
	if clicked(&u.manageClusters) {
		u.dropMsg = "The cluster administration is not implemented yet"
	}
	if clicked(&u.statsReset) {
		if id := u.notesNode; id != "" {
			_ = u.sessionSvc.ResetStats(id)
		}
	}
	if clicked(&u.shotsOpen) {
		if err := u.systemSvc.OpenPath(u.systemSvc.ScreenshotsDir()); err != nil {
			u.dropMsg = err.Error()
		}
	}

	// The search field's Enter searches forward.
	for {
		ev, ok := u.searchQuery.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			u.searchTree(false)
		}
	}
}

// searchTree jumps to the next (or previous) node matching the query.
func (u *ui) searchTree(backwards bool) {
	q := strings.TrimSpace(u.searchQuery.Text())
	if q == "" {
		return
	}
	field := "name"
	switch u.searchField.Value {
	case "host":
		field = "host"
	case "desc":
		field = "desc"
	}
	match := u.configSvc.Search(q, field, u.searchLast, backwards)
	if match == "" {
		return
	}
	u.searchLast = match
	u.selected = []string{match}
	u.tree.Selected = map[string]bool{match: true}
	u.expandPathTo(match)
	u.saveUIState()
}

// expandPathTo expands the groups on the path to id so the node is visible.
func (u *ui) expandPathTo(id string) {
	var path []string
	var find func(n *config.TreeNode) bool
	find = func(n *config.TreeNode) bool {
		if n == nil {
			return false
		}
		if n.ID == id {
			return true
		}
		for _, c := range n.Children {
			if find(c) {
				path = append(path, c.ID)
				return true
			}
		}
		return false
	}
	if u.rawTree == nil {
		return
	}
	find(u.rawTree)
	if u.tree.Expanded == nil {
		u.tree.Expanded = map[string]bool{}
	}
	for _, gid := range path {
		u.tree.Expanded[gid] = true
	}
}

// ---- tab context menu -----------------------------------------------------------

func secondIsSplit(splits map[string]*splitState, id string) bool {
	for _, s := range splits {
		if s.secondary == id {
			return true
		}
	}
	return false
}

func tabIcon(t *tab) rune {
	switch t.kind {
	case "telnet":
		return fonts.IconPlug
	case "sftp":
		return fonts.IconGlobe
	default:
		return fonts.IconServer
	}
}

// openTabMenu opens the context menu of a terminal tab (TabContextMenu.svelte).
func (u *ui) openTabMenu(tmpID string, _ image.Point) {
	u.tabMenuTarget = tmpID
	_, isSplit := u.splits[tmpID]

	items := []gtk.MenuItem{}

	var gotoItems []gtk.MenuItem
	for i, t := range u.tabs {
		gotoItems = append(gotoItems, gtk.MenuItem{
			ID:    "goto:" + t.sess.TmpID,
			Label: fmt.Sprintf("%d: %s", i+1, t.sess.Title()),
			Icon:  tabIcon(t),
		})
	}
	items = append(items,
		gtk.MenuItem{ID: "goto", Label: "Goto TAB", Icon: fonts.IconList, Submenu: gotoItems},
		gtk.MenuItem{Separator: true},
	)

	if isSplit {
		items = append(items,
			gtk.MenuItem{ID: "unsplit", Label: "Unsplit"},
			gtk.MenuItem{ID: "equal-resize", Label: "Equally resize terminals"},
		)
	} else {
		var candidates []*tab
		for _, t := range u.tabs {
			if t.sess.TmpID == tmpID {
				continue
			}
			if _, ok := u.splits[t.sess.TmpID]; ok {
				continue
			}
			if secondIsSplit(u.splits, t.sess.TmpID) {
				continue
			}
			candidates = append(candidates, t)
		}
		var vert, horiz []gtk.MenuItem
		for i, t := range candidates {
			label := fmt.Sprintf("%d: %s", i+1, t.sess.Title())
			vert = append(vert, gtk.MenuItem{ID: "split-v:" + t.sess.TmpID, Label: label})
			horiz = append(horiz, gtk.MenuItem{ID: "split-h:" + t.sess.TmpID, Label: label})
		}
		items = append(items, gtk.MenuItem{
			ID:    "split",
			Label: "Split",
			Submenu: []gtk.MenuItem{
				{ID: "split-v", Label: "Vertically", Disabled: len(vert) == 0, Submenu: vert},
				{ID: "split-h", Label: "Horizontally", Disabled: len(horiz) == 0, Submenu: horiz},
			},
		})
	}

	items = append(items,
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "detach", Label: "Detach TAB to a new Window", Icon: fonts.IconExternal, Disabled: true},
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "disconnect", Label: "Disconnect session", Icon: fonts.IconStop},
		gtk.MenuItem{ID: "close", Label: "Close terminal", Icon: fonts.IconClose, Shortcut: "Ctrl+Shift+W"},
		gtk.MenuItem{ID: "close-others", Label: "Close other terminals"},
		gtk.MenuItem{ID: "close-all", Label: "Close all terminals"},
	)
	u.tabMenu.Items = items
	u.tabMenu.Open(u.lastPtr)
}

// handleTabMenu applies the item chosen in the tab menu.
func (u *ui) handleTabMenu() {
	id, ok := u.tabMenu.Activated()
	if !ok {
		return
	}
	target := u.tabMenuTarget
	switch {
	case strings.HasPrefix(id, "goto:"):
		tid := strings.TrimPrefix(id, "goto:")
		for i, t := range u.tabs {
			if t.sess.TmpID == tid {
				u.nb.Selected = i + 1
				u.focus = true
				break
			}
		}
	case id == "unsplit":
		u.unsplitTab(target)
	case id == "equal-resize":
		u.equalResize(target)
	case strings.HasPrefix(id, "split-v:"):
		u.splitTab(target, strings.TrimPrefix(id, "split-v:"), "horizontal")
	case strings.HasPrefix(id, "split-h:"):
		u.splitTab(target, strings.TrimPrefix(id, "split-h:"), "vertical")
	case id == "disconnect":
		_ = u.sessionSvc.Disconnect(target)
	case id == "close":
		u.closeTab(target)
	case id == "close-others":
		u.closeOtherTabs(target)
	case id == "close-all":
		u.closeAllTabs()
	}
}

// ---- tree context menu ---------------------------------------------------------

// openTreeMenu opens the context menu of the tree (ConnectionTree.svelte's).
// gtx is unused: the menu opens at the current pointer position.
func (u *ui) openTreeMenu(_ layout.Context, _ string, _ image.Point) {
	selNodes := u.selectedNodes()
	isSingle := len(selNodes) == 1
	isRoot := isSingle && selNodes[0].ID == ""
	isGroup := isSingle && selNodes[0].IsGroup
	isConn := isSingle && !selNodes[0].IsGroup
	anyProtected, allProtected := false, len(selNodes) > 0
	for _, n := range selNodes {
		if n.Protected {
			anyProtected = true
		} else {
			allProtected = false
		}
	}
	var target *config.TreeNode
	if len(selNodes) > 0 {
		target = selNodes[0]
	}
	targetID := ""
	targetName := ""
	if target != nil {
		targetID, targetName = target.ID, target.Name
	}
	canPaste := (isRoot || isGroup) && len(u.clipboardIDs) > 0 && !anyProtected

	protectLabel := "Protect"
	if allProtected {
		protectLabel = "Un-Protect"
	} else if anyProtected {
		protectLabel = "Toggle Protected state"
	}
	protectIcon := fonts.IconLock
	if allProtected {
		protectIcon = fonts.IconUnlock
	}

	addDisabled := (!isRoot && !isGroup) || anyProtected

	items := []gtk.MenuItem{
		{ID: "keybindings", Label: "Show Keybindings", Icon: fonts.IconQuestion},
		{Separator: true},
		{ID: "expand-all", Label: "Expand all", Icon: fonts.IconPlus},
		{ID: "collapse-all", Label: "Collapse all", Icon: fonts.IconMinus},
		{Separator: true},
		{ID: "protection", Label: protectLabel, Icon: protectIcon, Disabled: isRoot},
		{ID: "edit", Label: "Edit connection(s)", Icon: fonts.IconEdit, Disabled: isRoot},
		// The web GUI has this item as a stub too (enabled, no action).
		{ID: "copy-password", Label: "Copy Password", Icon: fonts.IconCopy, Disabled: !isConn},
		{Separator: true},
	}

	if isGroup && !isRoot {
		items = append(items, gtk.MenuItem{ID: "start-group", Label: "Start Group", Icon: fonts.IconPlay})
	}

	items = append(items,
		gtk.MenuItem{ID: "add-conn", Label: fmt.Sprintf("Add Connection under '%s'", targetName), Icon: fonts.IconPlus, Disabled: addDisabled},
		gtk.MenuItem{ID: "add-group", Label: fmt.Sprintf("Add Group under '%s'", targetName), Icon: fonts.IconFolderAdd, Disabled: addDisabled},
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "rename", Label: "Rename Group/Connection", Icon: fonts.IconPencil, Disabled: isRoot || anyProtected || !isSingle},
		gtk.MenuItem{ID: "delete", Label: "Delete...", Icon: fonts.IconTrash, Disabled: isRoot || anyProtected},
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "clone", Label: "Clone connection", Icon: fonts.IconCopy, Disabled: !isConn || anyProtected},
		gtk.MenuItem{ID: "copy", Label: "Copy node(s)", Icon: fonts.IconCopy, Disabled: isRoot},
		gtk.MenuItem{ID: "cut", Label: "Cut node(s)", Icon: fonts.IconCut, Disabled: isRoot || anyProtected},
		gtk.MenuItem{ID: "paste", Label: "Paste node(s)", Icon: fonts.IconPaste, Disabled: !canPaste},
		gtk.MenuItem{Separator: true},
		// These open dialogs that are not ported yet.
		gtk.MenuItem{ID: "edit-variables", Label: "Edit Local Variables...", Icon: fonts.IconEdit, Disabled: true},
		gtk.MenuItem{ID: "bulk-edit", Label: "Bulk Edit...", Icon: fonts.IconEdit, Disabled: true},
		gtk.MenuItem{ID: "wol", Label: "Wake On LAN...", Icon: fonts.IconBolt, Disabled: true},
	)

	// Start N instances of the node (connections) or the group's connections.
	startItems := make([]gtk.MenuItem, 0, 9)
	for i := 1; i <= 9; i++ {
		label := fmt.Sprintf("%d instance", i)
		if i > 1 {
			label += "s"
		}
		startItems = append(startItems, gtk.MenuItem{ID: fmt.Sprintf("start:%d", i), Label: label})
	}
	items = append(items, gtk.MenuItem{
		ID:       "start",
		Label:    "Start",
		Icon:     fonts.IconPlay,
		Disabled: isRoot,
		Submenu:  startItems,
	})

	u.treeMenuTarget = targetID
	u.treeMenu.Items = items
	u.treeMenu.Open(u.lastPtr)
}

// handleTreeMenu applies the item chosen in the tree menu.
func (u *ui) handleTreeMenu() {
	id, ok := u.treeMenu.Activated()
	if !ok {
		return
	}
	sel := append([]string{}, u.selected...)
	selNodes := u.selectedNodes()
	isSingle := len(selNodes) == 1
	isGroup := isSingle && selNodes[0].IsGroup
	target := u.treeMenuTarget

	switch {
	case id == "keybindings":
		u.keys.open = true
	case id == "edit":
		// Stage 6 ports the Edit Connection window.
		u.dropMsg = "The Edit Connection window is not implemented yet"
	case id == "expand-all":
		u.expandAll()
	case id == "collapse-all":
		u.collapseAll()
	case id == "protection":
		allProtected := len(selNodes) > 0
		for _, n := range selNodes {
			if !n.Protected {
				allProtected = false
			}
		}
		if err := u.configSvc.SetProtected(sel, !allProtected); err != nil {
			u.dropMsg = err.Error()
		}
		u.finishTreeChange()
	case id == "start-group":
		if target == "" && u.rawTree != nil {
			// The root: start every connection.
			u.openConnections(collectConnIDs(u.rawTree), "", 1)
			return
		}
		if n := u.findNode(target); n != nil {
			u.openConnections(collectConnIDs(n), "", 1)
		}
	case id == "add-conn":
		u.openAddRename(false, false, target, "", "", "")
	case id == "add-group":
		u.openAddRename(false, true, target, "", "", "")
	case id == "rename":
		if isSingle {
			n := selNodes[0]
			u.openAddRename(true, n.IsGroup, n.ParentID, n.ID, n.Name, n.Title)
		}
	case id == "delete":
		u.startDelete(sel)
	case id == "clone":
		if isSingle && !selNodes[0].IsGroup {
			if _, err := u.configSvc.CloneNode(sel[0]); err != nil {
				u.dropMsg = err.Error()
			}
			u.finishTreeChange()
		}
	case id == "copy":
		if err := u.configSvc.CopyNodes(sel); err != nil {
			u.dropMsg = err.Error()
			break
		}
		u.clipboardIDs, u.clipboardCut = sel, false
	case id == "cut":
		if err := u.configSvc.CutNodes(sel); err != nil {
			u.dropMsg = err.Error()
			break
		}
		u.clipboardIDs, u.clipboardCut = sel, true
	case id == "paste":
		if err := u.configSvc.PasteNodes(target); err != nil {
			u.dropMsg = err.Error()
		}
		if u.clipboardCut {
			u.clipboardIDs = nil
		}
		u.finishTreeChange()
	case strings.HasPrefix(id, "start:"):
		n := 0
		if _, err := fmt.Sscanf(id, "start:%d", &n); err == nil && n > 0 {
			ids := sel
			if isSingle && isGroup {
				ids = collectConnIDs(selNodes[0])
			} else if isSingle {
				ids = []string{sel[0]}
			}
			if len(ids) > 0 {
				u.openConnections(ids, "", n)
			}
		}
	}
}
