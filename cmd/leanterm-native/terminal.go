package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/termview"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// The terminal of a tab beyond the view itself: the find bar
// (FindInTerminal.svelte), short notices, the macros bar (MacrosBar.svelte)
// and the context menu (TerminalContextMenu.svelte).

type findBar struct {
	open, focus                 bool
	editor                      widget.Editor
	matchCase, wholeWord, regex widget.Bool
	prev, next, close           widget.Clickable
	searcher                    termview.Searcher
	text                        string
}

type notice struct {
	text, folder string
	err          bool
	until        time.Time
	open, close  widget.Clickable
}

type termExtras struct {
	find       findBar
	notice     notice
	macroCombo gtk.Combo
	macroRun   widget.Clickable
	macroBtns  []widget.Clickable
	connID     string
	family     string
}

// lookFor is the look of a new terminal: the global one with the local
// shell's or the connection's personal look over it (TerminalTab.svelte).
func (u *ui) lookFor(info session.SessionInfo) config.TerminalLookConfig {
	if u.cfg == nil {
		return config.TerminalLookConfig{}
	}
	var personal *config.TerminalLookConfig
	if info.Kind == "local" {
		personal = u.cfg.LocalShell.Look
	} else if conn, ok := u.core.FindConnection(info.ConnID); ok && conn.Terminal.UsePersonal {
		personal = conn.Terminal.Look
	}
	return termview.MergeLook(u.cfg.Terminal.Look, personal)
}

// engineName is the engine of new terminals: -engine, else the setting.
func (u *ui) engineName() string {
	if u.engine != "" {
		return u.engine
	}
	if u.cfg != nil {
		return u.cfg.Terminal.Engine
	}
	return ""
}

// newTerminal attaches a view to a session the backend opened.
func (u *ui) newTerminal(info session.SessionInfo) (*tab, error) {
	look := u.lookFor(info)
	s, err := termview.AttachWith(u.core.Sessions, u.core.Bus, info.TmpID, termview.EngineOptions{
		Engine:     u.engineName(),
		Colors:     termview.LookColors(look),
		Scrollback: termview.LookScrollback(look),
	}, func() {
		if u.activeTabID() != info.TmpID && u.cfg != nil && u.cfg.Terminal.TabsPosition != "left" {
			u.newData[info.TmpID] = true
		}
		u.w.Invalidate()
	})
	if err != nil {
		return nil, err
	}
	t := &tab{sess: s, view: termview.New(s.Engine), engine: s.Engine.Name(), kind: info.Kind, hasSFTP: info.HasSFTP}
	t.x.connID = info.ConnID
	t.view.OnResize = s.Resize
	u.applyLook(t, look)
	t.view.OnOpenLink = func(uri string) {
		if err := u.systemSvc.OpenURL(uri); err != nil {
			u.showNotice(t, "Opening the link failed: "+err.Error(), "", true)
		}
	}
	t.view.OnContextMenu = func(image.Point) { u.openTermMenu(t) }
	t.view.OnShortcut = func(e key.Event) bool { return u.terminalShortcut(t, e) }
	return t, nil
}

// applyLook sets a terminal's font, cursor and selection behavior; colors
// and history apply to new terminals only, as in the web GUI.
func (u *ui) applyLook(t *tab, look config.TerminalLookConfig) {
	t.view.ApplyLook(look)
	t.x.family = termview.LookFamily(look)
	t.view.SelectionToClipboard = u.cfg != nil && u.cfg.Appearance.SelectionToClipboard
}

// applyLooks follows a changed look in the open terminals.
func (u *ui) applyLooks() {
	for _, t := range u.tabs {
		if info := u.core.Sessions.GetInfo(t.sess.TmpID); info != nil {
			u.applyLook(t, u.lookFor(*info))
		}
	}
}

// termShaper is the shaper for a terminal font family: the GUI's faces and
// the family's (regular, bold, italic) under its name.
func (u *ui) termShaper(family string) *text.Shaper {
	if family == "" {
		return u.th.Shaper
	}
	if sh, ok := u.shapers[family]; ok {
		return sh
	}
	sh := familyShaper(family)
	if u.shapers == nil {
		u.shapers = map[string]*text.Shaper{}
	}
	u.shapers[family] = sh
	return sh
}

