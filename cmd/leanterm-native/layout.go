package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// record lays w out into a macro and returns its size.
func record(gtx layout.Context, w layout.Widget) (op.CallOp, layout.Dimensions) {
	rec := op.Record(gtx.Ops)
	d := w(gtx)
	return rec.Stop(), d
}

// layout draws the whole window.
func (u *ui) layout(gtx layout.Context) layout.Dimensions {
	if u.quitAfter.Load() { // the self-test of -quit-after
		u.exitNow()
	}
	u.winWidth = gtx.Constraints.Max.X
	u.trackPointer(gtx)
	for drained := false; !drained; {
		select {
		case f := <-u.filesEvents:
			f()
		default:
			drained = true
		}
	}
	u.sessionEvents()
	u.handleTabEvents()
	u.handleLeftNB()
	u.handleTreeEvents(gtx)
	u.handleClicks(gtx)
	u.handleDrops()
	if u.settingsChanged {
		u.reloadSettings()
		u.settingsChanged = false
	}
	u.unsplitDisconnected()
	u.handleTabMenu()
	u.handleTreeMenu()
	u.handleTermMenu(gtx)

	for _, t := range u.tabs {
		title, icon, c := u.tabLook(t)
		// The status color is the title's, shown bold (Notebook.svelte); the
		// icon is the method's.
		t.nb.Title, t.nb.Icon, t.nb.Color = title, icon, c
		t.nb.IconColor = tabIconColors[icon]
	}
	active := u.activeTabID()
	delete(u.newData, active)

	cfg := u.cfg
	compact := cfg != nil && cfg.General.Layout == "compact"

	hideButtonBar := compact
	if !compact && cfg != nil && cfg.Appearance.AutoHideButtonBar && active != "info" {
		hideButtonBar = true
	}
	buttonBarTop := true
	if cfg != nil && cfg.Appearance.ButtonBarPosition == "bottom" {
		buttonBarTop = false
	}

	// The global shortcuts are processed only while the focus is not on a
	// terminal or a text input and no dialog is open (like the web GUI).
	u.globalKeys(gtx)

	return layout.Stack{Alignment: layout.NW}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			kids := []layout.FlexChild{}
			bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.buttonBar(gtx, buttonBarTop) })
			if !hideButtonBar && buttonBarTop {
				kids = append(kids, bar)
			}
			kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return u.mainBody(gtx)
			}))
			if !hideButtonBar && !buttonBarTop {
				kids = append(kids, bar)
			}
			// The drop status message.
			if u.dropMsg != "" {
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.th.Statusbar(gtx, layout.Rigid(u.th.StatusText(u.dropMsg)))
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		}),
		// The dialogs and menus on top.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			u.handleQuit(gtx)
			u.handleDelete(gtx)
			u.handleAddRename(gtx)
			u.handleAbout(gtx)
			u.handleKeybindings(gtx)
			return layout.Dimensions{}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.prompter.Layout(gtx, u.th)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.tabMenu.Layout(gtx, u.th)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.treeMenu.Layout(gtx, u.th)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.termMenu.Layout(gtx, u.th)
		}),
		layout.Expanded(u.fileOverlays),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return u.editConn.Layout(gtx, u.th) }),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			u.handleMacroConfirm(gtx)
			return layout.Dimensions{}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			u.handleLock(gtx)
			return layout.Dimensions{}
		}),
		// On top of everything, passing all events on: where the pointer is,
		// for the context menus.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.globalPointer(gtx)
		}),
	)
}

// dialogOpen reports whether any dialog is open (the global keys go to the
// dialog then, not to the window).
func (u *ui) dialogOpen() bool {
	return u.quit.open || u.del.open || u.addRen.open || u.about.open || u.keys.open || u.prompter.Active() || u.macroConfirmOpen || u.filesDialogOpen() || u.editConn.Opened()
}

// ---- button bar --------------------------------------------------------------

