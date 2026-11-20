package leanedit

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// Icons of the menus and the tool bar (the gtk-* stock icons).
const (
	iconNew     = fonts.IconFileO
	iconOpen    = fonts.IconFolderOpen
	iconSave    = fonts.IconDownload
	iconSaveAs  = fonts.IconSave
	iconRefresh = fonts.IconRefresh
	iconClose   = fonts.IconClose
	iconTrash   = fonts.IconTrash
	iconUndo    = fonts.IconUndo
	iconRedo    = fonts.IconRedo
	iconCut     = fonts.IconCut
	iconCopy    = fonts.IconCopy
	iconPaste   = fonts.IconPaste
	iconFind    = fonts.IconSearch
	iconQuit    = fonts.IconPowerOff
	iconEdit    = fonts.IconEdit
	iconOK      = fonts.IconCheck
)

// uiState holds the widgets.
type uiState struct {
	th          *gtk.Theme
	focusEditor bool

	menuBtns  []widget.Clickable
	menuHover []gtk.Hover
	menuOpen  int // -1 = none
	menu      gtk.Menu
	menuDefs  []topMenu
	popup     gtk.Menu
	popupDefs []item

	tools    [11]widget.Clickable
	tabClick map[int]*widget.Clickable
	tabClose map[int]*widget.Clickable
	tabHover map[int]*gtk.Hover
	tabMid   map[int]*int
	tabsList layout.List

	newB, openB widget.Clickable
	sb          [6]widget.Clickable

	titleW  map[int]int // menu title widths
	barH    int         // menu bar height
	pointer image.Point // last pointer position (popups open there)

	qdlg  gtk.Dialog
	qbtns []widget.Clickable

	keysTag int
}

// item is a menu entry (EditorApp.svelte's MenuItem).
type item struct {
	label, shortcut string
	icon            rune
	disabled        bool
	checked         bool
	separator       bool
	sub             []item
	action          func(gtx layout.Context)
}

type topMenu struct {
	label  string
	items  []item
	action func(gtx layout.Context) // a menu without items (Settings)
}

func sep() item { return item{separator: true} }

func toGtk(items []item, prefix string) []gtk.MenuItem {
	out := make([]gtk.MenuItem, len(items))
	for i, it := range items {
		id := fmt.Sprintf("%s%d", prefix, i)
		out[i] = gtk.MenuItem{ID: id, Label: it.label, Icon: it.icon, Shortcut: it.shortcut, Disabled: it.disabled,
			Separator: it.separator, Checked: it.checked}
		if len(it.sub) > 0 {
			out[i].Submenu = toGtk(it.sub, id+".")
		}
	}
	return out
}

func findItem(items []item, id string) *item {
	var idx int
	rest := id
	for {
		head, tail, more := strings.Cut(rest, ".")
		if _, err := fmt.Sscan(head, &idx); err != nil || idx < 0 || idx >= len(items) {
			return nil
		}
		if !more {
			return &items[idx]
		}
		items, rest = items[idx].sub, tail
	}
}

