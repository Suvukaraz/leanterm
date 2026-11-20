package gtk

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Tab is one tab of a Notebook.
type Tab struct {
	Title string
	// Icon is a Nerd Fonts icon before the title (0 = none).
	Icon      rune
	IconColor color.NRGBA
	// Color is the status color of the title (shown bold), Bg a custom
	// background; zero values use the theme.
	Color, Bg color.NRGBA
	Closable  bool

	click, close widget.Clickable
	hover        Hover
}

// NotebookEvent is what happened to a tab.
type NotebookEvent struct {
	Kind NotebookEventKind
	Tab  int
	// From and To are the indices of a TabMoved event (the final index of
	// the tab that was dragged to another position).
	From, To int
	// Pos is where a context click happened (window coordinates of the tab
	// row's origin plus the offset in the tab).
	Pos image.Point
}

// NotebookEventKind is the kind of a NotebookEvent.
type NotebookEventKind uint8

const (
	TabSelected NotebookEventKind = iota
	TabClosed
	TabMiddleClick
	TabContextClick
	TabDoubleClick
	// TabMoved is emitted when a tab was dragged to another position in
	// the row (only when Notebook.Reorderable).
	TabMoved
)

// Notebook is the header of a GTK notebook (Notebook.svelte): a row of tabs on
// the header color, the selected tab in the window color with an accent line
// on top. With Reorderable the tabs can be dragged to another position in
// the row (the web GUI does not have this; LeanTerm's tabs can be moved).
type Notebook struct {
	Tabs     []*Tab
	Selected int
	// Reorderable enables dragging tabs to another position.
	Reorderable bool

	events  []NotebookEvent
	dragTag int
	drag    tabDrag
	rects   []image.Rectangle // tab row-local rectangles, from the last layout
	sizes   []int             // tab widths of the last layout, in order
	height  int               // measured height of the tab row
}

// tabDrag is the state of an ongoing tab drag.
type tabDrag struct {
	armed  bool
	pid    pointer.ID
	from   int
	startX int
	active bool
	x      int
}

// Events returns and clears what happened since the last call.
func (n *Notebook) Events() []NotebookEvent {
	ev := n.events
	n.events = nil
	return ev
}

// Layout handles input and draws the tab row (27px high).
func (n *Notebook) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	for i, t := range n.Tabs {
		if t.close.Clicked(gtx) {
			n.events = append(n.events, NotebookEvent{Kind: TabClosed, Tab: i})
			continue
		}
		for {
			c, ok := t.click.Update(gtx)
			if !ok {
				break
			}
			if c.NumClicks == 2 {
				n.events = append(n.events, NotebookEvent{Kind: TabDoubleClick, Tab: i})
				continue
			}
			if n.Selected != i {
				n.Selected = i
				n.events = append(n.events, NotebookEvent{Kind: TabSelected, Tab: i})
			}
		}
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: t, Kinds: pointer.Press})
			if !ok {
				break
			}
			e, ok := ev.(pointer.Event)
			if !ok {
				continue
			}
			switch {
			case e.Buttons.Contain(pointer.ButtonTertiary):
				n.events = append(n.events, NotebookEvent{Kind: TabMiddleClick, Tab: i})
			case e.Buttons.Contain(pointer.ButtonSecondary):
				n.events = append(n.events, NotebookEvent{Kind: TabContextClick, Tab: i, Pos: e.Position.Round()})
			}
		}
	}
	n.dragInput(gtx)
	// Callers read the events before Layout: ask for the frame that does.
	if len(n.events) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}

	// The drag area covers the tab row and sits before the tabs, so they
	// are its children; while a drag is active all pointer events for it
	// come to this tag.
	h0 := n.height
	if h0 <= 0 {
		h0 = gtx.Dp(27)
	}
	row := clipRect(gtx, image.Pt(gtx.Constraints.Max.X, h0))
	event.Op(gtx.Ops, &n.dragTag)

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	n.sizes = n.sizes[:0]
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for i, t := range n.Tabs {
			i, t := i, t
			kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return n.tab(gtx, th, t, i == n.Selected)
			}))
		}
		return layout.Flex{}.Layout(gtx, kids...)
	})
	h := max(d.Size.Y, gtx.Dp(26)) + gtx.Dp(1)
	n.height = h
	// Tab rectangles (row-local) for the drag handling of the next frame.
	n.rects = n.rects[:0]
	x := 0
	for _, w := range n.sizes {
		n.rects = append(n.rects, image.Rect(x, 0, x+w, h))
		x += w
	}
	FillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, h), p.HeaderBg)
	call.Add(gtx.Ops)
	if n.drag.active {
		n.dragIndicator(gtx, p)
	}
	o := offset(gtx, 0, h-gtx.Dp(1))
	HLine(gtx, p.BorderColor)
	o.Pop()
	row.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

