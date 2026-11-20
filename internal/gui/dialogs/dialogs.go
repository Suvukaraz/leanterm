// Package dialogs holds the smaller windows of the main window
// (frontend/src/components/dialogs and cluster): Wake On LAN, Quick
// Connect, Edit Local Variables, Bulk Edit, the cluster administration
// and the Power Cluster Controller. Each is a struct with Open and Layout;
// the GUI connects them to the services through function fields.
package dialogs

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// Sizes of the web dialogs' pixel fonts (the GUI font is 13.33px).
const (
	px12 = 0.9
	px11 = 0.825
)

// mailbox hands results of goroutines (service calls may block) to the UI
// goroutine: work runs in the background, its result function on the next
// frame (drain), so only the UI goroutine touches a dialog's fields.
type mailbox struct{ ch chan func() }

func (m *mailbox) init() {
	if m.ch == nil {
		m.ch = make(chan func(), 8)
	}
}

// run calls work in a goroutine and then apply (with its result) on the UI
// goroutine.
func (m *mailbox) run(invalidate func(), work func() func()) {
	m.init()
	ch := m.ch
	go func() {
		apply := work()
		ch <- apply
		if invalidate != nil {
			invalidate()
		}
	}()
}

// drain applies the results that arrived.
func (m *mailbox) drain() {
	m.init()
	for {
		select {
		case f := <-m.ch:
			f()
		default:
			return
		}
	}
}

// statusBox is the dialogs' .status-box: base color, radius 4px, an accent
// line on the left (3px) and 12px text.
func statusBox(th *gtk.Theme, s string, fg color.NRGBA, accent bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		pal := th.Pal
		left := unit.Dp(8)
		if accent {
			left = 10
		}
		m := op.Record(gtx.Ops)
		d := layout.Inset{Top: 6, Bottom: 6, Left: left + 3, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return gtk.Text{S: s, Size: gtk.Em(px12), Color: fg}.Layout(gtx, th)
		})
		call := m.Stop()
		gtk.Box(gtx, d.Size, 4, pal.BaseColor, color.NRGBA{})
		if accent {
			gtk.FillRect(gtx, image.Rect(0, gtx.Dp(1), gtx.Dp(3), d.Size.Y-gtx.Dp(1)), pal.Accent)
		}
		call.Add(gtx.Ops)
		return d
	}
}

func text(th *gtk.Theme, s string, em float32, c color.NRGBA, w font.Weight) layout.Widget {
	return gtk.Text{S: s, Size: gtk.Em(em), Color: c, Weight: w}.Widget(th)
}

// fixed lays w out exactly w px wide.
func fixed(width unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		px := gtx.Dp(width)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		d := w(gtx)
		d.Size.X = px
		return d
	}
}

func button(th *gtk.Theme, c *widget.Clickable, label string, icon rune, def, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return gtk.Button{Label: label, Icon: icon, Default: def, Disabled: disabled}.Layout(gtx, th, c)
	}
}

func gapCol(gap unit.Dp, children ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for _, c := range children {
			if c == nil {
				continue
			}
			if len(kids) > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Height: gap}.Layout))
			}
			kids = append(kids, layout.Rigid(c))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
}

// grid is a two column grid with a fixed label column.
func grid(labelW, colGap, rowGap unit.Dp, rows ...form.GridRow) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.Widget
		for _, r := range rows {
			r := r
			kids = append(kids, form.FlexRow(colGap, fixed(labelW, r.Label), form.Grow{W: r.Field}))
		}
		return gapCol(rowGap, kids...)(gtx)
	}
}

// clickableRows is a list box of the cluster windows (.list-box): base
// color, border, radius 4px, padding 4px; rows 4px 8px in 12px text, the
// selected one on the accent color.
type clickableRows struct {
	clicks []widget.Clickable
	hover  []gtk.Hover
	list   widget.List
}

