package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- message dialog (quit / delete confirmations) ---------------------------

// messageDialog is a MessageDialog.svelte: icon, primary and secondary text,
// two buttons. used by QuitConfirm.svelte and DeleteConfirmDialog.svelte.
type messageDialog struct {
	dlg       gtk.Dialog
	open      bool
	wasOpen   bool
	title     string
	icon      rune
	primary   string
	secondary string
	noLabel   string
	yesLabel  string
	no        widget.Clickable
	yes       widget.Clickable
}

func (m *messageDialog) openWith(title string, icon rune, primary, secondary, noLabel, yesLabel string) {
	m.open, m.title, m.icon, m.primary, m.secondary, m.noLabel, m.yesLabel = true, title, icon, primary, secondary, noLabel, yesLabel
}

// Layout draws the dialog; yes reports whether the primary answer was chosen.
func (m *messageDialog) Layout(gtx layout.Context, th *gtk.Theme) bool {
	if !m.open {
		return false
	}
	m.dlg.Title = m.title
	m.dlg.Width = 440
	m.dlg.Closable = false
	yes, no := m.yes.Clicked(gtx), m.no.Clicked(gtx)
	m.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		p := th.Pal
		return layout.Inset{Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.IconBox(gtx, m.icon, 32, p.WarningFg)
				}),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					kids := []layout.FlexChild{}
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: m.primary, Size: gtk.Em(1.05), Color: p.FgColor, Weight: font.SemiBold}.Layout(gtx, th)
					}))
					if m.secondary != "" {
						kids = append(kids, layout.Rigid(layout.Spacer{Height: 6}.Layout))
						kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: m.secondary, Size: gtk.Em(0.95), Color: p.FgColor, LineHeight: 1.3}.Layout(gtx, th)
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				}),
			)
		})
	},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: m.noLabel, Icon: fonts.IconClose}
			return b.Layout(gtx, th, &m.no)
		},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: m.yesLabel, Icon: fonts.IconCheck, Primary: true}
			return b.Layout(gtx, th, &m.yes)
		},
	)
	if yes || no {
		m.open = false
	}
	return yes
}

// ---- quit & delete -----------------------------------------------------------

type quitDialog struct {
	messageDialog
	// editorCase reports that the pending quit was stopped at the
	// unsaved-documents check: Yes skips that check on the next round,
	// No shows the editor (QuitConfirm.svelte).
	editorCase bool
}

type deleteDialog struct {
	messageDialog
	ids []string
}

// requestQuit runs the QuitConfirm chain: unsaved editor documents, running
// SFTP transfers, open terminals, then (when configured) every exit first.
func (u *ui) requestQuit() { u.requestQuitChecked(false) }

func (u *ui) requestQuitChecked(editorChecked bool) {
	if !editorChecked && u.editorSvc.HasUnsaved() {
		u.quit.editorCase = true
		u.quit.openWith("Confirm Exit", fonts.IconWarning,
			"Unsaved Changes in the Editor",
			"The editor has documents with unsaved changes. Quit anyway and discard them? (No shows the editor.)",
			"No", "Yes")
		return
	}
	u.quit.editorCase = false
	if n := u.sftpSvc.RunningTransfers(); n > 0 {
		u.quit.openWith("Confirm Exit", fonts.IconWarning,
			"Active File Transfers in Progress",
			"There are active SFTP transfers in progress. Do you want to cancel the transfers and quit?",
			"No", "Yes")
		return
	}
	if n := len(u.tabs); n > 0 {
		u.quit.openWith("Confirm Exit", fonts.IconWarning,
			"Active Connections Open",
			fmt.Sprintf("There are %d active connection(s) open. Are you sure you want to disconnect and quit?", n),
			"No", "Yes")
		return
	}
	if u.cfg != nil && u.cfg.General.ConfirmExit {
		u.quit.openWith("Confirm Exit", fonts.IconQuestion,
			"Quit LeanTerm",
			"Are you sure you want to close LeanTerm?",
			"No", "Yes")
		return
	}
	u.exitNow()
}