// familyShaper is a shaper with the GUI's fonts and an installed family
// (regular, bold, italic, bold italic) under the family's name.
func familyShaper(family string) *text.Shaper {
	coll := append([]font.FontFace{}, fonts.Collection()...)
	for _, v := range []struct {
		style  string
		weight font.Weight
		italic font.Style
	}{{"", font.Normal, font.Regular}, {"Bold", font.Bold, font.Regular}, {"Italic", font.Normal, font.Italic}, {"Bold Italic", font.Bold, font.Italic}} {
		if f, ok := fonts.Face(family, v.style); ok {
			f.Font = font.Font{Typeface: font.Typeface(family), Weight: v.weight, Style: v.italic}
			coll = append(coll, f)
		}
	}
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(coll))
}

// terminalShortcut handles TerminalView.svelte's application keys.
func (u *ui) terminalShortcut(t *tab, e key.Event) bool {
	ctrl, shift, alt := e.Modifiers.Contain(key.ModCtrl), e.Modifiers.Contain(key.ModShift), e.Modifiers.Contain(key.ModAlt)
	switch {
	case ctrl && e.Name == key.NameF3, ctrl && shift && !alt && e.Name == "F":
		t.x.find.open, t.x.find.focus = true, true
	case ctrl && alt && e.Name == "S" && (t.kind == "local" || t.hasSFTP):
		t.sftpOpen = !t.sftpOpen
	case ctrl && shift && !alt && e.Name == "W":
		u.closeTab(t.sess.TmpID)
	default:
		return false
	}
	u.w.Invalidate()
	return true
}

func (u *ui) showNotice(t *tab, text, folder string, isErr bool) {
	d := 6 * time.Second
	if isErr {
		d = 10 * time.Second
	}
	t.x.notice.text, t.x.notice.folder, t.x.notice.err = text, folder, isErr
	t.x.notice.until = time.Now().Add(d)
	u.w.Invalidate()
}

// terminalViewArea is the macros bar and the terminal with the find bar and
// the notice over it.
func (u *ui) terminalViewArea(gtx layout.Context, t *tab) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.macrosBar(gtx, t) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtk.Fill(gtx, u.th.Pal.TerminalBg)
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return t.view.Layout(gtx, u.termShaper(t.x.family))
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 4, Right: 20}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return u.findPanel(gtx, t)
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 12, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return u.noticePanel(gtx, t)
						})
					})
				}),
			)
		}),
	)
}

// floatPanel draws w on the window background with a 1px border and radius 4
// (the find bar and the notice).
func (u *ui) floatPanel(gtx layout.Context, border color.NRGBA, pad layout.Inset, w layout.Widget) layout.Dimensions {
	p := u.th.Pal
	gtx.Constraints.Min = image.Point{}
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions { return pad.Layout(gtx, w) })
	r := image.Rectangle{Max: d.Size}
	rad, bw := gtx.Dp(4), max(1, gtx.Dp(1))
	// box-shadow 0 3px 8px.
	for i := 1; i <= 3; i++ {
		s := gtx.Dp(unit.Dp(i * 2))
		sr := image.Rectangle{Min: image.Pt(-s/2, -s/2+gtx.Dp(3)), Max: d.Size.Add(image.Pt(s/2, s/2+gtx.Dp(3)))}
		paint.FillShape(gtx.Ops, color.NRGBA{A: 22}, clip.UniformRRect(sr, rad+s/2).Op(gtx.Ops))
	}
	paint.FillShape(gtx.Ops, border, clip.UniformRRect(r, rad).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, p.BgColor, clip.UniformRRect(r.Inset(bw), rad-bw).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return d
}

