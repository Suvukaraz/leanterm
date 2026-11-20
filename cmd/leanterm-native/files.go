package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/filebrowser"
)

// The file browser next to a terminal (stage 5): SFTP for SSH tabs, the
// local file system for local shells (TerminalTab.svelte with SftpPanel).

// filePanel returns the tab's browser panel, created on first use.
func (u *ui) filePanel(t *tab) *filebrowser.Panel {
	if t.files != nil {
		return t.files
	}
	b := &filebrowser.Browser{TmpID: t.sess.TmpID, SFTP: u.sftpSvc, Local: u.localFS, Changed: u.w.Invalidate}
	if t.kind == "local" {
		b.Backend = filebrowser.LocalBackend{S: u.localFS}
	} else {
		b.Backend = filebrowser.SFTPBackend{S: u.sftpSvc}
	}
	id := t.sess.TmpID
	t.files = &filebrowser.Panel{
		B:             b,
		Connected:     func() bool { return t.sess.State() == "connected" },
		PointerPos:    func() image.Point { return u.lastPtr },
		TerminalInput: func(s string) { _ = u.core.Sessions.WriteBytes(id, []byte(s)) },
		ChooseFile: func(title, dir string, folder bool) (string, error) {
			return u.systemSvc.ChooseFile(title, dir, folder)
		},
		OnClose: func() { t.sftpOpen = false; u.focus = true },
	}
	return t.files
}

// hasFiles reports whether a tab has a file browser (local shells and SSH
// with SFTP).
func hasFiles(t *tab) bool { return t.kind == "local" || t.hasSFTP }

// withFiles lays out a terminal with its file browser beside it (left or
// right as configured, the width kept in the UI state).
func (u *ui) withFiles(gtx layout.Context, t *tab, term layout.Widget) layout.Dimensions {
	if !t.sftpOpen || !hasFiles(t) {
		return term(gtx)
	}
	fp := u.filePanel(t)
	sb := u.cfg.SFTPBrowser
	// SFTP needs the SSH connection: the browser opens once it is up.
	if t.kind == "local" || t.sess.State() == "connected" {
		fp.B.Open(sb.ShowHidden, sb.FollowTerminalFolder)
	}
	// Follow the terminal's folder (OSC 7, /proc).
	if d := t.sess.Dir(); d != "" && d != t.lastDir {
		t.lastDir = d
		fp.B.TerminalFolder(d)
	}
	width := u.sftpWidth
	if width <= 0 {
		width = sb.Width
	}
	if width <= 0 {
		width = 340
	}
	panel := func(gtx layout.Context) layout.Dimensions { return fp.Layout(gtx, u.th) }
	right := sb.Position == "right"
	p := &u.filesPaned
	p.MinFirst, p.MinSecond = 220, 150
	if right {
		p.MinFirst, p.MinSecond = 150, 220
	}
	// The pane keeps the panel's width: on the right it is measured from
	// the right edge.
	total := gtx.Constraints.Max.X
	if !p.Dragging() {
		if right {
			p.Pos = total - gtx.Dp(unit.Dp(width))
		} else {
			p.Pos = gtx.Dp(unit.Dp(width))
		}
	}
	first, second := panel, term
	if right {
		first, second = term, panel
	}
	d := p.Layout(gtx, u.th, first, second)
	if p.Dragging() {
		w := p.Pos
		if right {
			w = total - p.Pos
		}
		u.sftpWidth = int(float32(w) / gtx.Metric.PxPerDp)
		u.noteUIStateSoon()
	}
	return d
}

// fileOverlays draws the menus and dialogs of the active tab's browser.
func (u *ui) fileOverlays(gtx layout.Context) layout.Dimensions {
	filebrowser.ConfirmDelete = func() bool { return u.cfg == nil || u.cfg.SFTPBrowser.ConfirmDelete }
	if t := u.activeTab(); t != nil && t.files != nil {
		t.files.LayoutOverlays(gtx, u.th)
	}
	return layout.Dimensions{}
}

func (u *ui) activeTab() *tab {
	if u.nb.Selected == 0 || u.nb.Selected-1 >= len(u.tabs) {
		return nil
	}
	return u.tabs[u.nb.Selected-1]
}

// filesDialogOpen: the active browser shows a menu or a dialog.
func (u *ui) filesDialogOpen() bool {
	t := u.activeTab()
	return t != nil && t.files != nil && t.files.DialogOpen()
}

// watchFileEvents asks to upload files changed in their application back.
func (u *ui) watchFileEvents() {
	u.core.Bus.On(events.SFTPFileModified, func(_ string, data any) {
		m, ok := data.(map[string]any)
		if !ok {
			return
		}
		sftpID, _ := m["sftpID"].(string)
		local, _ := m["localPath"].(string)
		remote, _ := m["remotePath"].(string)
		u.filesEvents <- func() {
			for _, t := range u.tabs {
				if t.files != nil && t.files.B.Snapshot().ID == sftpID {
					t.files.FileModified(local, remote)
				}
			}
		}
		u.w.Invalidate()
	})
}

// dropFiles gives dropped files to the active tab's browser, if shown.
func (u *ui) dropFiles(paths []string) bool {
	t := u.activeTab()
	if t == nil || t.files == nil || !t.sftpOpen {
		return false
	}
	t.files.Drop(paths)
	return true
}