// handleQuitDialog draws the quit dialog and applies the answer; it reports
// whether the dialog is open.
func (u *ui) handleQuit(gtx layout.Context) {
	if u.quit.open && !u.quit.wasOpen {
		gtx.Execute(key.FocusCmd{Tag: nil}) // nothing behind the backdrop holds the focus
	}
	u.quit.wasOpen = u.quit.open
	if !u.quit.open {
		return
	}
	wasEditorCase := u.quit.editorCase
	if u.quit.Layout(gtx, u.th) {
		// Yes.
		if wasEditorCase {
			// Restart the chain without the unsaved-documents check
			// (QuitConfirm.svelte).
			u.requestQuitChecked(true)
			return
		}
		// Yes: quit for good.
		u.exitNow()
		return
	}
	if !u.quit.open && wasEditorCase {
		// No: show the editor (QuitConfirm.svelte).
		_ = u.editorSvc.ShowWindow()
		u.quit.editorCase = false
	}
}

// startDelete opens the delete confirmation for the current selection.
func (u *ui) startDelete(ids []string) {
	n := len(ids)
	msg := fmt.Sprintf("Delete %d node(s) and ALL of their contents?", n)
	if n == 1 {
		msg = "Delete selected node and ALL of its contents?"
	}
	u.del.openWith("Confirm Delete", fonts.IconWarning, msg, "", "No", "Yes")
	u.del.ids = append([]string{}, ids...)
}

// handleDeleteDialog draws the delete dialog and applies the answer.
func (u *ui) handleDelete(gtx layout.Context) {
	if u.del.open && !u.del.wasOpen {
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	u.del.wasOpen = u.del.open
	if !u.del.open {
		return
	}
	if !u.del.Layout(gtx, u.th) {
		return
	}
	ids := u.del.ids
	if err := u.configSvc.DeleteNodes(ids); err != nil {
		u.dropMsg = err.Error()
		return
	}
	u.selected = nil
	u.rebuildTree()
	u.saveUIState()
}

// ---- add / rename node -------------------------------------------------------

type addRenameDialog struct {
	dlg         gtk.Dialog
	open        bool
	isRename    bool
	isGroup     bool
	parentID    string
	nodeID      string
	name        widget.Editor
	title       widget.Editor
	ok          widget.Clickable
	cancel      widget.Clickable
	errMsg      string
	focusName   bool
	currentName string
}

// openAddRename opens the dialog for a new node or to rename one.
func (u *ui) openAddRename(isRename, isGroup bool, parentID, nodeID, currentName, currentTitle string) {
	d := &u.addRen
	d.open, d.isRename, d.isGroup = true, isRename, isGroup
	d.parentID, d.nodeID = parentID, nodeID
	d.name.SetText(currentName)
	d.title.SetText(currentTitle)
	d.errMsg = ""
	d.focusName = true
	d.currentName = currentName
}

// Layout draws the add/rename dialog.
func (u *ui) handleAddRename(gtx layout.Context) {
	d := &u.addRen
	if !d.open {
		return
	}
	kind := "Connection"
	if d.isGroup {
		kind = "Group"
	}
	if d.isRename {
		d.dlg.Banner = fmt.Sprintf("Renaming %s '%s'", kind, d.currentName)
		d.dlg.Title = "Rename Node"
	} else {
		parent := d.parentID
		if parent == "" {
			parent = "ROOT"
		}
		d.dlg.Banner = fmt.Sprintf("Adding new %s into '%s'", kind, parent)
		d.dlg.Title = "Add Node"
	}
	d.dlg.Width = 420
	d.dlg.Closable = true
	showTitle := d.isRename && !d.isGroup

	confirm := func() bool {
		name := strings.TrimSpace(d.name.Text())
		if name == "" {
			d.errMsg = "Name cannot be empty"
			return false
		}
		var err error
		if d.isRename {
			err = u.configSvc.RenameNode(d.nodeID, name, strings.TrimSpace(d.title.Text()))
		} else {
			_, err = u.configSvc.AddNode(d.parentID, d.isGroup, name, "")
		}
		if err != nil {
			d.errMsg = err.Error()
			return false
		}
		return true
	}

	if d.dlg.CloseRequested() {
		d.open = false
		return
	}
	// Enter in the fields confirms.
	submit := func() {
		if confirm() {
			d.open = false
			u.finishTreeChange()
		}
	}
	for {
		ev, ok := d.name.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			submit()
		}
	}
	for {
		ev, ok := d.title.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			submit()
		}
	}

	ok, cancel := d.ok.Clicked(gtx), d.cancel.Clicked(gtx)
	d.dlg.Layout(gtx, u.th, func(gtx layout.Context) layout.Dimensions {
		if d.focusName {
			d.focusName = false
			gtx.Execute(key.FocusCmd{Tag: &d.name})
		}
		p := u.th.Pal
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if d.errMsg == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							gtk.Fill(gtx, p.ErrorBg)
							return layout.Dimensions{Size: gtx.Constraints.Max}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 10, Right: 10, Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return gtk.Text{S: d.errMsg, Size: gtk.Em(0.9), Color: p.ErrorFg}.Layout(gtx, u.th)
							})
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return formRow(gtx, u.th, "Enter new NAME:", func(gtx layout.Context) layout.Dimensions {
					ph := "e.g. web-prod-01"
					if d.isGroup {
						ph = "e.g. Production Servers"
					}
					return gtk.Entry{Placeholder: ph, Width: -1}.Layout(gtx, u.th, &d.name)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !showTitle {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return formRow(gtx, u.th, "Enter new TITLE:", func(gtx layout.Context) layout.Dimensions {
						return gtk.Entry{Placeholder: "e.g. web-prod-01 (Prod US-East)", Width: -1}.Layout(gtx, u.th, &d.title)
					})
				})
			}),
		)
	},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: "Cancel", Icon: fonts.IconClose}
			return b.Layout(gtx, u.th, &d.cancel)
		},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: "OK", Icon: fonts.IconCheck, Default: true}
			return b.Layout(gtx, u.th, &d.ok)
		},
	)
	if cancel {
		d.open = false
		return
	}
	if ok {
		if confirm() {
			d.open = false
			u.finishTreeChange()
		}
	}
}

