package gtk

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"
)

// ListBox is a scrollable list with one selected row (the lists of the font
// chooser): base color, 1px border with 3px corners, rows with 3px 6px
// padding, the selected row in the selection colors.
type ListBox struct {
	Selected int

	list     layout.List
	clicks   []widget.Clickable
	changed  bool
	tag      int
	scrollTo bool
}

// Changed reports whether the user selected another row since the last call.
func (l *ListBox) Changed() bool {
	c := l.changed
	l.changed = false
	return c
}

// Select selects row i and scrolls it into view.
func (l *ListBox) Select(i int) {
	l.Selected = i
	l.scrollTo = true
}

func (l *ListBox) set(i, n int) {
	i = max(0, min(i, n-1))
	if i != l.Selected {
		l.Selected = i
		l.changed = true
	}
	l.scrollTo = true
}

// Layout draws n rows; row draws the text of row i.
func (l *ListBox) Layout(gtx layout.Context, th *Theme, n int, label func(i int) string) layout.Dimensions {
	p := th.Pal
	if len(l.clicks) < n {
		l.clicks = append(l.clicks, make([]widget.Clickable, n-len(l.clicks))...)
	}
	for i := 0; i < n; i++ {
		if l.clicks[i].Clicked(gtx) {
			l.set(i, n)
			gtx.Execute(key.FocusCmd{Tag: &l.tag})
		}
	}
	page := max(1, gtx.Constraints.Max.Y/max(1, gtx.Dp(22)))
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &l.tag, Name: key.NameUpArrow},
			key.Filter{Focus: &l.tag, Name: key.NameDownArrow},
			key.Filter{Focus: &l.tag, Name: key.NamePageUp},
			key.Filter{Focus: &l.tag, Name: key.NamePageDown},
			key.Filter{Focus: &l.tag, Name: key.NameHome},
			key.Filter{Focus: &l.tag, Name: key.NameEnd},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press || n == 0 {
			continue
		}
		switch e.Name {
		case key.NameUpArrow:
			l.set(l.Selected-1, n)
		case key.NameDownArrow:
			l.set(l.Selected+1, n)
		case key.NamePageUp:
			l.set(l.Selected-page, n)
		case key.NamePageDown:
			l.set(l.Selected+page, n)
		case key.NameHome:
			l.set(0, n)
		case key.NameEnd:
			l.set(n-1, n)
		}
	}
	if l.scrollTo && n > 0 {
		l.scrollTo = false
		first, last := l.list.Position.First, l.list.Position.First+l.list.Position.Count-1
		if l.Selected < first || l.Selected >= last {
			l.list.Position.First = max(0, l.Selected-page/2)
			l.list.Position.Offset = 0
		}
	}

	size := gtx.Constraints.Max
	border := p.BorderColor
	focused := gtx.Focused(&l.tag)
	if focused {
		border = p.Accent
	}
	box(gtx, size, gtx.Dp(3), p.BaseColor, border)
	area := clipRect(gtx, size)
	event.Op(gtx.Ops, &l.tag)
	area.Pop()
	inner := gtx
	b := gtx.Dp(1)
	inner.Constraints = layout.Exact(size.Sub(image.Pt(2*b, 2*b)))
	o := offset(gtx, b, b)
	defer o.Pop()
	cl := clipRect(inner, inner.Constraints.Max)
	defer cl.Pop()
	l.list.Axis = layout.Vertical
	return l.list.Layout(inner, n, func(gtx layout.Context, i int) layout.Dimensions {
		return l.clicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fg := p.TextColor
			call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 3, Bottom: 3, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return Text{S: label(i), Color: fg, MaxLines: 1}.Layout(gtx, th)
				})
			})
			d.Size.X = gtx.Constraints.Max.X
			switch {
			case i == l.Selected:
				bg := p.SelectedBg
				if !focused {
					bg = faded(bg, 0.85)
				}
				FillRect(gtx, image.Rectangle{Max: d.Size}, bg)
				fg = p.SelectedFg
				call, _ = record(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 3, Bottom: 3, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return Text{S: label(i), Color: fg, MaxLines: 1}.Layout(gtx, th)
					})
				})
			case l.clicks[i].Hovered():
				FillRect(gtx, image.Rectangle{Max: d.Size}, p.EditorActiveLine)
			}
			call.Add(gtx.Ops)
			pointer.CursorDefault.Add(gtx.Ops)
			return d
		})
	})
}
