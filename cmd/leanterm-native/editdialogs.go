package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/dialogs"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Stage 6: the dialogs that edit the configuration.

// setupEditDialogs connects the dialogs to the services.
func (u *ui) setupEditDialogs(readOnly bool) {
	form.Fonts = func() []gtk.FontFamily {
		fams, err := u.systemSvc.ListFonts()
		if err != nil {
			return nil
		}
		out := make([]gtk.FontFamily, len(fams))
		for i, f := range fams {
			out[i] = gtk.FontFamily{Name: f.Name, Styles: f.Styles, Monospace: f.Monospace}
		}
		return out
	}
	e := &u.editConn
	e.Config = u.configSvc
	e.ReadOnly = readOnly
	e.GlobalLook = func() config.TerminalLookConfig { return u.core.Store.Config().Terminal.Look }
	e.OpenFolder = func(dir string) {
		if dir == "" {
			dir = u.systemSvc.SessionLogsDir()
		}
		_ = u.systemSvc.OpenPath(dir)
	}
	e.Saved = func() { u.rebuildTree() }
	e.F.ChooseFile = func(title, dir string, folder bool) (string, error) {
		return u.systemSvc.ChooseFile(title, dir, folder)
	}
	e.F.Changed = u.w.Invalidate

	p := &u.prefs
	p.Config = u.configSvc
	p.ChooseSaveFile = u.systemSvc.ChooseSaveFile
	p.OpenFolder = func(dir string) { _ = u.systemSvc.OpenPath(dir) }
	p.LogsDir, p.ScreenshotsDir = u.systemSvc.SessionLogsDir, u.systemSvc.ScreenshotsDir
	p.Invalidate = u.w.Invalidate
	p.F.ChooseFile = e.F.ChooseFile
	p.F.Changed = u.w.Invalidate
	u.setupSmallDialogs()
	p.Saved = func(before, after services.Settings) {
		// The new preference wins over a dragged panel width.
		if before.SFTPBrowser.Width != after.SFTPBrowser.Width {
			u.sftpWidth = 0
			u.noteUIStateSoon()
		}
	}
}

// openEdit opens the Edit Connection dialog for the selected connection.
func (u *ui) openEdit() {
	if n := u.singleSelected(); n != nil && !n.IsGroup && n.ID != "" {
		u.editConn.Open(n.ID)
	}
}

// treeToolbarClicks handles the tree toolbar (CommandPanel.svelte): add
// group or connection under the selected group (or the root), edit,
// rename and delete.
func (u *ui) treeToolbarClicks(gtx layout.Context) {
	b := &u.buttons
	parent := ""
	if n := u.singleSelected(); n != nil && n.IsGroup {
		parent = n.ID
	}
	if b.addGroup.Clicked(gtx) && u.canAdd() {
		u.openAddRename(false, true, parent, "", "", "")
	}
	if b.addConn.Clicked(gtx) && u.canAdd() {
		u.openAddRename(false, false, parent, "", "", "")
	}
	if b.edit.Clicked(gtx) && u.canEdit() {
		u.openEdit()
	}
	if b.rename.Clicked(gtx) && u.canRename() {
		u.startRenameSelected()
	}
	if b.delete.Clicked(gtx) && u.canDelete() {
		u.startDeleteSelected()
	}
}

// setupSmallDialogs connects Wake On LAN, Quick Connect, the variables,
// bulk edit and cluster windows to the services.
func (u *ui) setupSmallDialogs() {
	inv := u.w.Invalidate
	u.wol.Send, u.wol.LookupArp, u.wol.Invalidate = u.systemSvc.WakeOnLan, u.systemSvc.LookupArpMAC, inv
	u.quick.Invalidate = inv
	u.quick.Connect = func(title, method, host string, port int, user, password string) error {
		_, err := u.sessionSvc.OpenQuick(fmt.Sprintf("quick-%d", time.Now().UnixMilli()), title, method, host, port, user, password)
		return err
	}
	u.editVars.Load, u.editVars.Save, u.editVars.Invalidate = u.configSvc.GetConnectionForEdit, u.configSvc.SaveConnection, inv
	u.bulk.Apply, u.bulk.Invalidate = u.configSvc.ApplyBulkEdit, inv
	u.bulk.Applied = u.rebuildTree

	cs := u.clusterSvc
	u.clusterFns = dialogs.Clusters{
		List: cs.List,
		Sessions: func() []dialogs.Session {
			var out []dialogs.Session
			for _, s := range u.sessionSvc.List() {
				out = append(out, dialogs.Session{TmpID: s.TmpID, Title: s.Title, Kind: string(s.Kind)})
			}
			return out
		},
		Create:  cs.Create,
		Delete:  cs.Delete,
		Add:     cs.Add,
		Remove:  cs.Remove,
		Matches: cs.AutoClusterMatches,
		SaveRule: func(r config.AutoClusterConfig) error {
			cfg := u.core.Store.Config()
			cfg.AutoClusters = append(cfg.AutoClusters, r)
			return u.core.Store.Save()
		},
		Saved: func() [][2]string {
			var out [][2]string
			for _, c := range u.core.Store.Config().Connections {
				title := c.Title
				if title == "" {
					title = c.Name
				}
				cl := strings.Join(c.Clusters, ", ")
				if cl == "" {
					cl = "(no clusters)"
				}
				out = append(out, [2]string{title, cl})
			}
			return out
		},
		Send:      cs.PCCSend,
		Restart:   cs.RestartAll,
		CloseAll:  cs.CloseAll,
		PreventSB: cs.SetPreventSingleBroadcast,
	}
	u.clAdmin.C, u.pcc.C = &u.clusterFns, &u.clusterFns
	u.clAdmin.OpenPCC, u.pcc.OpenClusters = u.pcc.Open, u.clAdmin.Open
}