// buttonBar is the main button bar (MainButtonBar.svelte): normal GTK
// buttons with colored icons, 4px apart, padding 3px 6px, on a gradient
// from the window background to the header color, with a border towards
// the body.
func (u *ui) buttonBar(gtx layout.Context, top bool) layout.Dimensions {
	p := u.th.Pal
	pp := u.cfg != nil && u.cfg.General.ProtectWithPassword
	locked := u.lock.active
	type item struct {
		icon   rune
		color  color.NRGBA
		label  string
		click  *widget.Clickable
		active bool
		off    bool
	}
	lockIcon := fonts.IconUnlock
	if locked {
		lockIcon = fonts.IconLock
	}
	items := []item{
		{fonts.IconList, iconGreen, "Connections", &u.buttons.panelToggle, u.panelVisible, false},
		{fonts.IconBriefcase, iconSFTP, "File Browser", &u.buttons.sftpToggle, u.sftpVisible, false},
		{fonts.IconSitemap, iconGray, "Clusters", &u.buttons.clustersToggle, u.clustersVisible, false},
		{fonts.IconTerminal, iconGreen, "Local shell", &u.buttons.localShell, false, false},
		{fonts.IconPowerOff, iconGray, "Wake On Lan", &u.buttons.wol, false, false},
		{fonts.IconTools, iconGray, "Settings", &u.buttons.prefs, false, false},
		{lockIcon, iconLock, "", &u.buttons.lock, locked, !pp},
		{fonts.IconInfo, color.NRGBA{}, "About", &u.buttons.about, false, false},
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 6, Right: 6, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			var kids []layout.FlexChild
			for i, it := range items {
				if i > 0 {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				}
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := gtk.Button{Icon: it.icon, IconColor: it.color, Label: it.label, Active: it.active, Disabled: it.off}
					return b.Layout(gtx, u.th, it.click)
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	d.Size.X = gtx.Constraints.Max.X
	d.Size.Y += 1 // the border
	r := image.Rectangle{Max: d.Size}
	grad := paint.LinearGradientOp{Stop1: f32.Pt(0, 0), Color1: p.BgColor, Stop2: f32.Pt(0, float32(d.Size.Y)), Color2: p.HeaderBg}
	st := clip.Rect(r).Push(gtx.Ops)
	grad.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	line := image.Rectangle{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}
	if !top {
		line = image.Rectangle{Max: image.Pt(d.Size.X, 1)}
		off := op.Offset(image.Pt(0, 1)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
	} else {
		call.Add(gtx.Ops)
	}
	gtk.FillRect(gtx, line, p.BorderColor)
	return d
}

// tabIconColors are the main colors of the method icons in the tabs.
var tabIconColors = map[rune]color.NRGBA{
	fonts.IconTerminal: iconGreen,
}

// The web GUI's icons are colored SVGs; the Nerd Font icons get their main
// color.
var (
	iconGreen = color.NRGBA{R: 0x3f, G: 0xb3, B: 0x3f, A: 0xff}
	iconSFTP  = color.NRGBA{R: 0x8a, G: 0xb2, B: 0x5a, A: 0xff}
	iconGray  = color.NRGBA{R: 0xb8, G: 0xb8, B: 0xb8, A: 0xff}
	iconLock  = color.NRGBA{R: 0xc8, G: 0xa0, B: 0x60, A: 0xff}
)

// ---- body -------------------------------------------------------------------

// mainBody is the connections panel and the main panel, with the paned separator.
func (u *ui) mainBody(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	gtk.Fill(gtx, p.BgColor)

	treeLeft := true
	if u.cfg != nil && u.cfg.Appearance.TreePosition == "right" {
		treeLeft = false
	}

	if !u.panelVisible {
		return u.mainPanel(gtx)
	}
	first, second := layout.Widget(u.treePanel), layout.Widget(u.mainPanel)
	if !treeLeft {
		first, second = layout.Widget(u.mainPanel), layout.Widget(u.treePanel)
		u.panel.MinFirst, u.panel.MinSecond = 300, 150
	} else {
		u.panel.MinFirst, u.panel.MinSecond = 150, 300
	}
	return u.panel.Layout(gtx, u.th, first, second)
}

// mainPanel is the terminal notebook next to the connection panel.
func (u *ui) mainPanel(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.notebook(gtx)
		}),
	)
}

// notebook is the main notebook with the Info tab and the terminal tabs.
func (u *ui) notebook(gtx layout.Context) layout.Dimensions {
	prev := u.nb.Selected
	page := func(gtx layout.Context) layout.Dimensions {
		if u.nb.Selected == 0 {
			return u.infoPage(gtx)
		}
		i := u.nb.Selected - 1
		if i >= len(u.tabs) {
			return layout.Dimensions{}
		}
		return u.terminalPage(gtx, u.tabs[i], prev != u.nb.Selected)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.nb.Layout(gtx, u.th)
		}),
		layout.Flexed(1, page),
	)
}