// Layout draws the window.
func (a *App) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	u := &a.ui_
	u.th = th
	if u.tabClick == nil {
		u.menuOpen = -1
		u.tabClick, u.tabClose, u.tabHover, u.tabMid = map[int]*widget.Clickable{}, map[int]*widget.Clickable{}, map[int]*gtk.Hover{}, map[int]*int{}
	}
	if !a.ready {
		a.ready = true
		a.settings = a.Svc.GetEditorSettings()
		a.TakePending()
	}
	a.drain()
	a.shortcuts(gtx)
	a.handleMenus(gtx)
	pal := th.Pal
	gtk.Fill(gtx, pal.BgColor)
	a.updateUnsaved()
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.menuBar),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !a.toolbar {
				return layout.Dimensions{}
			}
			return a.toolBar(gtx)
		}),
		layout.Rigid(a.tabBar),
		layout.Flexed(1, a.body),
		layout.Rigid(a.statusBar),
	)
	a.trackPointer(gtx)
	// Overlays: the open menu, a popup and a question.
	if u.menuOpen >= 0 {
		u.menu.Layout(gtx, th)
		if !u.menu.Opened() {
			u.menuOpen = -1
		}
	}
	u.popup.Layout(gtx, th)
	if a.quick.open {
		a.quickOpenLayout(gtx, th)
	}
	if a.align.open {
		a.alignLayout(gtx, th)
	}
	if a.question != nil {
		a.questionLayout(gtx, th)
	}
	if u.focusEditor && a.question == nil && !a.quick.open && !a.align.open {
		u.focusEditor = false
		if t := a.Active(); t != nil {
			gtx.Execute(key.FocusCmd{Tag: t.ed})
		}
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// trackPointer follows the pointer over the whole window without taking
// events from anything (popups open where it is).
func (a *App) trackPointer(gtx layout.Context) {
	u := &a.ui_
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &u.pointer, Kinds: pointer.Move | pointer.Press | pointer.Drag})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			u.pointer = e.Position.Round()
		}
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &u.pointer)
}

// ---- menu bar (EditorMenuBar.svelte) ------------------------------------------------------

func (a *App) handleMenus(gtx layout.Context) {
	u := &a.ui_
	if u.menuOpen >= 0 {
		if id, ok := u.menu.Activated(); ok {
			if it := findItem(u.menuDefs[u.menuOpen].items, id); it != nil && it.action != nil && !it.disabled {
				it.action(gtx)
			}
			u.menuOpen = -1
		}
	}
	if id, ok := u.popup.Activated(); ok {
		if it := findItem(u.popupDefs, id); it != nil && it.action != nil && !it.disabled {
			it.action(gtx)
		}
	}
}

func (a *App) menuBar(gtx layout.Context) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	u.menuDefs = a.menus()
	if len(u.menuBtns) != len(u.menuDefs) {
		u.menuBtns = make([]widget.Clickable, len(u.menuDefs))
		u.menuHover = make([]gtk.Hover, len(u.menuDefs))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := layout.Inset{Top: 2, Bottom: 2, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for i := range u.menuDefs {
			i := i
			if i > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Width: 1}.Layout))
			}
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, i)
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, pal.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
	call.Add(gtx.Ops)
	u.barH = d.Size.Y
	return d
}

func (a *App) menuButton(gtx layout.Context, i int) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	def := u.menuDefs[i]
	c := &u.menuBtns[i]
	hovered := u.menuHover[i].Update(gtx)
	open := func() {
		if def.action != nil {
			return
		}
		u.menuOpen = i
		u.menu.Items = toGtk(def.items, "")
		// The menu opens below the button (window coordinates of the bar).
		u.menu.Open(image.Pt(gtx.Dp(4)+menuLeft(u, i, gtx), u.barH))
	}
	if c.Clicked(gtx) {
		switch {
		case def.action != nil:
			u.menuOpen = -1
			def.action(gtx)
		case u.menuOpen == i:
			u.menu.Close()
			u.menuOpen = -1
		default:
			open()
		}
	}
	// Moving over another title while a menu is open switches menus.
	if hovered && u.menuOpen >= 0 && u.menuOpen != i && def.action == nil {
		open()
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, gtk.Text{S: def.label, Color: pal.FgColor}.Widget(th))
		call := m.Stop()
		if hovered || u.menuOpen == i {
			gtk.Box(gtx, d.Size, 3, pal.RowHover, pal.BorderColor)
		}
		u.menuHover[i].Add(gtx)
		call.Add(gtx.Ops)
		u.menuWidths(i, d.Size.X)
		return d
	})
}

// menuWidths remembers the title widths for placing the menus.
func (u *uiState) menuWidths(i, w int) {
	if u.titleW == nil {
		u.titleW = map[int]int{}
	}
	u.titleW[i] = w
}

func menuLeft(u *uiState, i int, gtx layout.Context) int {
	x := 0
	for j := 0; j < i; j++ {
		x += u.titleW[j] + gtx.Dp(1)
	}
	return x
}