// findPanel is FindInTerminal.svelte: the entry with previous, next and
// close, and the match options; Enter finds the next match, Shift+Enter the
// previous one, Escape closes. Typing searches as you type.
func (u *ui) findPanel(gtx layout.Context, t *tab) layout.Dimensions {
	fb := &t.x.find
	if !fb.open {
		return layout.Dimensions{}
	}
	opts := func() termview.SearchOptions {
		return termview.SearchOptions{CaseSensitive: fb.matchCase.Value, WholeWord: fb.wholeWord.Value, Regex: fb.regex.Value}
	}
	find := func(backward bool) {
		if fb.text != "" {
			fb.searcher.Find(t.view, fb.text, opts(), backward)
		}
	}
	closeBar := func() {
		fb.open = false
		u.focus = true
	}
	// Shift+Enter and Escape before the entry takes the keys.
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &fb.editor, Name: key.NameReturn, Required: key.ModShift},
			key.Filter{Focus: &fb.editor, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			if e.Name == key.NameEscape {
				closeBar()
				return layout.Dimensions{}
			}
			find(true)
		}
	}
	for {
		ev, ok := fb.editor.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.SubmitEvent:
			find(false)
		case widget.ChangeEvent:
			fb.text = fb.editor.Text()
			find(false)
		}
	}
	if fb.prev.Clicked(gtx) {
		find(true)
	}
	if fb.next.Clicked(gtx) {
		find(false)
	}
	if fb.close.Clicked(gtx) {
		closeBar()
		return layout.Dimensions{}
	}
	if fb.focus {
		gtx.Execute(key.FocusCmd{Tag: &fb.editor})
		fb.focus = false
	}
	p := u.th.Pal
	return u.floatPanel(gtx, p.BorderColor, layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
		btn := func(icon rune, c *widget.Clickable) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Icon: icon}.Layout(gtx, u.th, c)
			})
		}
		gap := func(w unit.Dp) layout.FlexChild { return layout.Rigid(layout.Spacer{Width: w}.Layout) }
		check := func(b *widget.Bool, label string) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return u.th.CheckButton(gtx, b, label, false)
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Entry{Placeholder: "Find in terminal...", Width: 180}.Layout(gtx, u.th, &fb.editor)
					}),
					gap(4), btn(fonts.IconArrowLeft, &fb.prev),
					gap(4), btn(fonts.IconArrowRight, &fb.next),
					gap(4), btn(fonts.IconClose, &fb.close),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					check(&fb.matchCase, "Match case"), gap(12),
					check(&fb.wholeWord, "Whole word"), gap(12),
					check(&fb.regex, "Regular expression"),
				)
			}),
		)
	})
}

// noticePanel is TerminalView.svelte's .terminal-notice: the text, "Open
// folder" for saved files, and ×.
func (u *ui) noticePanel(gtx layout.Context, t *tab) layout.Dimensions {
	n := &t.x.notice
	if n.text == "" {
		return layout.Dimensions{}
	}
	if n.close.Clicked(gtx) || gtx.Now.After(n.until) {
		n.text = ""
		return layout.Dimensions{}
	}
	if n.open.Clicked(gtx) && n.folder != "" {
		_ = u.systemSvc.OpenPath(n.folder)
	}
	gtx.Execute(op.InvalidateCmd{At: n.until})
	p := u.th.Pal
	border := p.BorderColor
	if n.err {
		border = p.ErrorFg
	}
	gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-gtx.Dp(24))
	return u.floatPanel(gtx, border, layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 8}, func(gtx layout.Context) layout.Dimensions {
		small := func(label string, c *widget.Clickable) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return gtk.Button{Label: label, Variant: gtk.Flat}.Layout(gtx, u.th, c)
				})
			})
		}
		kids := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return gtk.Text{S: n.text, Size: unit.Sp(12), Color: p.FgColor, MaxLines: 1}.Layout(gtx, u.th)
		})}
		if n.folder != "" {
			kids = append(kids, small("Open folder", &n.open))
		}
		kids = append(kids, small("×", &n.close))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
}

// ---- macros -------------------------------------------------------------------

type macroItem struct {
	remote       bool
	command      string
	description  string
	confirm      bool
	appendReturn bool
}

