package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// ---- tree data --------------------------------------------------------------

// rebuildTree reloads the connection tree from the config.
func (u *ui) rebuildTree() {
	root := u.configSvc.GetTree()
	u.rawTree = &root
	u.byID = map[string]*config.TreeNode{}
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		u.byID[n.ID] = n
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(u.rawTree)
	sel := map[string]bool{}
	for _, id := range u.selected {
		if _, ok := u.byID[id]; ok {
			sel[id] = true
		}
	}
	u.tree.Selected = sel
	u.tree.Root = gtkTreeNode(u.rawTree)
}

func gtkTreeNode(n *config.TreeNode) *gtk.TreeNode {
	out := &gtk.TreeNode{
		ID:        n.ID,
		Name:      n.Name,
		Group:     n.IsGroup,
		Method:    n.Method,
		Protected: n.Protected,
		Favourite: n.Favourite,
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, gtkTreeNode(c))
	}
	return out
}

// findNode looks a node up by id.
func (u *ui) findNode(id string) *config.TreeNode { return u.byID[id] }

// selectedNodes returns the selected nodes that still exist.
func (u *ui) selectedNodes() []*config.TreeNode {
	var out []*config.TreeNode
	for _, id := range u.selected {
		if n := u.byID[id]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

func (u *ui) isRootSelected() bool {
	return len(u.selected) == 1 && u.selected[0] == ""
}

func (u *ui) singleSelected() *config.TreeNode {
	if len(u.selected) != 1 {
		return nil
	}
	return u.byID[u.selected[0]]
}

func (u *ui) canAdd() bool {
	cfg := activeLeftTabOf(u)
	if cfg != "connections" {
		return false
	}
	n := u.singleSelected()
	if n == nil {
		return false
	}
	if n.ID != "" && !n.IsGroup {
		return false
	}
	return !n.Protected
}

func (u *ui) canEdit() bool {
	if activeLeftTabOf(u) != "connections" || len(u.selected) < 1 || u.isRootSelected() {
		return false
	}
	n := u.singleSelected()
	return n != nil && !n.IsGroup
}

func (u *ui) canRename() bool {
	if activeLeftTabOf(u) != "connections" || len(u.selected) != 1 || u.isRootSelected() {
		return false
	}
	n := u.singleSelected()
	return n != nil && !n.Protected
}

func (u *ui) canDelete() bool {
	if activeLeftTabOf(u) != "connections" || len(u.selected) < 1 || u.isRootSelected() {
		return false
	}
	for _, n := range u.selectedNodes() {
		if n.Protected {
			return false
		}
	}
	return true
}

func (u *ui) canConnect() bool {
	switch activeLeftTabOf(u) {
	case "connections", "favourites", "history":
		return len(u.selected) > 0
	case "clusters":
		return true
	}
	return false
}

// activeLeftTabOf returns the left tab, tolerating a nil config.
func activeLeftTabOf(u *ui) string {
	if u.activeLeftTab == "" {
		return "connections"
	}
	return u.activeLeftTab
}

// collectConnIDs flattens groups to their connection ids (the order is the
// tree's, depth first).
func collectConnIDs(n *config.TreeNode) []string {
	var out []string
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		if !n.IsGroup && n.ID != "" {
			out = append(out, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}

// connectSelected opens the connections of the selection.
func (u *ui) connectSelected() {
	tab := activeLeftTabOf(u)
	switch tab {
	case "connections":
		var ids []string
		for _, n := range u.selectedNodes() {
			if n.IsGroup {
				ids = append(ids, collectConnIDs(n)...)
			} else {
				ids = append(ids, n.ID)
			}
		}
		if len(ids) > 0 {
			u.openConnections(ids, "", 1)
		}
	case "favourites":
		if len(u.selected) > 0 {
			u.openConnections(u.selected, "", 1)
		}
	case "history":
		if len(u.selected) > 0 {
			u.openConnections(u.selected, "", 1)
		}
	}
}

// openConnections opens connections in tabs (instances times each).
func (u *ui) openConnections(ids []string, cluster string, instances int) {
	if instances < 1 {
		instances = 1
	}
	if _, err := u.sessionSvc.Open(services.OpenRequest{ConnIDs: ids, Where: "tab", Cluster: cluster, Instances: instances}); err != nil {
		u.dropMsg = err.Error()
	}
}

// toggleFavourite toggles the favourite of the single selected connection.
func (u *ui) toggleFavourite() {
	n := u.singleSelected()
	if n == nil || n.IsGroup {
		return
	}
	if err := u.configSvc.SetFavourite(n.ID, !n.Favourite); err != nil {
		u.dropMsg = err.Error()
	}
}

// startRenameSelected opens the rename dialog for the single selected node.
func (u *ui) startRenameSelected() {
	if !u.canRename() {
		return
	}
	n := u.singleSelected()
	u.openAddRename(true, n.IsGroup, n.ParentID, n.ID, n.Name, n.Title)
}

// startDeleteSelected opens the delete confirmation for the selection.
func (u *ui) startDeleteSelected() {
	if !u.canDelete() {
		return
	}
	u.startDelete(append([]string{}, u.selected...))
}

// expandAll/collapseAll expand or collapse the whole tree.
func (u *ui) expandAll() {
	if u.tree.Expanded == nil {
		u.tree.Expanded = map[string]bool{}
	}
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		if n.IsGroup {
			u.tree.Expanded[n.ID] = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(u.rawTree)
	u.saveExpandedSoon()
}

func (u *ui) collapseAll() {
	// The root stays open (TreeView.svelte's collapseAll).
	u.tree.Expanded = map[string]bool{"": true}
	u.saveExpandedSoon()
}

// saveExpandedSoon persists the expanded groups to the UI state.
func (u *ui) saveExpandedSoon() {
	_ = u // saved by saveUIState; expanded groups are part of it
	u.saveUIState()
}

// ---- favourites and history --------------------------------------------------

// favouriteIDs returns the ids of all favourite connections, depth first.
func (u *ui) favouriteIDs() []string {
	var out []string
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		if !n.IsGroup && n.Favourite {
			out = append(out, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(u.rawTree)
	return out
}

// historyEntries returns the connection history (newest first).
func (u *ui) historyEntries() []HistoryRow {
	hist := u.sessionSvc.GetHistory()
	out := make([]HistoryRow, 0, len(hist))
	for _, e := range hist {
		out = append(out, HistoryRow{
			ID:        e.ID,
			Name:      e.Name,
			Method:    e.Method,
			Timestamp: formatTimestamp(e.StartedAt),
		})
	}
	return out
}

// HistoryRow is a row of the history list.
type HistoryRow struct {
	ID        string
	Name      string
	Method    string
	Timestamp string
}

// formatTimestamp renders "hh:mm:ss dd-mm-yyyy" like the web's formatDate.
func formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d %02d-%02d-%04d", t.Hour(), t.Minute(), t.Second(), t.Day(), t.Month(), t.Year())
}

// ---- the tree panel ----------------------------------------------------------

// treePanel is the command panel (CommandPanel.svelte).
func (u *ui) treePanel(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.panelToolbar(gtx) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.leftNotebook(gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !u.searchVisible {
				return layout.Dimensions{}
			}
			return u.searchArea(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.searchStartRow(gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.clustersRow(gtx) }),
	)
}

// The main colors of the web GUI's toolbar icons (a yellow folder, a red
// connection plug).
var toolbarColors = map[rune]color.NRGBA{
	fonts.IconFolderAdd: {R: 0xe0, G: 0xb0, B: 0x30, A: 0xff},
	fonts.IconPlug:      {R: 0xd9, G: 0x53, B: 0x4f, A: 0xff},
}

// panelToolbar is the toolbar with the tree operations.
func (u *ui) panelToolbar(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	const pad = 2
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 4, Right: 4, Top: pad, Bottom: pad}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			btn := func(icon rune, tooltip string, click *widget.Clickable, disabled bool) layout.FlexChild {
				return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := gtk.Button{Variant: gtk.CP, Icon: icon, IconColor: toolbarColors[icon], Disabled: disabled}
					return b.Layout(gtx, u.th, click)
				})
			}
			compact := u.cfg != nil && u.cfg.General.Layout == "compact"
			kids := []layout.FlexChild{}
			kids = append(kids,
				btn(fonts.IconFolderAdd, "", &u.buttons.addGroup, !u.canAdd()),
				btn(fonts.IconPlug, "", &u.buttons.addConn, !u.canAdd()),
				btn(fonts.IconEdit, "", &u.buttons.edit, !u.canEdit()),
				btn(fonts.IconPencil, "", &u.buttons.rename, !u.canRename()),
				btn(fonts.IconTrash, "", &u.buttons.delete, !u.canDelete()),
			)
			if compact {
				kids = append(kids, btn(fonts.IconClose, "", &u.buttons.closeWin, false))
			}
			spaced := []layout.FlexChild{}
			for i, k := range kids {
				if i > 0 {
					spaced = append(spaced, layout.Rigid(layout.Spacer{Width: 2}.Layout))
				}
				spaced = append(spaced, k)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, spaced...)
		})
	})
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, p.HeaderBg)
	call.Add(gtx.Ops)
	pant := p.BorderColor
	gtk.FillRect(gtx, image.Rectangle{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}, pant)
	return d
}