// ---- tool bar ---------------------------------------------------------------------------------

func (a *App) toolBar(gtx layout.Context) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	t := a.Active()
	noDoc := t == nil || !t.ready
	type tool struct {
		icon     rune
		disabled bool
		action   func(gtx layout.Context)
	}
	tools := []tool{
		{iconNew, false, func(layout.Context) { a.NewDocument() }},
		{iconOpen, false, func(layout.Context) { a.ChooseAndOpen() }},
		{iconSave, noDoc, func(layout.Context) { a.Save() }},
		{iconSaveAs, noDoc, func(layout.Context) { a.SaveAs() }},
		{0, false, nil},
		{iconUndo, noDoc, func(layout.Context) { a.run(func(t *Tab) { t.ed.Undo() }) }},
		{iconRedo, noDoc, func(layout.Context) { a.run(func(t *Tab) { t.ed.Redo() }) }},
		{0, false, nil},
		{iconCut, noDoc, func(gtx layout.Context) { a.copySelection(gtx, true) }},
		{iconCopy, noDoc, func(gtx layout.Context) { a.copySelection(gtx, false) }},
		{iconPaste, noDoc, func(gtx layout.Context) { a.paste(gtx) }},
		{0, false, nil},
		{iconFind, noDoc, func(gtx layout.Context) { a.openSearch(gtx) }},
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	n := 0
	d := layout.Inset{Top: 3, Bottom: 3, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for _, tl := range tools {
			tl := tl
			if tl.action == nil {
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// .tb-sep: 1px x 18px, margin 0 4px (+ the 2px gaps).
					return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						sz := image.Pt(gtx.Dp(1), gtx.Dp(18))
						gtk.FillRect(gtx, image.Rectangle{Max: sz}, pal.BorderColor)
						return layout.Dimensions{Size: sz}
					})
				}))
				continue
			}
			c := &u.tools[n]
			n++
			if c.Clicked(gtx) && !tl.disabled {
				tl.action(gtx)
			}
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return gtk.Button{Icon: tl.icon, Disabled: tl.disabled}.Layout(gtx, th, c)
				})
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, pal.BgColor)
	gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// ---- tabs -----------------------------------------------------------------------------------