// macros lists the connection's macros, then the global ones ("[G] ").
func (u *ui) macros(t *tab) []macroItem {
	var out []macroItem
	add := func(ms []config.MacroConfig, remote bool, prefix string) {
		for _, m := range ms {
			if m.Command == "" {
				continue
			}
			d := m.Description
			if d == "" {
				d = m.Command
			}
			out = append(out, macroItem{remote: remote, command: m.Command, description: prefix + d, confirm: m.Confirm, appendReturn: remote && m.AppendReturn})
		}
	}
	if conn, ok := u.core.FindConnection(t.x.connID); ok && t.x.connID != "" {
		add(conn.Macros.Remote, true, "")
		add(conn.Macros.Local, false, "")
	}
	if u.cfg != nil {
		add(u.cfg.Macros.Remote, true, "[G] ")
		add(u.cfg.Macros.Local, false, "[G] ")
	}
	return out
}

func (u *ui) runMacro(t *tab, m macroItem) {
	if m.confirm {
		u.macroConfirm = gtk.MessageDialog{
			Type: gtk.MessageQuestion, Title: "Confirm Macro Execution", Buttons: gtk.ButtonsYesNo,
			Primary: fmt.Sprintf("Execute %s macro?", map[bool]string{true: "remote", false: "local"}[m.remote]), Secondary: "Command: " + m.command,
		}
		u.macroPending, u.macroTab, u.macroConfirmOpen = m, t, true
		return
	}
	u.execMacro(t, m)
}

func (u *ui) execMacro(t *tab, m macroItem) {
	var err error
	if m.remote {
		err = u.sessionSvc.RunRemoteMacro(t.sess.TmpID, m.command, m.appendReturn)
	} else {
		err = u.sessionSvc.RunLocalMacro(t.sess.TmpID, m.command)
	}
	if err != nil {
		u.showNotice(t, "The macro failed: "+err.Error(), "", true)
	}
}

// handleMacroConfirm shows the confirmation of a macro.
func (u *ui) handleMacroConfirm(gtx layout.Context) {
	if !u.macroConfirmOpen {
		return
	}
	u.macroConfirm.Layout(gtx, u.th)
	if r, ok := u.macroConfirm.Response(); ok {
		u.macroConfirmOpen = false
		if r == "yes" && u.macroTab != nil {
			u.execMacro(u.macroTab, u.macroPending)
		}
		u.focus = true
	}
}

// macrosBar is MacrosBar.svelte: "Macros:" and a button per macro, or a
// combo with Execute.
func (u *ui) macrosBar(gtx layout.Context, t *tab) layout.Dimensions {
	mode := ""
	if u.cfg != nil {
		mode = u.cfg.Terminal.MacrosBar
	}
	if mode == "" || mode == "never" {
		return layout.Dimensions{}
	}
	ms := u.macros(t)
	if len(ms) == 0 {
		return layout.Dimensions{}
	}
	p := u.th.Pal
	if len(t.x.macroBtns) != len(ms) {
		t.x.macroBtns = make([]widget.Clickable, len(ms))
	}
	for i := range ms {
		if t.x.macroBtns[i].Clicked(gtx) {
			u.runMacro(t, ms[i])
		}
	}
	if t.x.macroRun.Clicked(gtx) {
		var i int
		if _, err := fmt.Sscan(t.x.macroCombo.Value, &i); err == nil && i >= 0 && i < len(ms) {
			u.runMacro(t, ms[i])
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			kids := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Text{S: "Macros:", Size: unit.Sp(11), Color: p.MutedFg, Weight: font.SemiBold}.Layout(gtx, u.th)
			})}
			gap := layout.Rigid(layout.Spacer{Width: 8}.Layout)
			if mode == "combo" {
				var opts []gtk.ComboOption
				for i, m := range ms {
					opts = append(opts, gtk.ComboOption{Value: fmt.Sprint(i), Label: m.description})
				}
				kids = append(kids, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return t.x.macroCombo.Layout(gtx, u.th, opts, false)
				}), layout.Rigid(layout.Spacer{Width: 6}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return gtk.Button{Label: "Execute", Icon: fonts.IconPlay, Disabled: t.x.macroCombo.Value == ""}.Layout(gtx, u.th, &t.x.macroRun)
				}))
			} else {
				for i, m := range ms {
					icon := fonts.IconPlay
					if !m.remote {
						icon = fonts.IconHome
					}
					if i > 0 {
						kids = append(kids, layout.Rigid(layout.Spacer{Width: 6}.Layout))
					} else {
						kids = append(kids, gap)
					}
					kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Button{Label: m.description, Icon: icon}.Layout(gtx, u.th, &t.x.macroBtns[i])
					}))
				}
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff})
	call.Add(gtx.Ops)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-max(1, gtx.Dp(1)), d.Size.X, d.Size.Y), color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff})
	return d
}