// leftNotebook is the tabbed tree widget (nbTree).
func (u *ui) leftNotebook(gtx layout.Context) layout.Dimensions {
	u.syncLeftTabs()
	page := func(gtx layout.Context) layout.Dimensions {
		switch u.activeLeftTab {
		case "favourites":
			return u.favouritesPage(gtx)
		case "history":
			return u.historyPage(gtx)
		case "clusters":
			return u.clustersPage(gtx)
		default:
			return u.connectionsPage(gtx)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.leftNB.Layout(gtx, u.th) }),
		layout.Flexed(1, page),
	)
}

// syncLeftTabs keeps the left notebook's tab list in sync with the look.
func (u *ui) syncLeftTabs() {
	isCompact := u.cfg != nil && u.cfg.General.Layout == "compact"
	showTitles := u.cfg == nil || u.cfg.Appearance.ShowTreeTitles
	label := func(id, plain string) string {
		if isCompact || (!showTitles && u.activeLeftTab != id) {
			return ""
		}
		return " " + plain
	}
	// The tabs keep their identity across frames: their click and hover
	// state lives in them.
	if u.leftTabs == nil {
		u.leftTabs = map[string]*gtk.Tab{}
	}
	mk := func(id, title string, icon rune, _ string) *gtk.Tab {
		t := u.leftTabs[id]
		if t == nil {
			t = &gtk.Tab{Closable: false}
			u.leftTabs[id] = t
		}
		t.Title, t.Icon = label(id, title), icon
		t.IconColor = leftTabColors[id]
		return t
	}
	var tabs []*gtk.Tab
	tabs = append(tabs, mk("connections", "Connections", fonts.IconList, "Connection Tree"))
	tabs = append(tabs, mk("favourites", "Favourites", fonts.IconStar, "Favourites"))
	tabs = append(tabs, mk("history", "History", fonts.IconHistory, "Connection History"))
	if u.clustersVisible {
		tabs = append(tabs, mk("clusters", "Clusters", fonts.IconSitemap, "Clusters"))
	}
	u.leftNB.Tabs = tabs
	// Keep the active tab valid.
	switch u.activeLeftTab {
	case "":
		u.activeLeftTab = "connections"
	case "clusters":
		if !u.clustersVisible {
			u.activeLeftTab = "connections"
		}
	case "favourites", "history", "connections":
	default:
		u.activeLeftTab = "connections"
	}
	idx := 0
	for i, id := range []string{"connections", "favourites", "history", "clusters"} {
		if id == u.activeLeftTab {
			idx = i
			break
		}
	}
	if u.activeLeftTab == "clusters" && !u.clustersVisible {
		idx = 0
	}
	// Tabs order matches the insertion order above.
	u.leftNB.Selected = idx
}