func (a *App) tabBar(gtx layout.Context) layout.Dimensions {
	u := &a.ui_
	if len(a.tabs) == 0 {
		return layout.Dimensions{}
	}
	th := u.th
	pal := th.Pal
	// Clicks first (the list may change).
	for _, t := range append([]*Tab(nil), a.tabs...) {
		id := t.ID
		if c := u.tabClose[id]; c != nil && c.Clicked(gtx) {
			a.flow(func() { a.closeTab(id) })
			continue
		}
		if c := u.tabClick[id]; c != nil && c.Clicked(gtx) {
			a.activate(id)
		}
		// Middle click closes (onauxclick).
		if tag := u.tabMid[id]; tag != nil {
			for {
				ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
				if !ok {
					break
				}
				if e, ok := ev.(pointer.Event); ok && e.Buttons.Contain(pointer.ButtonTertiary) {
					a.flow(func() { a.closeTab(id) })
				}
			}
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	u.tabsList.Axis = layout.Horizontal
	d := u.tabsList.Layout(gtx, len(a.tabs), func(gtx layout.Context, i int) layout.Dimensions {
		return a.tabLayout(gtx, a.tabs[i])
	})
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	h := d.Size.Y + gtx.Dp(1)
	gtk.FillRect(gtx, image.Rect(0, 0, d.Size.X, h), pal.HeaderBg)
	call.Add(gtx.Ops)
	gtk.FillRect(gtx, image.Rect(0, h-gtx.Dp(1), d.Size.X, h), pal.BorderColor)
	return layout.Dimensions{Size: image.Pt(d.Size.X, h)}
}

func (a *App) tabLayout(gtx layout.Context, t *Tab) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	c, x, hv := u.tabClick[t.ID], u.tabClose[t.ID], u.tabHover[t.ID]
	if c == nil {
		c, x, hv = new(widget.Clickable), new(widget.Clickable), new(gtk.Hover)
		u.tabClick[t.ID], u.tabClose[t.ID], u.tabHover[t.ID], u.tabMid[t.ID] = c, x, hv, new(int)
	}
	mid := u.tabMid[t.ID]
	active := t.ID == a.active
	hovered := hv.Update(gtx)
	fg := pal.MutedFg
	if active {
		fg = pal.FgColor
	}
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		// .editor-tab: padding 4px 6px 4px 10px, gap 5px, max-width 240px.
		d := layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(240-16))
			var kids []layout.FlexChild
			gap := layout.Rigid(layout.Spacer{Width: 5}.Layout)
			if t.File != nil && t.File.Kind == "sftp" {
				kids = append(kids, layout.Rigid(gtk.Text{S: "⇄", Size: gtk.Em(0.9), Color: pal.LinkFg}.Widget(th)), gap)
			}
			kids = append(kids, layout.Flexed(1, gtk.Text{S: t.Title, Color: fg, MaxLines: 1}.Widget(th)))
			if t.Dirty {
				kids = append(kids, gap, layout.Rigid(gtk.Text{S: "●", Size: gtk.Em(0.8), Color: pal.WarningFg}.Widget(th)))
			}
			kids = append(kids, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return x.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					m := op.Record(gtx.Ops)
					d := layout.Inset{Left: 3, Right: 3}.Layout(gtx, gtk.Text{S: "×", Size: gtk.Em(1.1), Color: fadeTo(fg, pal.HeaderBg, x.Hovered()), LineHeight: 1}.Widget(th))
					call := m.Stop()
					if x.Hovered() {
						gtk.Box(gtx, d.Size, 3, pal.RowHover, color.NRGBA{})
					}
					call.Add(gtx.Ops)
					return d
				})
			}))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
		call := m.Stop()
		size := image.Pt(d.Size.X+gtx.Dp(1), d.Size.Y)
		switch {
		case active:
			gtk.FillRect(gtx, image.Rectangle{Max: size}, pal.BaseColor)
			gtk.FillRect(gtx, image.Rect(0, size.Y-gtx.Dp(2), size.X-gtx.Dp(1), size.Y), pal.Accent)
		case hovered:
			gtk.FillRect(gtx, image.Rectangle{Max: size}, pal.RowHover)
		}
		gtk.FillRect(gtx, image.Rect(size.X-gtx.Dp(1), 0, size.X, size.Y), pal.BorderColor)
		area := clip.Rect{Max: size}.Push(gtx.Ops)
		hv.Add(gtx)
		event.Op(gtx.Ops, mid)
		area.Pop()
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
}

// fadeTo is the close button's opacity 0.7 (1 when hovered).
func fadeTo(c, bg color.NRGBA, full bool) color.NRGBA {
	if full {
		return c
	}
	mix := func(x, y uint8) uint8 { return uint8(float32(y) + (float32(x)-float32(y))*0.7) }
	return color.NRGBA{R: mix(c.R, bg.R), G: mix(c.G, bg.G), B: mix(c.B, bg.B), A: 0xff}
}

// ---- body ------------------------------------------------------------------------------------

func (a *App) look() look {
	ed := a.settings.Editor
	size := float32(ed.Font.Size)
	if size <= 0 {
		size = 11
	}
	size = max(6, min(40, size+a.zoomPt))
	tw := ed.TabWidth
	if tw <= 0 {
		tw = 4
	}
	return look{family: ed.Font.Family, size: size, tabWidth: tw, softTab: !ed.IndentWithTabs, wrap: a.wrap(), spaces: a.whitespace(), pal: a.ui_.th.Pal}
}

func (a *App) wrap() bool {
	if a.wrapOverride != nil {
		return *a.wrapOverride
	}
	return a.settings.Editor.WordWrap
}