// formRow is a bold label over its field (the web view's .form-row).
func formRow(gtx layout.Context, th *gtk.Theme, label string, field layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: label, Size: gtk.Em(0.9), Color: th.Pal.FgColor, Weight: font.SemiBold}.Layout(gtx, th)
			})
		}),
		layout.Rigid(field),
	)
}

// finishTreeChange reloads the tree after an add/rename/delete.
func (u *ui) finishTreeChange() {
	u.rebuildTree()
	u.saveUIState()
}

// ---- about --------------------------------------------------------------------

type aboutDialog struct {
	dlg     gtk.Dialog
	open    bool
	wasOpen bool
	close   widget.Clickable
}

// handleAbout draws the About dialog (About.svelte).
func (u *ui) handleAbout(gtx layout.Context) {
	d := &u.about
	if d.open && !d.wasOpen {
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	d.wasOpen = d.open
	if !d.open {
		return
	}
	d.dlg.Title = "About LeanTerm"
	d.dlg.Width = 460
	d.dlg.Closable = true
	if d.dlg.CloseRequested() {
		d.open = false
		return
	}
	close := d.close.Clicked(gtx)
	d.dlg.Layout(gtx, u.th, func(gtx layout.Context) layout.Dimensions {
		p := u.th.Pal
		return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lg := gtk.Logo{}
					return lg.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: 16}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					kids := []layout.FlexChild{}
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: "LeanTerm", Size: gtk.Em(1.5), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, u.th)
					}))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: "Version 1.2.1", Size: gtk.Em(0.8), Color: p.MutedFg}.Layout(gtx, u.th)
					}))
					kids = append(kids, layout.Rigid(layout.Spacer{Height: 6}.Layout))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: "SSH, Telnet and SFTP connection manager for Linux and Windows, built with Go and Gio.", Size: gtk.Em(0.9), Color: p.FgColor, LineHeight: 1.4}.Layout(gtx, u.th)
					}))
					kids = append(kids, layout.Rigid(layout.Spacer{Height: 6}.Layout))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: "Licensed under the GNU General Public License v3 (GPLv3)", Size: gtk.Em(0.8), Color: p.MutedFg}.Layout(gtx, u.th)
					}))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				}),
			)
		})
	},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: "Close", Icon: fonts.IconClose, Default: true}
			return b.Layout(gtx, u.th, &d.close)
		},
	)
	if close {
		d.open = false
	}
}

// ---- keybindings ---------------------------------------------------------------

type keybindingsDialog struct {
	dlg       gtk.Dialog
	open      bool
	wasOpen   bool
	close     widget.Clickable
	scrollPos layout.Position
}

// keybindingRow is a line of KeybindingsPage.svelte's table.
type keybindingRow struct{ window, action, description, key string }