// The colors of the web GUI's tab icons (treelist, favourite, history).
var leftTabColors = map[string]color.NRGBA{
	"connections": iconGreen,
	"favourites":  {R: 0xf5, G: 0xa6, B: 0x23, A: 0xff},
	"history":     {R: 0x4a, G: 0x90, B: 0xe2, A: 0xff},
	"clusters":    iconGray,
}

// handleLeftNB processes the left notebook's events.
func (u *ui) handleLeftNB() {
	for _, ev := range u.leftNB.Events() {
		if ev.Kind != gtk.TabSelected {
			continue
		}
		order := []string{"connections", "favourites", "history"}
		if u.clustersVisible {
			order = append(order, "clusters")
		}
		if ev.Tab >= 0 && ev.Tab < len(order) {
			u.activeLeftTab = order[ev.Tab]
			u.saveUIState()
		}
	}
}

// ---- connections page ---------------------------------------------------------

// connectionsPage shows the tree (ConnectionTree.svelte).
func (u *ui) connectionsPage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)
	u.tree.ProtectedColor = u.protectedColor()
	return u.tree.Layout(gtx, u.th)
}

// protectedColor parses appearance.protected_color.
func (u *ui) protectedColor() color.NRGBA {
	if u.cfg != nil {
		if c := parseHexColor(u.cfg.Appearance.ProtectedColor, color.NRGBA{}); c.A != 0 {
			return c
		}
	}
	return color.NRGBA{R: 0xff, G: 0xb0, B: 0x22, A: 0xff}
}