func (a *App) body(gtx layout.Context) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	gtk.Fill(gtx, pal.BaseColor)
	t := a.Active()
	switch {
	case t != nil && t.Loading != "":
		return a.placeholder(gtx, gtk.Text{S: t.Loading, Color: pal.MutedFg}.Widget(th))
	case t != nil && t.ready:
		l := a.look()
		family := l.family
		typeface := font.Typeface(fonts.Monospace)
		if family != "" && !strings.EqualFold(family, "monospace") {
			typeface = font.Typeface(family + "," + string(fonts.Monospace))
		}
		var sh *text.Shaper
		if a.Shaper != nil {
			sh = a.Shaper(family)
		}
		if sh == nil {
			sh = th.Shaper
		}
		t.apply(l, typeface)
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !a.search.open {
					return layout.Dimensions{}
				}
				return a.searchPanelLayout(gtx, th, t)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !a.gotoLn.open {
					return layout.Dimensions{}
				}
				return a.gotoPanelLayout(gtx, th, t)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if t.layout(gtx, sh) {
					t.dirtyAt = gtx.Now
					a.recordEdit(t)
					a.markDirty(t)
				}
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
		)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	case len(a.tabs) == 0 && a.ready:
		if u.newB.Clicked(gtx) {
			a.NewDocument()
		}
		if u.openB.Clicked(gtx) {
			a.ChooseAndOpen()
		}
		return a.placeholder(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(gtk.Text{S: "No document open.", Color: pal.MutedFg}.Widget(th)),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Button{Label: "New", Icon: iconNew}.Layout(gtx, th, &u.newB)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Button{Label: "Open…", Icon: iconOpen}.Layout(gtx, th, &u.openB)
						}),
					)
				}),
			)
		})
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (a *App) placeholder(gtx layout.Context, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, w)
}

// markDirty recomputes a tab's dirty state after an edit (the web GUI
// debounces it; comparing strings is fast enough here).
func (a *App) markDirty(t *Tab) {
	t.Dirty = t.computeDirty()
	a.updateUnsaved()
}

// ---- status bar (EditorStatusBar.svelte) ------------------------------------------------------