func (n *Notebook) tab(gtx layout.Context, th *Theme, t *Tab, selected bool) layout.Dimensions {
	p := th.Pal
	hovered := t.hover.Update(gtx)
	return t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		fg, weight := p.FgColor, font.Normal
		if selected {
			weight = font.Medium
		}
		if t.Color != (color.NRGBA{}) {
			fg, weight = t.Color, font.Bold
		}
		// CSS: padding 5px 10px, gap 6px, font 0.9em, border-right 1px.
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10, Right: 10, Top: 5, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				var kids []layout.FlexChild
				if t.Icon != 0 {
					ic := t.IconColor
					if ic == (color.NRGBA{}) {
						ic = p.FgColor
					}
					kids = append(kids,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.IconBox(gtx, t.Icon, 16, ic) }),
						layout.Rigid(layout.Spacer{Width: 6}.Layout))
				}
				kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Dp(16)
					return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return Text{S: t.Title, Size: Em(0.9), Color: fg, Weight: weight, MaxLines: 1}.Layout(gtx, th)
					})
				}))
				if t.Closable {
					kids = append(kids,
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.tabClose(gtx, &t.close) }))
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
			})
		})
		size := image.Pt(d.Size.X+gtx.Dp(1), d.Size.Y)
		// The tab's own input area (middle/context clicks, hover) goes below
		// the content, so the close button gets its clicks.
		area := clipRect(gtx, size)
		eventArea(gtx, t)
		t.hover.Add(gtx)
		pointer.CursorPointer.Add(gtx.Ops)
		area.Pop()
		switch {
		case t.Bg != (color.NRGBA{}):
			FillRect(gtx, image.Rectangle{Max: size}, t.Bg)
		case selected:
			FillRect(gtx, image.Rectangle{Max: size}, p.BgColor)
		case hovered:
			FillRect(gtx, image.Rectangle{Max: size}, p.RowHover)
		}
		if selected {
			FillRect(gtx, image.Rect(0, 0, size.X-gtx.Dp(1), gtx.Dp(2)), p.SelectedBg)
		}
		FillRect(gtx, image.Rect(size.X-gtx.Dp(1), 0, size.X, size.Y), p.BorderColor)
		call.Add(gtx.Ops)
		n.sizes = append(n.sizes, size.X)
		return layout.Dimensions{Size: size}
	})
}

// tabClose draws a tab's "×" (1.2em, muted, a round hover background).
func (th *Theme) tabClose(gtx layout.Context, c *widget.Clickable) layout.Dimensions {
	p := th.Pal
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fg := p.MutedFg
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 3, Right: 3, Top: 0, Bottom: 0}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return Text{S: "×", Size: Em(1.2), Color: fg, LineHeight: 0.8, MaxLines: 1}.Layout(gtx, th)
			})
		})
		if c.Hovered() {
			s := max(d.Size.X, d.Size.Y)
			box(gtx, image.Pt(s, s), s/2, p.RowHover, color.NRGBA{})
			fg = p.FgColor
			call, _ = record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 3, Right: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return Text{S: "×", Size: Em(1.2), Color: fg, LineHeight: 0.8, MaxLines: 1}.Layout(gtx, th)
				})
			})
		}
		call.Add(gtx.Ops)
		return d
	})
}

// dragInput arms, tracks and finishes a tab drag.
func (n *Notebook) dragInput(gtx layout.Context) {
	if !n.Reorderable {
		if n.drag.armed || n.drag.active {
			n.drag = tabDrag{}
		}
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &n.dragTag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			if !e.Buttons.Contain(pointer.ButtonPrimary) {
				break
			}
			if i := n.tabAt(int(e.Position.X)); i >= 0 {
				n.drag = tabDrag{armed: true, pid: e.PointerID, from: i, startX: int(e.Position.X)}
			}
		case pointer.Drag:
			if !n.drag.armed || e.PointerID != n.drag.pid {
				break
			}
			x := int(e.Position.X)
			if !n.drag.active {
				dx := x - n.drag.startX
				slop := gtx.Dp(4)
				if dx*dx < slop*slop {
					break
				}
				n.drag.active = true
				gtx.Execute(pointer.GrabCmd{Tag: &n.dragTag, ID: e.PointerID})
			}
			n.drag.x = x
		case pointer.Release:
			if !n.drag.armed || e.PointerID != n.drag.pid {
				break
			}
			if n.drag.active {
				n.moveTab(n.drag.from, n.dropIndex(n.drag.x))
			}
			n.drag = tabDrag{}
		case pointer.Cancel:
			if e.PointerID == n.drag.pid {
				n.drag = tabDrag{}
			}
		}
	}
}