// handleTreeEvents processes the tree's events (selection, open, context, drop).
func (u *ui) handleTreeEvents(gtx layout.Context) {
	for _, ev := range u.tree.Events() {
		switch ev.Kind {
		case gtk.TreeSelect:
			u.setSelectedFromTree()
		case gtk.TreeActivate:
			n := u.byID[ev.ID]
			if n == nil {
				continue
			}
			if n.IsGroup {
				if u.tree.Expanded == nil {
					u.tree.Expanded = map[string]bool{}
				}
				u.tree.Expanded[n.ID] = !u.tree.Expanded[n.ID]
				u.saveUIState()
				continue
			}
			u.openConnections([]string{n.ID}, "", 1)
		case gtk.TreeMiddleClick:
			n := u.byID[ev.ID]
			if n != nil && !n.IsGroup {
				u.openConnections([]string{n.ID}, "", 1)
			}
		case gtk.TreeContextClick:
			u.openTreeMenu(gtx, ev.ID, ev.Pos)
		case gtk.TreeExpand:
			u.saveUIState()
		case gtk.TreeDrop:
			var ids []string
			for _, id := range ev.IDs {
				if n := u.byID[id]; n != nil && !n.Protected {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 {
				_ = u.configSvc.MoveNodes(ids, ev.ID) // ev.ID == "" = the root
			}
		}
	}
}

// setSelectedFromTree mirrors the tree's selection into u.selected.
func (u *ui) setSelectedFromTree() {
	var ids []string
	for id := range u.tree.Selected {
		if _, ok := u.byID[id]; ok {
			ids = append(ids, id)
		}
	}
	// Stable order: keep the tree's selection order when possible.
	u.selected = orderIDs(u.rawTree, ids)
}

// orderIDs orders ids as they appear in the tree (depth first).
func orderIDs(root *config.TreeNode, ids []string) []string {
	have := map[string]bool{}
	for _, id := range ids {
		have[id] = true
	}
	var out []string
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		if have[n.ID] {
			out = append(out, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	if root != nil {
		walk(root)
	}
	return out
}

// ---- favourites page ---------------------------------------------------------

// favouritesPage lists the favourite connections (the Favourites tab).
func (u *ui) favouritesPage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)
	favs := u.favouriteIDs()
	for len(u.favRows) < len(favs) {
		u.favRows = append(u.favRows, widget.Clickable{})
		u.favStars = append(u.favStars, widget.Clickable{})
	}
	if len(favs) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return gtk.Text{S: "No favourites added yet", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
		})
	}
	list := layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(favs), func(gtx layout.Context, i int) layout.Dimensions {
		return u.favRow(gtx, favs[i], i)
	})
}

// favRow is one favourite row.
func (u *ui) favRow(gtx layout.Context, id string, i int) layout.Dimensions {
	p := u.th.Pal
	n := u.byID[id]
	if n == nil {
		return layout.Dimensions{}
	}
	selected := containsID(u.selected, id)
	bg := p.BaseColor
	fg := p.TextColor
	if selected {
		bg, fg = p.SelectedBg, p.SelectedFg
	} else if u.favRows[i].Hovered() {
		bg = p.RowHover
	}
	starClicked := u.favStars[i].Clicked(gtx)
	var rowEv widget.Click
	rowClicked := false
	if c, ok := u.favRows[i].Update(gtx); ok {
		rowEv = c
		rowClicked = true
	}
	if starClicked {
		_ = u.configSvc.SetFavourite(id, false)
	} else if rowClicked {
		if rowEv.NumClicks == 2 {
			u.openConnections([]string{id}, "", 1)
		} else {
			u.selected = []string{id}
			u.tree.Selected = map[string]bool{id: true}
		}
	}
	return u.favRows[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 6, Right: 6, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.th.IconBox(gtx, methodIcon(n.Method), 16, fg)
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: n.Name, Color: fg, MaxLines: 1}.Layout(gtx, u.th)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.favStars[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							c := color.NRGBA{R: 0xf5, G: 0xa6, B: 0x23, A: 0xff}
							if selected {
								c = p.SelectedFg
							}
							return u.th.IconBox(gtx, fonts.IconStar, 16, c)
						})
					}),
				)
			})
		})
		gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, bg)
		call.Add(gtx.Ops)
		return d
	})
}

