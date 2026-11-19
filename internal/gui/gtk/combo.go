package gtk

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ComboOption is one entry of a combo box.
type ComboOption struct {
	Value, Label string
}

// Combo is a drop-down list (ComboBoxText.svelte, a <select>): a box with the
// chosen label and a chevron; a click opens the list below it, Up/Down change
// the value while it has the focus.
type Combo struct {
	Value string

	click   widget.Clickable
	items   []widget.Clickable
	open    bool
	changed bool
	hover   Hover
	dismiss int // tag for clicks outside the open list
}

// Changed reports whether the user picked another value since the last call.
func (c *Combo) Changed() bool {
	ch := c.changed
	c.changed = false
	return ch
}

func (c *Combo) pick(v string) {
	if v != c.Value {
		c.Value = v
		c.changed = true
	}
	c.open = false
}

func labelOf(opts []ComboOption, v string) string {
	for _, o := range opts {
		if o.Value == v {
			return o.Label
		}
	}
	return v
}

// Layout handles input and draws the combo box (and its open list on top).
func (c *Combo) Layout(gtx layout.Context, th *Theme, opts []ComboOption, disabled bool) layout.Dimensions {
	p := th.Pal
	if disabled {
		gtx = gtx.Disabled()
		c.open = false
	}
	if len(c.items) != len(opts) {
		c.items = make([]widget.Clickable, len(opts))
	}
	for i := range c.items {
		if c.items[i].Clicked(gtx) {
			c.pick(opts[i].Value)
			gtx.Execute(key.FocusCmd{Tag: &c.click})
		}
	}
	if c.click.Clicked(gtx) {
		c.open = !c.open
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &c.click, Name: key.NameUpArrow},
			key.Filter{Focus: &c.click, Name: key.NameDownArrow},
			key.Filter{Focus: &c.click, Name: key.NameEscape},
			pointer.Filter{Target: &c.dismiss, Kinds: pointer.Press},
		)
		if !ok {
			break
		}
		switch e := ev.(type) {
		case pointer.Event:
			c.open = false // a click outside the list
		case key.Event:
			if e.State != key.Press {
				continue
			}
			if e.Name == key.NameEscape {
				c.open = false
				continue
			}
			i := indexOf(opts, c.Value)
			if e.Name == key.NameUpArrow && i > 0 {
				c.pick(opts[i-1].Value)
			} else if e.Name == key.NameDownArrow && i < len(opts)-1 {
				c.pick(opts[i+1].Value)
			}
		}
	}

	// The box is as wide as the widest option, like a <select>.
	widest := 0
	for _, o := range opts {
		_, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return Text{S: o.Label, Color: p.TextColor}.Layout(gtx, th)
		})
		widest = max(widest, d.Size.X)
	}
	chevron := gtx.Dp(27)
	dims := c.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 7, Right: 1, Top: 5, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				d := Text{S: labelOf(opts, c.Value), Color: p.TextColor, MaxLines: 1}.Layout(gtx, th)
				d.Size.X = widest + chevron
				return d
			})
		})
		fill := p.BaseColor
		if gtx.Focused(&c.click) {
			fill = p.BtnFocusBg
		}
		border := p.BorderColor
		if disabled {
			fill, border = faded(fill, 0.5), faded(border, 0.5)
		}
		box(gtx, d.Size, gtx.Dp(2), fill, border)
		call.Add(gtx.Ops)
		th.chevronDown(gtx, image.Pt(d.Size.X-gtx.Dp(14), d.Size.Y/2), p.TextColor)
		if !disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return d
	})

	if c.open {
		// The list, drawn above everything else.
		m := op.Record(gtx.Ops)
		th.comboList(gtx, c, opts, dims.Size.X, dims.Size.Y)
		op.Defer(gtx.Ops, m.Stop())
	}
	return dims
}

func indexOf(opts []ComboOption, v string) int {
	for i, o := range opts {
		if o.Value == v {
			return i
		}
	}
	return -1
}

// comboList draws the open list below the box.
func (th *Theme) comboList(gtx layout.Context, c *Combo, opts []ComboOption, width, top int) {
	p := th.Pal
	// Clicks anywhere else close the list.
	big := clip.Rect{Min: image.Pt(-1<<20, -1<<20), Max: image.Pt(1<<20, 1<<20)}.Push(gtx.Ops)
	eventArea(gtx, &c.dismiss)
	big.Pop()

	defer offset(gtx, 0, top).Pop()
	gtx.Constraints.Min = image.Pt(width, 0)
	gtx.Constraints.Max.X = max(width, gtx.Dp(120))
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		var rows []layout.FlexChild
		for i, o := range opts {
			i, o := i, o
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.items[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					fg := p.TextColor
					call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: 7, Right: 7, Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return Text{S: o.Label, Color: fg, MaxLines: 1}.Layout(gtx, th)
						})
					})
					if c.items[i].Hovered() || o.Value == c.Value {
						FillRect(gtx, image.Rectangle{Max: d.Size}, p.SelectedBg)
						fg = p.SelectedFg
						call, _ = record(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 7, Right: 7, Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return Text{S: o.Label, Color: fg, MaxLines: 1}.Layout(gtx, th)
							})
						})
					}
					call.Add(gtx.Ops)
					pointer.CursorPointer.Add(gtx.Ops)
					return d
				})
			}))
		}
		return layout.Inset{Top: 2, Bottom: 2, Left: 1, Right: 1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
	th.shadow(gtx, d.Size)
	box(gtx, d.Size, gtx.Dp(2), p.MenuBg, p.BorderColor)
	call.Add(gtx.Ops)
}

// chevronDown draws the <select> arrow centered at c.
func (th *Theme) chevronDown(gtx layout.Context, c image.Point, col color.NRGBA) {
	s := float32(gtx.Dp(4))
	x, y := float32(c.X), float32(c.Y)
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(x-s, y-s/2))
	path.LineTo(f32.Pt(x, y+s/2))
	path.LineTo(f32.Pt(x+s, y-s/2))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: path.End(), Width: float32(gtx.Dp(unit.Dp(1.6)))}.Op())
}

// shadow draws a soft drop shadow under a popup of size sz (box-shadow
// 0 2px 8px --gtk-shadow).
func (th *Theme) shadow(gtx layout.Context, sz image.Point) {
	sh := th.Pal.Shadow
	for i := 1; i <= 4; i++ {
		d := gtx.Dp(unit.Dp(i * 2))
		r := image.Rectangle{Min: image.Pt(-d/2, -d/2+gtx.Dp(2)), Max: sz.Add(image.Pt(d/2, d/2+gtx.Dp(2)))}
		paint.FillShape(gtx.Ops, faded(sh, 0.18), clip.UniformRRect(r, gtx.Dp(3)+d/2).Op(gtx.Ops))
	}
}
