package main

import (
	"fmt"
	"image/color"
	"log"
	"strings"

	"gioui.org/layout"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// terminalTabIndex returns the index of tab id's tab in the notebook
// (0 is the Info tab; -1 when there is no such tab).
func (u *ui) terminalTabIndex(tmpID string) int {
	for i, t := range u.tabs {
		if t.sess.TmpID == tmpID {
			return i + 1
		}
	}
	return -1
}

func (u *ui) tabByID(tmpID string) *tab {
	for _, t := range u.tabs {
		if t.sess.TmpID == tmpID {
			return t
		}
	}
	return nil
}

// sessionEvents adds and removes tabs for sessions the backend opened/closed.
func (u *ui) sessionEvents() {
	for {
		select {
		case info := <-u.opened:
			t, err := u.newTerminal(info)
			if err != nil {
				log.Printf("%s: %v", info.Title, err)
				continue
			}
			t.sftpOpen = u.sftpVisible && (t.kind == "local" || t.hasSFTP)
			t.nb = &gtk.Tab{Title: info.Title, Closable: true}
			u.tabs = append(u.tabs, t)
			u.rebuildNotebook()
			u.nb.Selected = len(u.nb.Tabs) - 1
			u.focus = true
		case id := <-u.closed:
			u.removeTab(id)
		default:
			return
		}
	}
}

func (u *ui) activeTabID() string {
	if u.nb.Selected == 0 || u.nb.Selected-1 >= len(u.tabs) {
		return "info"
	}
	return u.tabs[u.nb.Selected-1].sess.TmpID
}

// rebuildNotebook rebuilds the notebook tab list: the Info tab first, then
// the terminal tabs that are not the secondary half of a split.
func (u *ui) rebuildNotebook() {
	if u.infoTab == nil {
		u.infoTab = &gtk.Tab{Title: "Info ", Icon: fonts.IconBulb, Closable: false}
	}
	primary := map[string]bool{}
	for primaryID := range u.splits {
		primary[primaryID] = true
	}
	secondary := map[string]bool{}
	for _, s := range u.splits {
		secondary[s.secondary] = true
	}
	tabs := []*gtk.Tab{u.infoTab}
	for _, t := range u.tabs {
		if secondary[t.sess.TmpID] {
			continue
		}
		tabs = append(tabs, t.nb)
	}
	u.nb.Tabs = tabs
	if u.nb.Selected >= len(u.nb.Tabs) {
		u.nb.Selected = len(u.nb.Tabs) - 1
	}
	if u.nb.Selected < 0 {
		u.nb.Selected = 0
	}
}

// removeTab closes the tab of a session that went away (or asks to go away).
func (u *ui) removeTab(tmpID string) {
	// A session that was the secondary half of a split: drop the split, the
	// tab vanishes with it.
	for pid, s := range u.splits {
		if s.secondary == tmpID {
			delete(u.splits, pid)
			u.rebuildNotebook()
			break
		}
	}
	for i, t := range u.tabs {
		if t.sess.TmpID != tmpID {
			continue
		}
		t.sess.Close()
		u.tabs = append(u.tabs[:i], u.tabs[i+1:]...)
		// A split whose primary vanished is dropped; the secondary becomes
		// a regular tab again.
		delete(u.splits, tmpID)
		delete(u.newData, tmpID)
		u.rebuildNotebook()
		return
	}
}

// closeTab asks to close a tab: for a split tab both sessions are closed.
func (u *ui) closeTab(tmpID string) {
	if s, ok := u.splits[tmpID]; ok {
		_ = u.sessionSvc.Close(s.secondary)
	}
	_ = u.sessionSvc.Close(tmpID)
}

// closeOtherTabs closes every terminal tab except exceptID.
func (u *ui) closeOtherTabs(exceptID string) {
	for _, t := range append([]*tab{}, u.tabs...) {
		if t.sess.TmpID == exceptID {
			continue
		}
		u.closeTab(t.sess.TmpID)
	}
}

// closeAllTabs closes every terminal tab.
func (u *ui) closeAllTabs() {
	for _, t := range append([]*tab{}, u.tabs...) {
		u.closeTab(t.sess.TmpID)
	}
}

// splitTab splits primaryID's tab to show secondaryID's terminal too.
func (u *ui) splitTab(primaryID, secondaryID, orientation string) {
	if primaryID == secondaryID {
		return
	}
	if _, isSplit := u.splits[primaryID]; isSplit {
		return
	}
	// The secondary must not itself be a primary of another split.
	delete(u.splits, secondaryID)
	u.splits[primaryID] = &splitState{orientation: orientation, secondary: secondaryID, paned: gtk.NewPaned()}
	// Select the primary tab.
	u.rebuildNotebook()
	for i, t := range u.nb.Tabs {
		if t == u.tabByID(primaryID).nb {
			u.nb.Selected = i
		}
	}
	u.focus = true
}

// unsplitTab removes a split; the secondary becomes a regular tab.
func (u *ui) unsplitTab(primaryID string) {
	delete(u.splits, primaryID)
	u.rebuildNotebook()
}

// equalResize resets a split to an even ratio.
func (u *ui) equalResize(primaryID string) {
	if s, ok := u.splits[primaryID]; ok {
		s.paned.Pos = -1 // unset: split evenly
	}
}

// tabLook gives a terminal tab its title and an icon colored by the state.
func (u *ui) tabLook(t *tab) (string, rune, color.NRGBA) {
	icon := fonts.IconTerminal
	switch t.kind {
	case "ssh":
		icon = fonts.IconServer
	case "telnet":
		icon = fonts.IconPlug
	case "sftp":
		icon = fonts.IconGlobe
	}
	c := u.th.Pal.WarningFg
	if u.newData[t.sess.TmpID] && t.sess.State() == "connected" {
		c = parseHexColor(u.cfg.Terminal.Colors.NewData, color.NRGBA{R: 0x00, G: 0x88, B: 0xff, A: 0xff})
	} else {
		switch t.sess.State() {
		case "connected":
			c = parseHexColor(u.cfg.Terminal.Colors.Connected, color.NRGBA{R: 0x0c, G: 0xba, B: 0x00, A: 0xff})
		case "disconnected", "closed":
			c = parseHexColor(u.cfg.Terminal.Colors.Disconnected, color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff})
		}
	}
	title := t.sess.Title()
	if title == "" {
		title = "Terminal"
	}
	if _, isSplit := u.splits[t.sess.TmpID]; isSplit {
		title += " [Split]"
	}
	return title, icon, c
}