// methodIcon maps a connection method to an icon.
func methodIcon(method string) rune {
	switch strings.ToLower(method) {
	case "ssh", "":
		return fonts.IconServer
	case "telnet":
		return fonts.IconPlug
	case "sftp", "local":
		return fonts.IconGlobe
	}
	return fonts.IconServer
}

func selectColor(selected bool, on, off uint32) color.NRGBA {
	if selected {
		return color.NRGBA{R: uint8(off >> 16), G: uint8(off >> 8), B: uint8(off), A: 0xff}
	}
	return color.NRGBA{R: uint8(on >> 16), G: uint8(on >> 8), B: uint8(on), A: 0xff}
}

// ---- history page ------------------------------------------------------------

// historyPage lists the recent connections (the History tab).
func (u *ui) historyPage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)
	hist := u.historyEntries()
	for len(u.histRows) < len(hist) {
		u.histRows = append(u.histRows, widget.Clickable{})
	}
	if len(hist) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return gtk.Text{S: "No connection history yet", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
		})
	}
	list := layout.List{Axis: layout.Vertical}
	return list.Layout(gtx, len(hist), func(gtx layout.Context, i int) layout.Dimensions {
		return u.histRow(gtx, hist[i], i)
	})
}

func (u *ui) histRow(gtx layout.Context, e HistoryRow, i int) layout.Dimensions {
	p := u.th.Pal
	selected := containsID(u.selected, e.ID)
	bg, fg := p.BaseColor, p.TextColor
	if selected {
		bg, fg = p.SelectedBg, p.SelectedFg
	} else if u.histRows[i].Hovered() {
		bg = p.RowHover
	}
	if c, ok := u.histRows[i].Update(gtx); ok {
		if c.NumClicks == 2 {
			u.openConnections([]string{e.ID}, "", 1)
		} else {
			u.selected = []string{e.ID}
			u.tree.Selected = map[string]bool{e.ID: true}
		}
	}
	return u.histRows[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 6, Right: 6, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.th.IconBox(gtx, methodIcon(e.Method), 16, fg)
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: e.Name, Color: fg, MaxLines: 1}.Layout(gtx, u.th)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: e.Timestamp, Size: gtk.Em(0.75), Color: faded(fg, 0.75), MaxLines: 1}.Layout(gtx, u.th)
					}),
				)
			})
		})
		gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, bg)
		call.Add(gtx.Ops)
		return d
	})
}

// ---- clusters page ------------------------------------------------------------

// clustersPage is the Clusters tab (the administration console is stage 6).
func (u *ui) clustersPage(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BaseColor)
	isCompact := u.cfg != nil && u.cfg.General.Layout == "compact"
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if isCompact {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				b := gtk.Button{Label: "Manage Clusters", Icon: fonts.IconSitemap, Variant: gtk.CP}
				return b.Layout(gtx, u.th, &u.manageClusters)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.HLine(gtx, p.BorderColor) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: "Clusters Administration", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
			})
		}),
	)
}

// ---- search area --------------------------------------------------------------