// ---- context menu -----------------------------------------------------------

// openTermMenu opens TerminalContextMenu.svelte's menu at the pointer.
func (u *ui) openTermMenu(t *tab) {
	u.termMenuTarget = t
	id := t.sess.TmpID
	info := u.core.Sessions.GetInfo(id)
	cluster := ""
	if info != nil {
		cluster = info.Cluster
	}
	conn, hasConn := u.core.FindConnection(t.x.connID)
	hasConn = hasConn && t.x.connID != ""

	items := []gtk.MenuItem{
		{ID: "keybindings", Label: "Show Terminal Keybindings", Icon: fonts.IconQuestion},
		{Separator: true},
		{ID: "copy", Label: "Copy", Icon: fonts.IconCopy, Shortcut: "Ctrl+Shift+C", Disabled: t.view.SelectionText() == ""},
		{ID: "paste", Label: "Paste", Icon: fonts.IconPaste, Shortcut: "Ctrl+Shift+V"},
		{Separator: true},
	}
	if u.clustersVisible {
		sub := []gtk.MenuItem{{ID: "cluster-new", Label: "New Cluster..."}}
		for _, c := range u.clusterSvc.List() {
			sub = append(sub, gtk.MenuItem{ID: "cluster:" + c.Name, Label: fmt.Sprintf("%s (%d terminals)", c.Name, len(c.Sessions))})
		}
		label := "Add to Cluster"
		if cluster != "" {
			label = "Change Cluster (" + cluster + ")"
		}
		items = append(items, gtk.MenuItem{ID: "cluster", Label: label, Icon: fonts.IconSitemap, Submenu: sub})
		if cluster != "" {
			items = append(items, gtk.MenuItem{ID: "cluster-remove", Label: "Remove from Cluster"})
		}
		items = append(items, gtk.MenuItem{ID: "pcc", Label: "Power Cluster Controller", Icon: fonts.IconList}, gtk.MenuItem{Separator: true})
	}
	ms := u.macros(t)
	var remote, local []gtk.MenuItem
	for i, m := range ms {
		label := strings.TrimPrefix(m.description, "[G] ")
		if m.confirm {
			label = "CONFIRM: " + label
		}
		it := gtk.MenuItem{ID: fmt.Sprintf("macro:%d", i), Label: label}
		if m.remote {
			remote = append(remote, it)
		} else {
			local = append(local, it)
		}
	}
	if len(remote) > 0 {
		items = append(items, gtk.MenuItem{ID: "remote", Label: "Remote commands", Submenu: remote})
	}
	if len(local) > 0 {
		items = append(items, gtk.MenuItem{ID: "local", Label: "Local commands", Submenu: local})
	}
	var insert []gtk.MenuItem
	if hasConn && len(conn.Variables) > 0 {
		var sub []gtk.MenuItem
		for i, v := range conn.Variables {
			sub = append(sub, gtk.MenuItem{ID: fmt.Sprintf("var:%d", i), Label: fmt.Sprintf("<V:%d> %s", i, paren(v.Value))})
		}
		insert = append(insert, gtk.MenuItem{ID: "user-vars", Label: "User variables...", Submenu: sub})
	}
	if u.cfg != nil && len(u.cfg.Variables) > 0 {
		var sub []gtk.MenuItem
		for i, v := range u.cfg.Variables {
			sub = append(sub, gtk.MenuItem{ID: fmt.Sprintf("gvar:%d", i), Label: fmt.Sprintf("<GV:%s> %s", v.Name, paren(v.Value))})
		}
		insert = append(insert, gtk.MenuItem{ID: "global-vars", Label: "Global variables...", Submenu: sub})
	}
	if len(insert) > 0 {
		items = append(items, gtk.MenuItem{ID: "insert", Label: "Insert value", Submenu: insert})
	}
	items = append(items,
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "find", Label: "Find...", Icon: fonts.IconSearch, Shortcut: "Ctrl+Shift+F"},
		gtk.MenuItem{ID: "screenshot", Label: "Take Screenshot", Icon: fonts.IconCircle},
		gtk.MenuItem{ID: "log", Label: "Save session log...", Icon: fonts.IconSave},
		gtk.MenuItem{ID: "reset", Label: "Reset"},
		gtk.MenuItem{ID: "reset-clear", Label: "Reset and clear"},
	)
	if t.kind == "local" || t.hasSFTP {
		label := map[bool]string{true: "Hide SFTP browser", false: "Show SFTP browser"}[t.sftpOpen]
		if t.kind == "local" {
			label = map[bool]string{true: "Hide file browser", false: "Show file browser"}[t.sftpOpen]
		}
		items = append(items, gtk.MenuItem{Separator: true}, gtk.MenuItem{ID: "sftp", Label: label, Icon: fonts.IconBriefcase, Shortcut: "Alt+Ctrl+S"})
	}
	if t.kind == "telnet" {
		items = append(items, gtk.MenuItem{Separator: true}, gtk.MenuItem{ID: "telnet", Label: "Telnet", Icon: fonts.IconPlug, Submenu: []gtk.MenuItem{
			{ID: "telnet:brk", Label: "Send BREAK"},
			{ID: "telnet:ip", Label: "Send Interrupt Process (IP)"},
			{ID: "telnet:ao", Label: "Send Abort Output (AO)"},
			{ID: "telnet:ayt", Label: "Send Are You There (AYT)"},
			{ID: "telnet:ec", Label: "Send Erase Character (EC)"},
			{ID: "telnet:el", Label: "Send Erase Line (EL)"},
		}})
	}
	if _, isSplit := u.splits[id]; isSplit {
		items = append(items, gtk.MenuItem{Separator: true},
			gtk.MenuItem{ID: "unsplit", Label: "Unsplit"},
			gtk.MenuItem{ID: "equal-resize", Label: "Equally resize terminals"})
	} else {
		var vert, horiz []gtk.MenuItem
		n := 0
		for _, o := range u.tabs {
			oid := o.sess.TmpID
			if _, ok := u.splits[oid]; oid == id || ok || secondIsSplit(u.splits, oid) {
				continue
			}
			n++
			label := fmt.Sprintf("%d: %s", n, o.sess.Title())
			vert = append(vert, gtk.MenuItem{ID: "split-v:" + oid, Label: label})
			horiz = append(horiz, gtk.MenuItem{ID: "split-h:" + oid, Label: label})
		}
		if n > 0 {
			items = append(items, gtk.MenuItem{Separator: true}, gtk.MenuItem{ID: "split", Label: "Split", Submenu: []gtk.MenuItem{
				{ID: "split-v", Label: "Vertically", Submenu: vert},
				{ID: "split-h", Label: "Horizontally", Submenu: horiz},
			}})
		}
	}
	items = append(items,
		gtk.MenuItem{Separator: true},
		gtk.MenuItem{ID: "restart", Label: "Restart session"},
		gtk.MenuItem{ID: "disconnect", Label: "Disconnect", Icon: fonts.IconStop},
		gtk.MenuItem{ID: "close", Label: "Close Terminal", Icon: fonts.IconClose, Shortcut: "Ctrl+Shift+W"},
	)
	u.termMenu.Items = items
	u.termMenu.Open(u.lastPtr)
	u.w.Invalidate()
}

