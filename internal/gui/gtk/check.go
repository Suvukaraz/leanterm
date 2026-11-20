package gtk

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Native form controls as the web view draws them (accent-color, dark/light
// color-scheme): 13px boxes and circles.
func (th *Theme) controlColors(disabled bool) (fill, border color.NRGBA) {
	if th.Pal.Name == "light" {
		if disabled {
			return color.NRGBA{R: 0xf6, G: 0xf6, B: 0xf6, A: 0xff}, color.NRGBA{R: 0xc4, G: 0xc4, B: 0xc4, A: 0xff}
		}
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, color.NRGBA{R: 0x76, G: 0x76, B: 0x76, A: 0xff}
	}
	if disabled {
		return color.NRGBA{R: 0x37, G: 0x37, B: 0x37, A: 0xff}, color.NRGBA{R: 0x57, G: 0x57, B: 0x57, A: 0xff}
	}
	return color.NRGBA{R: 0x3b, G: 0x3b, B: 0x3b, A: 0xff}, color.NRGBA{R: 0x85, G: 0x85, B: 0x85, A: 0xff}
}

// checkbox draws the 13px box; CSS gives it a margin of 2px 4px 2px 0.
func (th *Theme) checkbox(gtx layout.Context, checked, disabled bool) layout.Dimensions {
	sz := gtx.Dp(13)
	fill, border := th.controlColors(disabled)
	if checked {
		fill, border = th.Pal.Accent, th.Pal.Accent
		if disabled {
			fill, border = faded(fill, 0.5), faded(border, 0.5)
		}
	}
	defer offset(gtx, 0, gtx.Dp(2)).Pop()
	box(gtx, image.Pt(sz, sz), gtx.Dp(2), fill, border)
	if checked {
		// The check mark: two strokes, white.
		s := float32(sz)
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Pt(s*0.22, s*0.52))
		p.LineTo(f32.Pt(s*0.42, s*0.72))
		p.LineTo(f32.Pt(s*0.78, s*0.30))
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255},
			clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(2))}.Op())
	}
	return layout.Dimensions{Size: image.Pt(sz+gtx.Dp(4), sz+gtx.Dp(4))}
}

// radio draws the 13px circle.
func (th *Theme) radio(gtx layout.Context, on, disabled bool) layout.Dimensions {
	sz := gtx.Dp(13)
	fill, border := th.controlColors(disabled)
	ring := border
	if on {
		ring = th.Pal.Accent
	}
	defer offset(gtx, 0, gtx.Dp(2)).Pop()
	circle := func(inset float32, c color.NRGBA) {
		r := image.Rectangle{Max: image.Pt(sz, sz)}
		d := int(inset * float32(gtx.Dp(1)))
		r = r.Inset(d)
		paint.FillShape(gtx.Ops, c, clip.Ellipse(r).Op(gtx.Ops))
	}
	circle(0, ring)
	circle(1, fill)
	if on {
		circle(3, th.Pal.Accent)
	}
	return layout.Dimensions{Size: image.Pt(sz+gtx.Dp(4), sz+gtx.Dp(4))}
}

// CheckButton draws a check box with its label (CheckButton.svelte); b holds
// the state.
func (th *Theme) CheckButton(gtx layout.Context, b *widget.Bool, label string, disabled bool) layout.Dimensions {
	if disabled {
		gtx = gtx.Disabled()
	}
	// CSS: margin 2px 0.
	return layout.Inset{Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return th.checkButton(gtx, b, label, disabled)
	})
}

func (th *Theme) checkButton(gtx layout.Context, b *widget.Bool, label string, disabled bool) layout.Dimensions {
	return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if !disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.checkbox(gtx, b.Value, disabled) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// The box's margin-right (4px) is the gap to the label; a label
				// wraps when the room is short (a grid column).
				return Text{S: label, Color: th.Pal.FgColor, LineHeight: 1.2}.Layout(gtx, th)
			}),
		)
	})
}

// RadioOption is one choice of a radio group.
type RadioOption struct {
	Value, Label string
	Disabled     bool
}

// RadioGroup draws radio buttons for e (RadioGroup.svelte): horizontal with
// 8px between the options, or vertical with 4px.
func (th *Theme) RadioGroup(gtx layout.Context, e *widget.Enum, opts []RadioOption, vertical, disabled bool) layout.Dimensions {
	axis, gap := layout.Horizontal, unit.Dp(8)
	if vertical {
		axis, gap = layout.Vertical, 4
	}
	kids := make([]layout.FlexChild, 0, 2*len(opts))
	for i, o := range opts {
		o := o
		if i > 0 {
			kids = append(kids, layout.Rigid(layout.Spacer{Width: gap, Height: gap}.Layout))
		}
		kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			off := disabled || o.Disabled
			if off {
				gtx = gtx.Disabled()
			}
			return e.Layout(gtx, o.Value, func(gtx layout.Context) layout.Dimensions {
				if !off {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.radio(gtx, e.Value == o.Value, off) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Radio labels are spans: the item's 4px gap adds to the
						// circle's margin.
						return layout.Inset{Left: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return Text{S: o.Label, Color: th.Pal.FgColor, LineHeight: 1.2, MaxLines: 1}.Layout(gtx, th)
						})
					}),
				)
			})
		}))
	}
	return layout.Flex{Axis: axis, Alignment: layout.Start}.Layout(gtx, kids...)
}