var keybindingRows = []keybindingRow{
	{"main", "find", "Find connection", "Ctrl+F"},
	{"main", "quit", "Quit application", "Ctrl+Q"},
	{"main", "localshell", "Open local shell", "Ctrl+Shift+T"},
	{"main", "showconnections", "Toggle connections tree", "Alt+N"},
	{"tree_connections", "edit_node", "Edit node", "Alt+E"},
	{"tree_connections", "add_favourite", "Add favourite", "Alt+F"},
	{"tree_connections", "del_favourite", "Remove favourite", "Alt+Shift+F"},
	{"tree_connections", "find", "Find in tree", "Ctrl+F"},
	{"tree_connections", "expand_all", "Expand all", "Ctrl+R"},
	{"tree_connections", "collapse_all", "Collapse all", "Ctrl+T"},
	{"tree_connections", "clone", "Clone connection", "Ctrl+D"},
	{"tree_connections", "copy", "Copy node", "Ctrl+C"},
	{"tree_connections", "cut", "Cut node", "Ctrl+X"},
	{"tree_connections", "paste", "Paste node", "Ctrl+V"},
	{"tree_connections", "protection", "Toggle node protection", "Alt+R"},
	{"tree_connections", "rename", "Rename node", "F2"},
	{"tree_connections", "connect_node", "Connect node", "Alt+C"},
	{"tabs", "close", "Close current tab", "Ctrl+F4"},
	{"tabs", "next", "Next tab", "Ctrl+Page Down"},
	{"tabs", "previous", "Previous tab", "Ctrl+Page Up"},
	{"terminal", "fullscreen", "Toggle fullscreen", "F11"},
	{"terminal", "start", "Start session", "Ctrl+Return"},
	{"terminal", "copy", "Copy selection", "Ctrl+Shift+C"},
	{"terminal", "paste", "Paste clipboard", "Ctrl+Shift+V"},
	{"terminal", "paste_password", "Paste password from vault", "Ctrl+P"},
	{"terminal", "disconnect", "Disconnect session", "Ctrl+Shift+W"},
	{"terminal", "close", "Close tab", "Ctrl+W"},
	{"terminal", "restart", "Restart connection", "Ctrl+Shift+R"},
	{"terminal", "find_terminal", "Find in terminal output", "Ctrl+F3"},
	{"terminal", "paste_primary", "Paste selection", "Shift+Insert"},
	{"terminal", "zoom_in", "Zoom in", "Ctrl+Plus"},
	{"terminal", "zoom_out", "Zoom out", "Ctrl+Minus"},
	{"terminal", "zoom_reset", "Zoom reset", "Ctrl+0"},
	{"terminal", "toggle_sftp_browser", "Toggle file browser panel", "Alt+Ctrl+S"},
	{"sftp_browser", "refresh", "Refresh remote files", "F5"},
	{"sftp_browser", "parent", "Navigate to parent folder", "Backspace"},
	{"sftp_browser", "new_folder", "Create new remote folder", "F7"},
	{"sftp_browser", "toggle_hidden", "Toggle hidden files", "Ctrl+H"},
	{"sftp_browser", "edit_path", "Focus remote path bar", "Ctrl+L"},
}

// handleKeybindings draws the keybindings reference (KeybindingsPage.svelte).
func (u *ui) handleKeybindings(gtx layout.Context) {
	d := &u.keys
	if d.open && !d.wasOpen {
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	d.wasOpen = d.open
	if !d.open {
		return
	}
	d.dlg.Title = "Keybindings"
	d.dlg.Banner = "Keyboard shortcut reference"
	d.dlg.Width = 640
	d.dlg.Height = 520
	d.dlg.Closable = true
	if d.dlg.CloseRequested() {
		d.open = false
		return
	}
	close := d.close.Clicked(gtx)
	d.dlg.Layout(gtx, u.th, func(gtx layout.Context) layout.Dimensions {
		p := u.th.Pal
		list := layout.List{Axis: layout.Vertical, Position: d.scrollPos}
		dims := list.Layout(gtx, len(keybindingRows)+1, func(gtx layout.Context, i int) layout.Dimensions {
			if i == 0 {
				// The info header.
				return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return gtk.Text{S: "Keyboard shortcut reference by window. Changing shortcuts is not supported yet.", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
				})
			}
			row := keybindingRows[i-1]
			bg := p.BaseColor
			if i%2 == 0 {
				bg = p.RowAlt
			}
			call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8, Right: 8, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(120), gtx.Dp(120)
							return gtk.Text{S: row.window, Mono: true, Size: gtk.Em(0.8), Color: p.FgColor}.Layout(gtx, u.th)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: row.action + ": " + row.description, Size: gtk.Em(0.85), Color: p.FgColor, MaxLines: 1}.Layout(gtx, u.th)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: row.key, Mono: true, Size: gtk.Em(0.8), Color: p.Accent}.Layout(gtx, u.th)
						}),
					)
				})
			})
			gtk.FillRect(gtx, image.Rectangle{Max: dims.Size}, bg)
			call.Add(gtx.Ops)
			return dims
		})
		d.scrollPos = list.Position
		return dims
	},
		func(gtx layout.Context) layout.Dimensions {
			b := gtk.Button{Label: "Close", Icon: fonts.IconClose, Default: true}
			return b.Layout(gtx, u.th, &d.close)
		},
	)
	if close {
		d.open = false
	}
}