// terminalPage is a terminal tab (or a split of two) plus its status bar.
func (u *ui) terminalPage(gtx layout.Context, t *tab, tabChanged bool) layout.Dimensions {
	showStatusBar := u.cfg == nil || u.cfg.Terminal.ShowStatusBar

	if p, sec, ok := u.splitSessions(t.sess.TmpID); ok {
		return u.splitPage(gtx, p, sec, showStatusBar, tabChanged)
	}
	// The file browser takes the full height beside the terminal and its
	// status bar (TerminalTab.svelte).
	return u.withFiles(gtx, t, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if (u.focus || tabChanged) && !u.prompter.Active() && !u.lock.active {
					t.view.Focus(gtx)
					u.focus = false
				}
				return u.terminalViewArea(gtx, t)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if showStatusBar {
					return u.terminalStatusBar(gtx, t)
				}
				return layout.Dimensions{}
			}),
		)
	})
}

// splitPage is a split tab: two terminals side by side (or stacked).
func (u *ui) splitPage(gtx layout.Context, p, sec *tab, showStatusBar, tabChanged bool) layout.Dimensions {
	sp := u.splits[p.sess.TmpID]
	sp.paned.Force50 = u.cfg != nil && u.cfg.General.ForceSplit50
	sp.paned.Vertical = sp.orientation == "vertical"
	return u.withFiles(gtx, p, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return sp.paned.Layout(gtx, u.th,
					func(gtx layout.Context) layout.Dimensions {
						if (u.focus || tabChanged) && !u.prompter.Active() && !u.lock.active {
							p.view.Focus(gtx)
							u.focus = false
						}
						return u.terminalViewArea(gtx, p)
					},
					func(gtx layout.Context) layout.Dimensions {
						return u.terminalViewArea(gtx, sec)
					},
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if showStatusBar {
					return u.terminalStatusBar(gtx, p)
				}
				return layout.Dimensions{}
			}),
		)
	})
}

// terminalStatusBar is the status bar under the terminal
// (TerminalTab.svelte's statusBar snippet): 30px high on the header color
// with a top border, padding 0 6px, 11px text, 6px gaps; the terminal and
// cluster icons and the status on the left, the file browser and Info
// buttons on the right.
func (u *ui) terminalStatusBar(gtx layout.Context, t *tab) layout.Dimensions {
	p := u.th.Pal
	if t.sftpBtn.Clicked(gtx) {
		t.sftpOpen = !t.sftpOpen
	}
	st := t.sess.State()
	connected := st == "connected"
	termColor := color.NRGBA{R: 0xe0, G: 0x1b, B: 0x24, A: 0xff} // asbru-terminal-ko
	if connected {
		termColor = iconGreen // asbru-terminal-ok
	}
	var cluster string
	if info := u.core.Sessions.GetInfo(t.sess.TmpID); info != nil {
		cluster = info.Cluster
	}
	base := t.sess.Status() // the last status message, else the state
	if base == "" {
		base = strings.ToUpper(st)
	}
	status := "Status: " + base
	if cluster != "" {
		status = "[IN CLUSTER: " + cluster + "] - " + status
	}
	gap := layout.Rigid(layout.Spacer{Width: 6}.Layout)
	left := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return u.th.IconBox(gtx, fonts.IconTerminal, 16, termColor)
	})}
	if u.clustersVisible || cluster != "" {
		iconC := p.MutedFg
		if cluster != "" {
			iconC = p.SuccessFg
		}
		left = append(left, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.th.IconBox(gtx, fonts.IconSitemap, 16, iconC)
		}))
	}
	left = append(left, gap, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.W.Layout(gtx, gtk.Text{S: status, Size: unit.Sp(11), Color: p.FgColor, MaxLines: 1}.Widget(u.th))
	}))
	// .status-btn: transparent, 1px border, radius 3px, padding 1px 3px.
	statusBtn := func(icon rune, c color.NRGBA, active bool, click *widget.Clickable) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 4, Right: 4, Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return u.th.IconBox(gtx, icon, 16, c)
					})
				})
				r := image.Rectangle{Max: d.Size}
				bw := max(1, gtx.Dp(1))
				switch {
				case active:
					paint.FillShape(gtx.Ops, p.Accent, clip.UniformRRect(r, gtx.Dp(3)).Op(gtx.Ops))
					paint.FillShape(gtx.Ops, p.BtnActiveBg, clip.UniformRRect(r.Inset(bw), gtx.Dp(3)-bw).Op(gtx.Ops))
				case click.Hovered():
					paint.FillShape(gtx.Ops, p.BorderColor, clip.UniformRRect(r, gtx.Dp(3)).Op(gtx.Ops))
					paint.FillShape(gtx.Ops, p.BtnHoverBg, clip.UniformRRect(r.Inset(bw), gtx.Dp(3)-bw).Op(gtx.Ops))
				}
				call.Add(gtx.Ops)
				return d
			})
		})
	}
	var right []layout.FlexChild
	// The file browser (local shells and SSH with SFTP); its panel is stage 5.
	if t.kind == "local" || t.hasSFTP {
		ic, c := fonts.IconFolder, toolbarColors[fonts.IconFolderAdd]
		if t.kind != "local" {
			ic, c = fonts.IconBriefcase, iconSFTP
		}
		right = append(right, statusBtn(ic, c, t.sftpOpen, &t.sftpBtn), gap)
	}
	right = append(right, statusBtn(fonts.IconInfo, p.FgColor, false, &u.buttons.infoTabBtn))

	h := gtx.Dp(30)
	w := gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rect(0, 0, w, h), p.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, 0, w, max(1, gtx.Dp(1))), p.BorderColor)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w, h))
	layout.Inset{Left: 6, Right: 6, Top: 1}.Layout(g, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, left...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, right...)
			}),
		)
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// ---- global shortcuts ---------------------------------------------------------

const allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand

// globalKeys processes the shortcuts of MainWindow.svelte's handleKeyDown.
func (u *ui) globalKeys(gtx layout.Context) {
	// While a dialog, the lock screen or a terminal/text input holds the
	// focus, the keys belong there.
	if u.dialogOpen() || u.lock.active || u.treeMenu.Opened() || u.tabMenu.Opened() || u.termMenu.Opened() {
		return // an open menu owns the keyboard (ContextMenu.svelte)
	}
	if u.terminalFocused(gtx) {
		return
	}
	// The search entry: Escape closes the search, Shift+Enter searches
	// backwards (the web GUI's handleSearchKeyDown).
	if u.searchVisible && gtx.Source.Focused(&u.searchQuery) {
		for {
			ev, ok := gtx.Event(
				key.Filter{Focus: &u.searchQuery, Name: key.NameEscape},
				key.Filter{Focus: &u.searchQuery, Name: key.NameReturn, Optional: allMods},
			)
			if !ok {
				break
			}
			e, ok := ev.(key.Event)
			if !ok || e.State != key.Press {
				continue
			}
			switch e.Name {
			case key.NameEscape:
				u.searchVisible = false
				u.noteUIStateSoon()
			case key.NameReturn:
				if e.Modifiers.Contain(key.ModShift) {
					u.searchTree(true)
				}
			}
		}
		return
	}
	if gtx.Source.Focused(&u.notes) {
		return
	}
	// A terminal's find bar takes its own keys (Enter, Escape).
	for _, t := range u.tabs {
		if gtx.Source.Focused(&t.x.find.editor) {
			return
		}
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameF9, Optional: allMods},
			key.Filter{Name: key.NameF2, Optional: allMods},
			key.Filter{Name: key.NameDeleteForward, Optional: allMods},
			key.Filter{Name: key.NameEscape, Optional: allMods},
			key.Filter{Name: "F", Required: key.ModCtrl, Optional: allMods},
			key.Filter{Name: "S", Required: key.ModCtrl, Optional: allMods},
			key.Filter{Name: "L", Required: key.ModCtrl | key.ModAlt, Optional: allMods},
			key.Filter{Name: "Q", Required: key.ModCtrl, Optional: allMods},
		)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameF9:
			u.panelVisible = !u.panelVisible
			u.noteUIStateSoon()
		case e.Name == key.NameEscape:
			if u.searchVisible {
				u.searchVisible = false
				u.noteUIStateSoon()
			}
		case e.Name == key.NameF2:
			u.startRenameSelected()
		case e.Name == key.NameDeleteForward:
			u.startDeleteSelected()
		case e.Name == "F":
			u.searchVisible = !u.searchVisible
			u.noteUIStateSoon()
		case e.Name == "S":
			if err := u.configSvc.Save(); err != nil {
				u.dropMsg = err.Error()
			}
		case e.Name == "Q":
			u.requestQuit()
		case e.Name == "L":
			if _, err := u.sessionSvc.OpenLocalShell("tab"); err != nil {
				u.dropMsg = err.Error()
			}
		}
	}
}

// ---- drops -------------------------------------------------------------------

// handleDrops reports files dropped from the desktop (the editor is stage 7).
func (u *ui) handleDrops() {
	for {
		select {
		case d := <-u.drops:
			if u.dropFiles(d.Paths) {
				continue
			}
			u.dropMsg = fmt.Sprintf("Dropped: %s (LeanEdit is not implemented yet)", strings.Join(d.Paths, ", "))
		default:
			return
		}
	}
}

func (u *ui) onDrop(d dnd.Drop) {
	select {
	case u.drops <- d:
	default:
	}
	u.w.Invalidate()
}