func (l *clickableRows) layout(gtx layout.Context, th *gtk.Theme, labels []string, selected int, empty string, minH, maxH unit.Dp, onSelect func(int)) layout.Dimensions {
	pal := th.Pal
	if len(l.clicks) != len(labels) {
		l.clicks = make([]widget.Clickable, len(labels))
		l.hover = make([]gtk.Hover, len(labels))
	}
	for i := range l.clicks {
		if l.clicks[i].Clicked(gtx) {
			onSelect(i)
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = max(0, gtx.Dp(minH)-gtx.Dp(10))
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(maxH)-gtx.Dp(10))
		if len(labels) == 0 {
			return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(12).Layout(gtx, text(th, empty, px11, pal.MutedFg, font.Normal))
			})
		}
		l.list.Axis = layout.Vertical
		d := l.list.Layout(gtx, len(labels), func(gtx layout.Context, i int) layout.Dimensions {
			return l.clicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				hovered := l.hover[i].Update(gtx)
				m := op.Record(gtx.Ops)
				fg := pal.FgColor
				if i == selected {
					fg = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
				}
				d := layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(gtx, text(th, labels[i], px12, fg, font.Normal))
				d.Size.X = gtx.Constraints.Max.X
				call := m.Stop()
				switch {
				case i == selected:
					gtk.Box(gtx, d.Size, 3, pal.Accent, color.NRGBA{})
				case hovered:
					gtk.Box(gtx, d.Size, 3, pal.RowHover, color.NRGBA{})
				}
				l.hover[i].Add(gtx)
				call.Add(gtx.Ops)
				return d
			})
		})
		d.Size.Y = max(d.Size.Y, gtx.Constraints.Min.Y)
		return d
	})
	call := m.Stop()
	gtk.Box(gtx, d.Size, 4, pal.BaseColor, pal.BorderColor)
	call.Add(gtx.Ops)
	return d
}

func invalidateAt(t time.Time) op.InvalidateCmd { return op.InvalidateCmd{At: t} }

// boxed draws w with an inset on a rounded box with a border.
func boxed(gtx layout.Context, bg, border color.NRGBA, radius unit.Dp, in layout.Inset, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := in.Layout(gtx, w)
	call := m.Stop()
	gtk.Box(gtx, d.Size, radius, bg, border)
	call.Add(gtx.Ops)
	return d
}

// fill draws w with an inset on a plain background.
func fill(gtx layout.Context, bg color.NRGBA, in layout.Inset, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := in.Layout(gtx, w)
	call := m.Stop()
	gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, bg)
	call.Add(gtx.Ops)
	return d
}

// wrapped is text that wraps in the width it gets.
func wrapped(th *gtk.Theme, s string, em float32, c color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return gtk.Text{S: s, Size: gtk.Em(em), Color: c}.Layout(gtx, th)
	}
}

// rowsWithLines stacks rows full width with a 1px line between them.
func rowsWithLines(line color.NRGBA, rows ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		y := 0
		for i, r := range rows {
			o := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			g := gtx
			g.Constraints.Min.Y = 0
			d := r(g)
			o.Pop()
			y += d.Size.Y
			if i < len(rows)-1 {
				gtk.FillRect(gtx, image.Rect(0, y, gtx.Constraints.Max.X, y+gtx.Dp(1)), line)
				y += gtx.Dp(1)
			}
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y)}
	}
}

// tableRow lays cells out in columns: fixed widths (dp, weight 0) or shares
// of the rest (weights), with 1px borders between and around them; cells
// are centered vertically.
func tableRow(border color.NRGBA, weights []float32, widths []unit.Dp, cells ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := gtx.Dp(1)
		total := gtx.Constraints.Max.X
		rest, sum := total-(len(cells)+1)*b, float32(0)
		for i := range cells {
			if weights[i] == 0 {
				rest -= gtx.Dp(widths[i])
			} else {
				sum += weights[i]
			}
		}
		ws := make([]int, len(cells))
		for i := range cells {
			if weights[i] == 0 {
				ws[i] = gtx.Dp(widths[i])
			} else {
				ws[i] = int(float32(rest) * weights[i] / sum)
			}
		}
		calls := make([]op.CallOp, len(cells))
		hs := make([]int, len(cells))
		h := 0
		for i, c := range cells {
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(ws[i], 0), Max: image.Pt(ws[i], gtx.Constraints.Max.Y)}
			m := op.Record(gtx.Ops)
			d := c(g)
			calls[i] = m.Stop()
			hs[i] = d.Size.Y
			h = max(h, d.Size.Y)
		}
		x := b
		for i := range cells {
			o := op.Offset(image.Pt(x, b+(h-hs[i])/2)).Push(gtx.Ops)
			calls[i].Add(gtx.Ops)
			o.Pop()
			x += ws[i]
			gtk.FillRect(gtx, image.Rect(x, 0, x+b, h+2*b), border)
			x += b
		}
		gtk.FillRect(gtx, image.Rect(0, 0, b, h+2*b), border)
		gtk.FillRect(gtx, image.Rect(0, 0, total, b), border)
		gtk.FillRect(gtx, image.Rect(0, h+b, total, h+2*b), border)
		return layout.Dimensions{Size: image.Pt(total, h+b)}
	}
}

// table stacks table rows (sharing their borders).
func table(border color.NRGBA, rows ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		y := 0
		for _, r := range rows {
			o := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			d := r(gtx)
			o.Pop()
			y += d.Size.Y
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y+gtx.Dp(1))}
	}
}