// parseHexColor parses "#rrggbb"; zero if unparsable.
func parseHexColor(s string, def color.NRGBA) color.NRGBA {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return def
	}
	var v uint32
	if _, err := fmt.Sscanf(s, "%x", &v); err != nil {
		return def
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// splitSessions returns the sessions of primaryID's split, if any.
func (u *ui) splitSessions(primaryID string) (primary, secondary *tab, ok bool) {
	s, isSplit := u.splits[primaryID]
	if !isSplit {
		return nil, nil, false
	}
	p, sec := u.tabByID(primaryID), u.tabByID(s.secondary)
	if p == nil || sec == nil {
		delete(u.splits, primaryID)
		u.rebuildNotebook()
		return nil, nil, false
	}
	return p, sec, true
}

// handleTabEvents processes the notebook's events.
func (u *ui) handleTabEvents() {
	for _, ev := range u.nb.Events() {
		switch ev.Kind {
		case gtk.TabSelected:
			u.focus = true
		case gtk.TabClosed:
			if ev.Tab == 0 { // the Info tab is not closable
				continue
			}
			id := u.tabIDAt(ev.Tab)
			if id != "" && id != "info" {
				u.closeTab(id)
			}
		case gtk.TabMiddleClick:
			if ev.Tab == 0 {
				continue
			}
			id := u.tabIDAt(ev.Tab)
			if id != "" && id != "info" {
				u.closeTab(id)
			}
		case gtk.TabContextClick:
			if ev.Tab == 0 {
				continue
			}
			id := u.tabIDAt(ev.Tab)
			if id != "" {
				u.openTabMenu(id, ev.Pos)
			}
		case gtk.TabMoved:
			u.moveTab(ev.From, ev.To)
		}
	}
	// The Info tab must stay first.
	if len(u.nb.Tabs) > 0 && u.nb.Tabs[0] != u.infoTab {
		u.rebuildNotebook()
	}
}

// tabIDAt maps a notebook tab index to the tmpID ("" when unknown).
func (u *ui) tabIDAt(i int) string {
	if i == 0 || i-1 >= len(u.tabs) {
		return ""
	}
	return u.tabs[i-1].sess.TmpID
}

// moveTab reorders the terminal tabs after a drag in the notebook.
func (u *ui) moveTab(from, to int) {
	if from == 0 || to == 0 || from-1 >= len(u.tabs) || to-1 >= len(u.tabs) {
		return
	}
	t := u.tabs[from-1]
	u.tabs = append(u.tabs[:from-1], u.tabs[from:]...)
	idx := to - 1
	u.tabs = append(u.tabs[:idx], append([]*tab{t}, u.tabs[idx:]...)...)
	u.rebuildNotebook()
}

// unsplitDisconnected drops or closes splits whose sessions went away.
func (u *ui) unsplitDisconnected() {
	if u.cfg == nil || !u.cfg.Terminal.UnsplitDisconnected {
		return
	}
	for pid, s := range u.splits {
		prim := u.tabByID(pid)
		sec := u.tabByID(s.secondary)
		if prim == nil || sec == nil {
			continue
		}
		if prim.sess.State() == "disconnected" || sec.sess.State() == "disconnected" {
			u.unsplitTab(pid)
		}
	}
}

// terminalFocused reports whether the keyboard focus is on a terminal.
func (u *ui) terminalFocused(gtx layout.Context) bool {
	for _, t := range u.tabs {
		if gtx.Source.Focused(t.view) {
			return true
		}
	}
	return false
}