func paren(v string) string {
	if v == "" {
		return ""
	}
	return "(" + v + ")"
}

// handleTermMenu applies the item chosen in the terminal menu.
func (u *ui) handleTermMenu(gtx layout.Context) {
	id, ok := u.termMenu.Activated()
	t := u.termMenuTarget
	if !ok || t == nil {
		return
	}
	tmpID := t.sess.TmpID
	var i int
	switch {
	case id == "keybindings":
		u.keys.open = true
	case id == "copy":
		t.view.Copy(gtx)
	case id == "paste":
		t.view.RequestPaste(gtx)
		u.focus = true
	case id == "cluster-new":
		u.clAdmin.Open()
	case strings.HasPrefix(id, "cluster:"):
		u.clusterSvc.Add(tmpID, strings.TrimPrefix(id, "cluster:"))
	case id == "cluster-remove":
		u.clusterSvc.Remove(tmpID)
	case id == "pcc":
		u.pcc.Open()
	case scan(id, "macro:%d", &i):
		if ms := u.macros(t); i < len(ms) {
			// The menu runs macros directly (TerminalContextMenu.svelte).
			u.execMacro(t, ms[i])
		}
	case scan(id, "var:%d", &i):
		if conn, ok := u.core.FindConnection(t.x.connID); ok && i < len(conn.Variables) {
			t.view.Paste(conn.Variables[i].Value)
		}
	case scan(id, "gvar:%d", &i):
		if u.cfg != nil && i < len(u.cfg.Variables) {
			t.view.Paste(u.cfg.Variables[i].Value)
		}
	case id == "find":
		t.x.find.open, t.x.find.focus = true, true
	case id == "screenshot":
		u.takeScreenshot(t)
	case id == "log":
		u.saveSessionLog(t)
	case id == "reset":
		t.view.Engine.Reset()
	case id == "reset-clear":
		t.view.Engine.Reset()
		t.view.Engine.Clear()
	case id == "sftp":
		t.sftpOpen = !t.sftpOpen
	case strings.HasPrefix(id, "telnet:"):
		_ = u.sessionSvc.SendTelnetCommand(tmpID, strings.TrimPrefix(id, "telnet:"))
	case id == "unsplit":
		u.unsplitTab(tmpID)
	case id == "equal-resize":
		u.equalResize(tmpID)
	case strings.HasPrefix(id, "split-v:"):
		u.splitTab(tmpID, strings.TrimPrefix(id, "split-v:"), "horizontal")
	case strings.HasPrefix(id, "split-h:"):
		u.splitTab(tmpID, strings.TrimPrefix(id, "split-h:"), "vertical")
	case id == "restart":
		_ = u.core.Sessions.WriteBytes(tmpID, []byte("\r"))
	case id == "disconnect":
		_ = u.sessionSvc.Disconnect(tmpID)
	case id == "close":
		u.closeTab(tmpID)
	}
	if id != "find" {
		u.focus = true
	}
}

