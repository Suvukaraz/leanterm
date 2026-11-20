package main

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
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
	u.trackPointer(gtx)
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

	for _, t := range u.tabs {
		title, icon, c := u.tabLook(t)
		t.nb.Title, t.nb.Icon, t.nb.IconColor = title, icon, c
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
			bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.buttonBar(gtx) })
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
	return u.quit.open || u.del.open || u.addRen.open || u.about.open || u.keys.open || u.prompter.Active()
}

// ---- button bar --------------------------------------------------------------

// buttonBar is the main button bar (MainButtonBar.svelte).
func (u *ui) buttonBar(gtx layout.Context) layout.Dimensions {
	p := u.th.Pal
	const pad = 2
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 7, Right: 7, Top: pad + 1, Bottom: pad + 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pp := u.cfg != nil && u.cfg.General.ProtectWithPassword
			locked := u.lock.active
			btn := func(icon rune, active, disabled bool, click *widget.Clickable) layout.FlexChild {
				return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := gtk.Button{Variant: gtk.CP, Icon: icon, Active: active, Disabled: disabled}
					return b.Layout(gtx, u.th, click)
				})
			}
			labelled := func(icon rune, label string, click *widget.Clickable, active bool) layout.FlexChild {
				return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := gtk.Button{Variant: gtk.CP, Icon: icon, Label: label, Active: active}
					return b.Layout(gtx, u.th, click)
				})
			}
			lockIcon := fonts.IconUnlock
			if locked {
				lockIcon = fonts.IconLock
			}
			kids := []layout.FlexChild{
				labelled(fonts.IconList, "Connections", &u.buttons.panelToggle, u.panelVisible),
				labelled(fonts.IconExchange, "File Browser", &u.buttons.sftpToggle, u.sftpVisible),
				labelled(fonts.IconSitemap, "Clusters", &u.buttons.clustersToggle, u.clustersVisible),
				labelled(fonts.IconTerminal, "Local shell", &u.buttons.localShell, false),
				labelled(fonts.IconBolt, "Wake On Lan", &u.buttons.wol, false),
				labelled(fonts.IconCog, "Settings", &u.buttons.prefs, false),
				btn(lockIcon, locked, !pp, &u.buttons.lock),
				labelled(fonts.IconInfo, "About", &u.buttons.about, false),
			}
			spaced := []layout.FlexChild{}
			for i, k := range kids {
				if i > 0 {
					spaced = append(spaced, layout.Rigid(layout.Spacer{Width: 4}.Layout))
				}
				spaced = append(spaced, k)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, spaced...)
		})
	})
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, p.HeaderBg)
	call.Add(gtx.Ops)
	gtk.FillRect(gtx, image.Rectangle{Min: image.Pt(0, d.Size.Y-1), Max: d.Size}, p.BorderColor)
	return d
}

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
}

// splitPage is a split tab: two terminals side by side (or stacked).
func (u *ui) splitPage(gtx layout.Context, p, sec *tab, showStatusBar, tabChanged bool) layout.Dimensions {
	sp := u.splits[p.sess.TmpID]
	sp.paned.Force50 = u.cfg != nil && u.cfg.General.ForceSplit50
	sp.paned.Vertical = sp.orientation == "vertical"
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
}

// terminalViewArea draws the terminal background and the view.
func (u *ui) terminalViewArea(gtx layout.Context, t *tab) layout.Dimensions {
	gtk.Fill(gtx, u.th.Pal.TerminalBg)
	return t.view.Layout(gtx, u.th.Shaper)
}

// terminalStatusBar is the status bar under the terminal
// (TerminalTab.svelte's statusBar snippet).
func (u *ui) terminalStatusBar(gtx layout.Context, t *tab) layout.Dimensions {
	p := u.th.Pal
	_, _, c := u.tabLook(t)
	st := t.sess.State()
	icon := fonts.IconPlug
	if st != "connected" && st != "connecting" {
		icon = fonts.IconPowerOff
	}

	kids := []layout.FlexChild{}
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return u.th.IconBox(gtx, icon, 16, c)
	}))
	var cluster string
	if info := u.core.Sessions.GetInfo(t.sess.TmpID); info != nil {
		cluster = info.Cluster
	}
	if u.clustersVisible || cluster != "" {
		iconC := p.MutedFg
		if cluster != "" {
			iconC = p.SuccessFg
		}
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.th.IconBox(gtx, fonts.IconSitemap, 16, iconC)
		}))
	}
	status := "Status: " + strings.ToUpper(st)
	if s := t.sess.Status(); s != "" {
		status += " · " + s
	}
	if cluster != "" {
		status = "[IN CLUSTER: " + cluster + "] - " + status
	}
	kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return gtk.Text{S: status, Size: gtk.Em(0.85), Color: p.FgColor, MaxLines: 1}.Layout(gtx, u.th)
	}))
	kids = append(kids, layout.Rigid(u.th.StatusText("Engine: "+t.engine)))
	if d := t.sess.Dir(); d != "" {
		kids = append(kids, layout.Rigid(u.th.StatusText("Folder: "+d)))
	}
	// The Info tab button (the file browser button is stage 5).
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		b := gtk.Button{Variant: gtk.Flat, Icon: fonts.IconInfo}
		return b.Layout(gtx, u.th, &u.buttons.infoTabBtn)
	}))
	return u.th.Statusbar(gtx, kids...)
}

// ---- global shortcuts ---------------------------------------------------------

const allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand

// globalKeys processes the shortcuts of MainWindow.svelte's handleKeyDown.
func (u *ui) globalKeys(gtx layout.Context) {
	// While a dialog, the lock screen or a terminal/text input holds the
	// focus, the keys belong there.
	if u.dialogOpen() || u.lock.active {
		return
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
