package theme

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Widget colors of the dark scheme (gtk.css tokens).
var (
	BtnBg          = rgb(0x3a3a3a)
	BtnBorder      = rgb(0x4a4a4a)
	BtnHoverBg     = rgb(0x474747)
	BtnHoverBorder = rgb(0x8a8a8a)
	BtnActiveBg    = rgb(0x2f2f2f)
	TextColor      = rgb(0xf2f2f2)
	SelectedBg     = rgb(0x2f6db3)
	Backdrop       = color.NRGBA{A: 0x80}
)

// rrect fills a rounded rectangle of size sz with a 1dp border.
func rrect(gtx layout.Context, sz image.Point, radius int, fill, border color.NRGBA) {
	r := image.Rectangle{Max: sz}
	paint.FillShape(gtx.Ops, border, clip.UniformRRect(r, radius).Op(gtx.Ops))
	b := max(1, gtx.Dp(1))
	inner := r.Inset(b)
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(inner, max(0, radius-b)).Op(gtx.Ops))
}

// Button draws a GTK push button (gtk.css "button") around label; the
// default button's label is bold.
func Button(gtx layout.Context, th *Theme, c *widget.Clickable, label string, isDefault bool) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Label(th.Theme, FontSize, label)
		l.Color = Fg
		l.MaxLines = 1
		if isDefault {
			l.Font.Weight = font.Bold
		}
		// Measure the label, then size the button: padding 5/12dp, at least
		// 80dp wide like GTK dialog buttons.
		gtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		ld := l.Layout(gtx)
		call := m.Stop()
		padX, padY := gtx.Dp(12), gtx.Dp(5)
		size := image.Pt(max(ld.Size.X+2*padX, gtx.Dp(80)), ld.Size.Y+2*padY)

		bg, border := BtnBg, BtnBorder
		switch {
		case c.Pressed():
			bg = BtnActiveBg
		case c.Hovered():
			bg, border = BtnHoverBg, BtnHoverBorder
		}
		if gtx.Focused(c) {
			border = Accent
		}
		rrect(gtx, size, gtx.Dp(4), bg, border)
		off := op.Offset(image.Pt((size.X-ld.Size.X)/2, padY)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: size, Baseline: ld.Baseline + padY}
	})
}

// Entry draws a GTK text entry (gtk.css ".gtk-entry"); mask hides secrets.
func Entry(gtx layout.Context, th *Theme, e *widget.Editor, hint string, mask bool) layout.Dimensions {
	e.SingleLine = true
	e.Submit = true // Enter sends a widget.SubmitEvent
	e.Mask = 0
	if mask {
		e.Mask = '•'
	}
	ed := material.Editor(th.Theme, e, hint)
	ed.Color = TextColor
	ed.HintColor = Muted
	ed.SelectionColor = color.NRGBA{R: 0x4a, G: 0x90, B: 0xd9, A: 0x60}
	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: 6, Right: 6, Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return ed.Layout(gtx)
	})
	call := m.Stop()
	border := Border
	if gtx.Focused(e) {
		border = Accent
	}
	rrect(gtx, dims.Size, gtx.Dp(3), Base, border)
	call.Add(gtx.Ops)
	return dims
}

// CheckButton draws a GTK check button with its label.
func CheckButton(gtx layout.Context, th *Theme, b *widget.Bool, label string) layout.Dimensions {
	return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Dp(14)
				fill, border := Base, BtnBorder
				if b.Hovered() {
					border = BtnHoverBorder
				}
				if b.Value {
					fill, border = SelectedBg, SelectedBg
				}
				rrect(gtx, image.Pt(sz, sz), gtx.Dp(2), fill, border)
				if b.Value {
					layout.Center.Layout(layout.Context{Ops: gtx.Ops, Metric: gtx.Metric, Constraints: layout.Exact(image.Pt(sz, sz)), Source: gtx.Source, Now: gtx.Now, Locale: gtx.Locale, Values: gtx.Values}, func(gtx layout.Context) layout.Dimensions {
						return Icon(gtx, th, '', unit.Sp(9), color.NRGBA{R: 255, G: 255, B: 255, A: 255})
					})
				}
				return layout.Dimensions{Size: image.Pt(sz, sz)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th.Theme, FontSize, label)
				l.Color = Fg
				return layout.Inset{Left: 6}.Layout(gtx, l.Layout)
			}),
		)
	})
}

// Label draws text in the GUI font.
func Label(gtx layout.Context, th *Theme, s string, c color.NRGBA) layout.Dimensions {
	l := material.Label(th.Theme, FontSize, s)
	l.Color = c
	return l.Layout(gtx)
}