// searchArea is the interactive search (the Ctrl+F panel).
func (u *ui) searchArea(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 6, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			row := func(gtx layout.Context) layout.Dimensions {
				kids := []layout.FlexChild{}
				kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return gtk.Entry{Placeholder: "Search...", Width: -1}.Layout(gtx, u.th, &u.searchQuery)
				}))
				iconBtn := func(icon rune, click *widget.Clickable) layout.FlexChild {
					return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Variant: gtk.CP, Icon: icon}
						return b.Layout(gtx, u.th, click)
					})
				}
				kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				kids = append(kids,
					iconBtn(fonts.IconArrowLeft, &u.buttons.searchPrev),
					layout.Rigid(layout.Spacer{Width: 2}.Layout),
					iconBtn(fonts.IconArrowRight, &u.buttons.searchNext),
					layout.Rigid(layout.Spacer{Width: 2}.Layout),
					iconBtn(fonts.IconClose, &u.buttons.searchClose),
				)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
			}
			radios := func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return u.th.RadioGroup(gtx, &u.searchField, []gtk.RadioOption{
						{Value: "name", Label: "Name"},
						{Value: "host", Label: "IP / Host"},
						{Value: "desc", Label: "Description"},
					}, false, false)
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(row),
				layout.Rigid(radios),
			)
		})
	})
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, p.BaseColor)
	h := d.Size.Y
	gtk.FillRect(gtx, image.Rectangle{Max: image.Pt(d.Size.X, 1)}, p.BorderColor)
	gtk.FillRect(gtx, image.Rectangle{Min: image.Pt(0, h-1), Max: d.Size}, p.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// ---- the connect row and the clusters row --------------------------------------

// searchStartRow is the row with the search button, Connect and the quick connect.
func (u *ui) searchStartRow(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	const pad = 2
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 4, Right: 4, Top: pad, Bottom: pad}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			kids := []layout.FlexChild{}
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := gtk.Button{Variant: gtk.CP, Icon: fonts.IconSearch, Active: u.searchVisible}
				return b.Layout(gtx, u.th, &u.buttons.searchToggle)
			}))
			kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
			kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				b := gtk.Button{Label: "Connect", Icon: fonts.IconPlug, Disabled: !u.canConnect()}
				return b.Layout(gtx, u.th, &u.buttons.connect)
			}))
			kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				b := gtk.Button{Variant: gtk.CP, Icon: fonts.IconBolt}
				return b.Layout(gtx, u.th, &u.buttons.quickConnect)
			}))
			kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
			single := u.singleSelected()
			favOn := single != nil && !single.IsGroup && single.Favourite
			favDisabled := len(u.selected) != 1 || single == nil || single.IsGroup
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				icon := fonts.IconStarOff
				if favOn {
					icon = fonts.IconStar
				}
				b := gtk.Button{Variant: gtk.CP, Icon: icon, Disabled: favDisabled}
				return b.Layout(gtx, u.th, &u.buttons.favourite)
			}))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, p.HeaderBg)
	gtk.FillRect(gtx, image.Rectangle{Min: image.Pt(0, 0), Max: image.Pt(d.Size.X, 1)}, p.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// clustersRow is the row with the Clusters and PCC buttons (and the compact
// toolbar). The administration windows are stage 6.
func (u *ui) clustersRow(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	isCompact := u.cfg != nil && u.cfg.General.Layout == "compact"
	if !u.clustersVisible && !isCompact {
		return layout.Dimensions{}
	}
	const pad = 4
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 4, Right: 4, Top: pad, Bottom: pad}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			kids := []layout.FlexChild{}
			if u.clustersVisible {
				kids = append(kids,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Label: "Clusters", Icon: fonts.IconSitemap, Variant: gtk.Normal}
						if isCompact {
							b.Label = ""
						}
						b.MinWidth = unit.Dp(float32(gtx.Constraints.Max.X-gtx.Dp(2)) / gtx.Metric.PxPerDp) // flex: 1
						return b.Layout(gtx, u.th, &u.buttons.clusters)
					}),
					layout.Rigid(layout.Spacer{Width: 4}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Label: "PCC", Icon: fonts.IconList, Variant: gtk.Normal}
						if isCompact {
							b.Label = ""
						}
						b.MinWidth = unit.Dp(float32(gtx.Constraints.Max.X-gtx.Dp(2)) / gtx.Metric.PxPerDp) // flex: 1
						return b.Layout(gtx, u.th, &u.buttons.pcc)
					}),
				)
			}
			if isCompact {
				if len(kids) > 0 {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				}
				kids = append(kids,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Variant: gtk.CP, Icon: fonts.IconTerminal}
						return b.Layout(gtx, u.th, &u.buttons.localShell)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Variant: gtk.CP, Icon: fonts.IconCog}
						return b.Layout(gtx, u.th, &u.buttons.prefs)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := gtk.Button{Variant: gtk.CP, Icon: fonts.IconPowerOff}
						return b.Layout(gtx, u.th, &u.buttons.quitBtn)
					}),
				)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, p.HeaderBg)
	gtk.FillRect(gtx, image.Rectangle{Max: image.Pt(d.Size.X, 1)}, p.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// ---- small helpers -------------------------------------------------------------

// containsID reports whether ids contains id.
func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// faded fades a color.
func faded(c color.NRGBA, opacity float32) color.NRGBA {
	c.A = uint8(float32(c.A) * opacity)
	return c
}