// tabAt maps an x coordinate (tab row local) to a tab index, or -1.
func (n *Notebook) tabAt(x int) int {
	for i, r := range n.rects {
		if x >= r.Min.X && x < r.Max.X {
			return i
		}
	}
	return -1
}

// dropIndex is the index the dragged tab would land on (the tabs around it
// shift); it is also used for the insertion indicator.
func (n *Notebook) dropIndex(x int) int {
	from := n.drag.from
	to := 0
	for i, r := range n.rects {
		if i == from {
			continue
		}
		if x > (r.Min.X+r.Max.X)/2 {
			to++
		}
	}
	if to >= len(n.Tabs) {
		to = len(n.Tabs) - 1
	}
	return to
}

// moveTab moves the tab from to the index to and makes it the selected tab
// of the notebook (the web GUI switches to a newly opened tab).
func (n *Notebook) moveTab(from, to int) {
	if from < 0 || from >= len(n.Tabs) || to < 0 || to >= len(n.Tabs) || from == to {
		return
	}
	t := n.Tabs[from]
	sel := -1
	if n.Selected >= 0 && n.Selected < len(n.Tabs) {
		sel = indexOfTab(n.Tabs, n.Tabs[n.Selected])
	}
	rest := append([]*Tab{}, n.Tabs[:from]...)
	rest = append(rest, n.Tabs[from+1:]...)
	n.Tabs = append(rest[:to], append([]*Tab{t}, rest[to:]...)...)
	if sel == from {
		n.Selected = to
	} else if sel > from && sel <= to {
		n.Selected = sel - 1
	} else if sel < from && sel >= to {
		n.Selected = sel + 1
	}
	n.events = append(n.events, NotebookEvent{Kind: TabMoved, Tab: to, From: from, To: to})
}

func indexOfTab(tabs []*Tab, t *Tab) int {
	for i, x := range tabs {
		if x == t {
			return i
		}
	}
	return -1
}

// dragIndicator draws the insertion line of an ongoing tab drag.
func (n *Notebook) dragIndicator(gtx layout.Context, p *Palette) {
	to := n.dropIndex(n.drag.x)
	x := 0
	count := 0
	for i, r := range n.rects {
		if count == to {
			break
		}
		if i == n.drag.from {
			continue
		}
		count++
		x = r.Max.X
	}
	FillRect(gtx, image.Rect(x-gtx.Dp(1), 0, x+gtx.Dp(2), n.height-gtx.Dp(1)), p.Accent)
}

// Paned splits its space between two widgets with a 6px handle
// (Paned.svelte); dragging the handle moves the split.
// NewPaned returns a Paned with an even split (Pos unset).
func NewPaned() Paned { return Paned{Pos: -1} }

type Paned struct {
	// Vertical stacks the panes (row-resize); else they are side by side.
	Vertical bool
	// Pos is the size of the first pane in pixels. Negative (the zero
	// value no longer means anything) splits the panes evenly.
	Pos int
	// MinFirst and MinSecond keep the panes from shrinking below a size.
	MinFirst, MinSecond unit.Dp
	// Force50 keeps the panes at an even split and ignores the drag
	// (the "Force split tabs to 50%" option of the web GUI).
	Force50 bool

	drag     widgetDrag
	dragging bool
	hover    Hover
}

// Dragging reports whether the handle is being dragged.
func (pn *Paned) Dragging() bool { return pn.dragging }

