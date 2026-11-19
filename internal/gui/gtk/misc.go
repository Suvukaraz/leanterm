package gtk

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Frame draws a labelled frame around w (Frame.svelte, a <fieldset>): a 1px
// border with 3px corners, the bold label on the top border, 8px 10px padding
// and 6px between the children of w (w lays them out).
func (th *Theme) Frame(gtx layout.Context, label string, w layout.Widget) layout.Dimensions {
	p := th.Pal
	return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lblCall, ld := record(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			if label == "" {
				return layout.Dimensions{}
			}
			return Text{S: label, Size: Em(0.95), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, th)
		})
		lineY := ld.Size.Y / 2
		top := max(ld.Size.Y, gtx.Dp(1))
		pad := layout.Inset{Left: 11, Right: 11, Bottom: 9, Top: 8}
		bodyCall, bd := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return pad.Layout(gtx, w)
		})
		size := image.Pt(max(bd.Size.X, gtx.Constraints.Min.X), top+bd.Size.Y)
		// Border from the label's middle line down; the label covers it.
		frame := image.Rectangle{Min: image.Pt(0, lineY), Max: size}
		b := max(1, gtx.Dp(1))
		paint.FillShape(gtx.Ops, p.BorderColor, clip.UniformRRect(frame, gtx.Dp(3)).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, p.BgColor, clip.UniformRRect(frame.Inset(b), max(0, gtx.Dp(3)-b)).Op(gtx.Ops))
		if label != "" {
			lx := gtx.Dp(11)
			FillRect(gtx, image.Rect(lx, 0, lx+ld.Size.X+gtx.Dp(8), ld.Size.Y), p.BgColor)
			o := offset(gtx, lx+gtx.Dp(4), 0)
			lblCall.Add(gtx.Ops)
			o.Pop()
		}
		// The content starts below the label (pad adds the 8px padding);
		// the web view lets it reach 1px into the label's line.
		o := offset(gtx, 0, max(0, top-gtx.Dp(1)))
		bodyCall.Add(gtx.Ops)
		o.Pop()
		return layout.Dimensions{Size: size}
	})
}

// ProgressBar draws a 100x10px bar filled to frac (0..1) (ProgressBar.svelte).
func (th *Theme) ProgressBar(gtx layout.Context, frac float32, width unit.Dp) layout.Dimensions {
	if width == 0 {
		width = 100
	}
	sz := image.Pt(gtx.Dp(width), gtx.Dp(10))
	r := gtx.Dp(5)
	defer clip.UniformRRect(image.Rectangle{Max: sz}, r).Push(gtx.Ops).Pop()
	FillRect(gtx, image.Rectangle{Max: sz}, th.Pal.BtnBorder)
	frac = float32(math.Max(0, math.Min(1, float64(frac))))
	FillRect(gtx, image.Rect(0, 0, int(frac*float32(sz.X)+0.5), sz.Y), th.Pal.SelectedBg)
	return layout.Dimensions{Size: sz}
}

// Scale draws a slider for f (0..1 of [Min, Max]) and its value in percent
// (Scale.svelte, an <input type=range> of the web view: 129px wide).
func (th *Theme) Scale(gtx layout.Context, f *widget.Float, disabled bool) layout.Dimensions {
	p := th.Pal
	if disabled {
		gtx = gtx.Disabled()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(gtx.Dp(129), gtx.Dp(16))
			gtx.Constraints = layout.Exact(sz)
			f.Update(gtx)
			{
				thumb := gtx.Dp(16)
				x := int(f.Value*float32(sz.X-thumb)) + thumb/2
				track := image.Rect(0, sz.Y/2-gtx.Dp(2), sz.X, sz.Y/2+gtx.Dp(2))
				fill, border := th.controlColors(disabled)
				// Track: the filled part in the accent color.
				paint.FillShape(gtx.Ops, border, clip.UniformRRect(track, gtx.Dp(2)).Op(gtx.Ops))
				paint.FillShape(gtx.Ops, fill, clip.UniformRRect(track.Inset(1), gtx.Dp(2)).Op(gtx.Ops))
				acc := p.Accent
				if disabled {
					acc = faded(acc, 0.5)
				}
				done := track
				done.Max.X = x
				paint.FillShape(gtx.Ops, acc, clip.UniformRRect(done, gtx.Dp(2)).Op(gtx.Ops))
				knob := image.Rect(x-thumb/2, 0, x+thumb/2, thumb)
				paint.FillShape(gtx.Ops, acc, clip.Ellipse(knob).Op(gtx.Ops))
			}
			// The input area: dragging and clicking set the value.
			d := f.Layout(gtx, layout.Horizontal, 8)
			if !disabled {
				area := clipRect(gtx, sz)
				pointer.CursorPointer.Add(gtx.Ops)
				area.Pop()
			}
			return d
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(35)
			return Text{S: fmt.Sprintf("%d%%", int(math.Round(float64(f.Value)*100))), Size: Em(0.85), Color: p.FgColor}.Layout(gtx, th)
		}),
	)
}

// Statusbar draws a status bar (Statusbar.svelte): header color, a border on
// top, 2px 8px padding, at least 22px high, children 8px apart; use
// StatusText for its texts.
func (th *Theme) Statusbar(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	p := th.Pal
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		// min-height 22px includes the border and the padding (Inset does
		// not take them off the minimum).
		gtx.Constraints.Min.Y = gtx.Dp(22 - 3 - 2)
		return layout.Inset{Left: 8, Right: 8, Top: 3, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			var kids []layout.FlexChild
			for i, c := range children {
				if i > 0 {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: 8}.Layout))
				}
				kids = append(kids, c)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	FillRect(gtx, image.Rectangle{Max: d.Size}, p.HeaderBg)
	HLine(gtx, p.BorderColor)
	call.Add(gtx.Ops)
	return d
}

// StatusText is a text of a status bar (0.85em).
func (th *Theme) StatusText(s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return Text{S: s, Size: Em(0.85), Color: th.Pal.FgColor, MaxLines: 1}.Layout(gtx, th)
	}
}

// ColorButton draws a button with a color swatch and its hex value
// (ColorButton.svelte); c reports clicks (the GUI opens a color chooser).
func (th *Theme) ColorButton(gtx layout.Context, c *widget.Clickable, value color.NRGBA, disabled bool) layout.Dimensions {
	p := th.Pal
	if disabled {
		gtx = gtx.Disabled()
	}
	hex := fmt.Sprintf("#%02x%02x%02x", value.R, value.G, value.B)
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			// CSS: padding 2px 6px, gap 6px; swatch 18x14 with a border.
			return layout.Inset{Left: 7, Right: 7, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sz := image.Pt(gtx.Dp(18), gtx.Dp(14))
						box(gtx, sz, gtx.Dp(1), value, p.BorderColor)
						return layout.Dimensions{Size: sz}
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Text{S: hex, Size: Em(0.85), Color: p.FgColor, Mono: true, LineHeight: 1.3, MaxLines: 1}.Layout(gtx, th)
					}),
				)
			})
		})
		bg, border := p.BtnBg, p.BtnBorder
		if c.Hovered() && !disabled {
			bg, border = p.BtnHoverBg, p.BtnHoverBorder
		}
		box(gtx, d.Size, gtx.Dp(4), bg, border)
		call.Add(gtx.Ops)
		if !disabled {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return d
	})
}
