package gtk

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
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
)

// Notebook is the header of a GTK notebook (Notebook.svelte): a row of tabs on
// the header color, the selected tab in the window color with an accent line
// on top.
type Notebook struct {
	Tabs     []*Tab
	Selected int

	events []NotebookEvent
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

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
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
	FillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, h), p.HeaderBg)
	call.Add(gtx.Ops)
	o := offset(gtx, 0, h-gtx.Dp(1))
	HLine(gtx, p.BorderColor)
	o.Pop()
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
		area := clipRect(gtx, size)
		eventArea(gtx, t)
		t.hover.Add(gtx)
		pointer.CursorPointer.Add(gtx.Ops)
		area.Pop()
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

// Paned splits its space between two widgets with a 6px handle
// (Paned.svelte); dragging the handle moves the split.
type Paned struct {
	// Vertical stacks the panes (row-resize); else they are side by side.
	Vertical bool
	// Pos is the size of the first pane in pixels (0 = half).
	Pos int
	// MinFirst and MinSecond keep the panes from shrinking below a size.
	MinFirst, MinSecond unit.Dp

	drag     widgetDrag
	dragging bool
	hover    Hover
}

// Layout draws the panes and the handle.
func (pn *Paned) Layout(gtx layout.Context, th *Theme, first, second layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	total, handle := size.X, gtx.Dp(6)
	if pn.Vertical {
		total = size.Y
	}
	if pn.Pos == 0 {
		pn.Pos = (total - handle) / 2
	}
	move, active := pn.drag.update(gtx, pn, pn.Vertical)
	pn.Pos += move
	pn.dragging = active
	pn.Pos = max(gtx.Dp(pn.MinFirst), min(pn.Pos, total-handle-gtx.Dp(pn.MinSecond)))

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