// ---- lock overlay --------------------------------------------------------------

type lockOverlay struct {
	active   bool
	password widget.Editor
	error    string
	checking bool
	unlock   widget.Clickable
	focusPwd bool
}

// toggleLock locks the GUI, or asks for the password to unlock it (the button
// of MainButtonBar.svelte).
func (u *ui) toggleLock() {
	if u.lock.active {
		// Unlocking goes through the overlay.
		return
	}
	u.lock.active = true
	u.lock.password.SetText("")
	u.lock.error = ""
	u.lock.focusPwd = true
	u.systemSvc.LockGUI()
}

// handleLock draws the lock overlay (GuiLockOverlay.svelte); it reports
// whether the GUI is locked.
func (u *ui) handleLock(gtx layout.Context) bool {
	l := &u.lock
	if !l.active {
		return false
	}
	if l.focusPwd {
		l.focusPwd = false
		gtx.Execute(key.FocusCmd{Tag: &l.password})
	}
	size := gtx.Constraints.Max
	gtk.FillRect(gtx, image.Rectangle{Max: size}, color.NRGBA{R: 0x0f, G: 0x0f, B: 0x0f, A: 0xf2})

	// The card.
	p := u.th.Pal
	card := func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Dp(420), size.X-2*gtx.Dp(16))
		g := gtx
		g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, size.Y)}
		call, d := record(g, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 32, Right: 32, Top: 24, Bottom: 24}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				kids := []layout.FlexChild{}
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return u.th.IconBox(gtx, fonts.IconLock, 28, p.WarningFg)
						}),
						layout.Rigid(layout.Spacer{Width: 12}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: "LeanTerm Locked", Size: gtk.Em(1.4), Color: p.FgColor, Weight: font.SemiBold}.Layout(gtx, u.th)
						}),
					)
				}))
				kids = append(kids, layout.Rigid(layout.Spacer{Height: 16}.Layout))
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return gtk.Text{S: "Please enter your Master Password to unlock the interface:", Size: gtk.Em(0.9), Color: p.MutedFg}.Layout(gtx, u.th)
				}))
				kids = append(kids, layout.Rigid(layout.Spacer{Height: 12}.Layout))
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return gtk.Entry{Placeholder: "Master Password...", Password: true, Width: -1}.Layout(gtx, u.th, &l.password)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := gtk.Button{Label: "Unlock", Icon: fonts.IconCheck, Disabled: l.checking || l.password.Text() == ""}
							return b.Layout(gtx, u.th, &l.unlock)
						}),
					)
				}))
				if l.error != "" {
					kids = append(kids, layout.Rigid(layout.Spacer{Height: 12}.Layout))
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: l.error, Size: gtk.Em(0.85), Color: p.ErrorFg}.Layout(gtx, u.th)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
			})
		})
		r := image.Rectangle{Max: d.Size}
		paint.FillShape(gtx.Ops, p.BorderColor, clip.UniformRRect(r, gtx.Dp(6)).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, p.HeaderBg, clip.UniformRRect(r.Inset(1), gtx.Dp(5)).Op(gtx.Ops))
		call.Add(gtx.Ops)
		return d
	}

	// Unlocking: the button or Enter in the password field.
	tryUnlock := func() {
		pwd := l.password.Text()
		if pwd == "" {
			l.error = "Please enter password"
			return
		}
		l.checking = true
		l.error = ""
		err := u.systemSvc.UnlockGUI(pwd)
		l.checking = false
		if err != nil {
			l.error = err.Error()
			if l.error == "" {
				l.error = "Incorrect master password"
			}
			return
		}
		l.active = false
		l.password.SetText("")
	}
	for {
		ev, ok := l.password.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			tryUnlock()
		}
	}
	if l.unlock.Clicked(gtx) {
		tryUnlock()
	}
	layout.Center.Layout(gtx, card)
	return true
}
