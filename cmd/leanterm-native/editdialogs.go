package main

import (
	"gioui.org/layout"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
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