func scan(s, format string, i *int) bool {
	_, err := fmt.Sscanf(s, format, i)
	return err == nil
}

// takeScreenshot saves what the terminal shows (TerminalContextMenu.svelte).
func (u *ui) takeScreenshot(t *tab) {
	img, err := t.view.Screenshot()
	var path string
	if err == nil {
		var buf bytes.Buffer
		if err = png.Encode(&buf, img); err == nil {
			path, err = u.systemSvc.SaveScreenshot(t.x.connID, t.sess.Title(), "data:image/png;base64,"+base64.StdEncoding.EncodeToString(buf.Bytes()))
		}
	}
	if err != nil {
		u.showNotice(t, "Screenshot failed: "+err.Error(), "", true)
		return
	}
	u.showNotice(t, "Screenshot saved: "+path, filepath.Dir(path), false)
}

// saveSessionLog saves the session's output; without a file dialog (stage
// 6) into the session logs folder under the default name.
func (u *ui) saveSessionLog(t *tab) {
	dir := u.systemSvc.SessionLogsDir()
	name := u.systemSvc.DefaultLogName(t.sess.Title())
	path, err := u.systemSvc.ChooseSaveFileIn("Save session log", dir, name)
	if err != nil || path == "" {
		path = filepath.Join(dir, name)
	}
	if err := u.sessionSvc.SaveSessionLog(t.sess.TmpID, path, true); err != nil {
		u.showNotice(t, "Saving the session log failed: "+err.Error(), "", true)
		return
	}
	u.showNotice(t, "Session log saved: "+path, filepath.Dir(path), false)
}