// Layout draws the panes and the handle.
func (pn *Paned) Layout(gtx layout.Context, th *Theme, first, second layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	total, handle := size.X, gtx.Dp(6)
	if pn.Vertical {
		total = size.Y
	}
	if pn.Force50 {
		pn.Pos = (total - handle) / 2
		pn.dragging = false
	} else {
		if pn.Pos < 0 {
			pn.Pos = (total - handle) / 2
		}
		move, active := pn.drag.update(gtx, pn, pn.Vertical)
		pn.Pos += move
		pn.dragging = active
		pn.Pos = max(gtx.Dp(pn.MinFirst), min(pn.Pos, total-handle-gtx.Dp(pn.MinSecond)))
	}

	rect := func(a, b int) image.Rectangle {
		if pn.Vertical {
			return image.Rect(0, a, size.X, b)
		}
		return image.Rect(a, 0, b, size.Y)
	}
	pane := func(r image.Rectangle, w layout.Widget) {
		o := offset(gtx, r.Min.X, r.Min.Y)
		g := gtx
		g.Constraints = layout.Exact(r.Size())
		cl := clipRect(g, r.Size())
		w(g)
		cl.Pop()
		o.Pop()
	}
	pane(rect(0, pn.Pos), first)
	h := rect(pn.Pos, pn.Pos+handle)
	c := th.Pal.PanedHandle
	if pn.dragging || pn.hover.Update(gtx) {
		c = th.Pal.PanedHandleHover
	}
	FillRect(gtx, h, c)
	o := offset(gtx, h.Min.X, h.Min.Y)
	area := clipRect(gtx, h.Size())
	eventArea(gtx, pn)
	pn.hover.Add(gtx)
	if pn.Vertical {
		pointer.CursorRowResize.Add(gtx.Ops)
	} else {
		pointer.CursorColResize.Add(gtx.Ops)
	}
	area.Pop()
	o.Pop()
	pane(rect(pn.Pos+handle, total), second)
	return layout.Dimensions{Size: size}
}

// LayoutSide draws the tabs as a column (Notebook.svelte position="left"):
// the tabs stacked and as wide as the widest, on the header color with a
// border on the right; the selected tab in the window color with the accent
// line on its left. Only selection is handled (no closing or dragging).
func (n *Notebook) LayoutSide(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	for i, t := range n.Tabs {
		for {
			if _, ok := t.click.Update(gtx); !ok {
				break
			}
			if n.Selected != i {
				n.Selected = i
				n.events = append(n.events, NotebookEvent{Kind: TabSelected, Tab: i})
			}
		}
	}
	if len(n.events) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
	// The column is as wide as the widest tab.
	w := 0
	for _, t := range n.Tabs {
		mgtx := gtx
		mgtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		d := n.sideTab(mgtx, th, t, false, 0)
		m.Stop()
		w = max(w, d.Size.X)
	}
	h := gtx.Constraints.Max.Y
	FillRect(gtx, image.Rect(0, 0, w+gtx.Dp(1), h), p.HeaderBg)
	y := 0
	for i, t := range n.Tabs {
		o := offset(gtx, 0, y)
		d := n.sideTab(gtx, th, t, i == n.Selected, w)
		o.Pop()
		y += d.Size.Y
	}
	FillRect(gtx, image.Rect(w, 0, w+gtx.Dp(1), h), p.BorderColor)
	return layout.Dimensions{Size: image.Pt(w+gtx.Dp(1), h)}
}

func (n *Notebook) sideTab(gtx layout.Context, th *Theme, t *Tab, selected bool, width int) layout.Dimensions {
	p := th.Pal
	hovered := t.hover.Update(gtx)
	weight := font.Normal
	if selected {
		weight = font.Medium
	}
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		if width > 0 {
			// The tab is a stretched button: its content is centered.
			gtx.Constraints.Min.X = width - gtx.Dp(1)
		}
		return layout.Inset{Left: 10, Right: 10, Top: 5, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return n.sideTabContent(gtx, th, t, weight)
			})
		})
	})
	// CSS border-right 1px on every tab.
	size := image.Pt(max(width, d.Size.X+gtx.Dp(1)), d.Size.Y)
	if width == 0 {
		return layout.Dimensions{Size: size}
	}
	area := clipRect(gtx, size)
	// The hover area is a child of the button's, so both get the events.
	t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		t.hover.Add(gtx)
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
	area.Pop()
	switch {
	case selected:
		FillRect(gtx, image.Rectangle{Max: size}, p.BgColor)
		FillRect(gtx, image.Rect(0, 0, gtx.Dp(2), size.Y), p.SelectedBg)
	case hovered:
		FillRect(gtx, image.Rectangle{Max: size}, p.RowHover)
	}
	FillRect(gtx, image.Rect(size.X-gtx.Dp(1), 0, size.X, size.Y), p.BorderColor)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (n *Notebook) sideTabContent(gtx layout.Context, th *Theme, t *Tab, weight font.Weight) layout.Dimensions {
	p := th.Pal
	var kids []layout.FlexChild
	if t.Icon != 0 {
		ic := t.IconColor
		if ic == (color.NRGBA{}) {
			ic = p.FgColor
		}
		kids = append(kids,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.IconBox(gtx, t.Icon, 16, ic) }),
			layout.Rigid(layout.Spacer{Width: 6}.Layout))
	}
	kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(16)
		return layout.W.Layout(gtx, Text{S: t.Title, Size: Em(0.9), Color: p.FgColor, Weight: weight, MaxLines: 1}.Widget(th))
	}))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
}