func (a *App) statusBar(gtx layout.Context) layout.Dimensions {
	u := &a.ui_
	th := u.th
	pal := th.Pal
	t := a.Active()
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	small := gtk.Em(0.9)
	itemText := func(s string, c color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 2, Bottom: 2, Left: 8, Right: 8}.Layout(gtx, gtk.Text{S: s, Size: small, Color: c, MaxLines: 1}.Widget(th))
		}
	}
	button := func(i int, s string, c color.NRGBA, onClick func(gtx layout.Context)) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			b := &u.sb[i]
			if b.Clicked(gtx) && onClick != nil {
				onClick(gtx)
			}
			return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				m := op.Record(gtx.Ops)
				d := itemText(s, c)(gtx)
				call := m.Stop()
				if b.Hovered() {
					gtk.Box(gtx, d.Size, 3, pal.RowHover, pal.BorderColor)
				}
				call.Add(gtx.Ops)
				return d
			})
		}
	}
	m := op.Record(gtx.Ops)
	var d layout.Dimensions
	if t == nil || !t.ready {
		d = layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(23)
			return layout.W.Layout(gtx, gtk.Text{S: a.status, Size: small, Color: pal.MutedFg, MaxLines: 1}.Widget(th))
		})
	} else {
		line, col := t.ed.CaretPos()
		pos := fmt.Sprintf("Line %d, Column %d", line+1, col+1)
		if n := t.ed.SelectionLen(); n > 0 {
			pos += fmt.Sprintf(" (%d selected)", n)
		}
		indent := fmt.Sprintf("Soft Tabs: %d", a.look().tabWidth)
		if a.settings.Editor.IndentWithTabs {
			indent = fmt.Sprintf("Tabs: %d", a.look().tabWidth)
		}
		eol := "Unix (LF)"
		if t.LineEnding == "crlf" {
			eol = "Windows (CRLF)"
		}
		eolColor := pal.FgColor
		if t.Mixed {
			eolColor = pal.WarningFg
		}
		lang := languageName(t.LanguageID)
		if !t.Highlight {
			lang += " (off)"
		}
		source := ""
		if t.File != nil && t.File.Kind == "sftp" {
			source = t.File.Host
			if source == "" {
				source = "remote"
			}
		}
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := itemText(pos, pal.FgColor)(gtx)
				d.Size.X = max(d.Size.X, gtx.Dp(150))
				return d
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				d := layout.Inset{Left: 8, Right: 8}.Layout(gtx, gtk.Text{S: a.status, Size: small, Color: pal.MutedFg, MaxLines: 1}.Widget(th))
				d.Size.X = gtx.Constraints.Max.X // flex: 1
				return d
			}),
		}
		if source != "" {
			kids = append(kids, layout.Rigid(itemText(source, pal.MutedFg)))
		}
		kids = append(kids,
			layout.Rigid(button(0, a.insLabel(), pal.FgColor, func(layout.Context) { a.toggleOverwrite() })),
			layout.Rigid(button(1, a.blockLabel(), pal.FgColor, func(layout.Context) { a.toggleBlock() })),
			layout.Rigid(itemText(indent, pal.FgColor)),
			layout.Rigid(button(2, strings.ToUpper(t.Encoding), pal.FgColor, func(gtx layout.Context) { a.openPopup(a.encodingItems(false)) })),
			layout.Rigid(button(3, eol, eolColor, func(layout.Context) { a.toggleLineEnding() })),
			layout.Rigid(button(4, lang, pal.FgColor, func(gtx layout.Context) { a.openPopup(a.languageItems()) })),
		)
		d = layout.Inset{Top: 1, Bottom: 1, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(22)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	}
	call := m.Stop()
	d.Size.X = gtx.Constraints.Max.X
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, pal.HeaderBg)
	gtk.FillRect(gtx, image.Rect(0, 0, d.Size.X, gtx.Dp(1)), pal.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// openPopup opens a menu where the pointer is (status bar buttons).
func (a *App) openPopup(items []item) {
	u := &a.ui_
	u.popupDefs = items
	u.popup.Items = toGtk(items, "")
	u.popup.Open(u.pointer)
}

// ---- questions -------------------------------------------------------------------------------

func (a *App) questionLayout(gtx layout.Context, th *gtk.Theme) {
	u := &a.ui_
	q := a.question
	pal := th.Pal
	if len(u.qbtns) != len(q.Choices) {
		u.qbtns = make([]widget.Clickable, len(q.Choices))
	}
	for i, c := range q.Choices {
		if u.qbtns[i].Clicked(gtx) {
			a.answer(c.ID)
			return
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			for _, c := range q.Choices {
				if c.Default {
					a.answer(c.ID)
					return
				}
			}
		}
	}
	if u.qdlg.CloseRequested() {
		a.answer(q.CancelID)
		return
	}
	u.qdlg.Title, u.qdlg.Width, u.qdlg.Closable = q.Title, 480, true
	icon, ic := fonts.IconQuestion, pal.MutedFg
	switch q.Type {
	case "warning", "error":
		icon, ic = fonts.IconWarning, color.NRGBA{R: 0xf5, G: 0xc2, B: 0x11, A: 0xff}
	case "info":
		icon, ic = fonts.IconInfo, pal.Accent
	}
	var acts []layout.Widget
	for i, c := range q.Choices {
		i, c := i, c
		acts = append(acts, func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: c.Label, Icon: c.Icon, Default: c.Default}.Layout(gtx, th, &u.qbtns[i])
		})
	}
	u.qdlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.IconBox(gtx, icon, 40, ic)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: 6}.Layout(gtx, gtk.Text{S: q.Message, Weight: font.Bold, Color: pal.FgColor}.Widget(th))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if q.Detail == "" {
							return layout.Dimensions{}
						}
						return gtk.Text{S: q.Detail, Color: pal.MutedFg}.Layout(gtx, th)
					}),
				)
			}),
		)
	}, acts...)
}

var _ = unit.Dp(0)
