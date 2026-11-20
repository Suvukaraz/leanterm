package gtk

import (
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Entry describes a text entry (Entry.svelte, gtk.css inputs).
type Entry struct {
	Placeholder string
	Password    bool
	Disabled    bool
	ReadOnly    bool
	// Width of the entry (0 = the web view's default of 228px; -1 = all
	// the space there is).
	Width unit.Dp
	// Mono sets the text in the monospace font; Size overrides the text
	// size (0 = the GUI font size).
	Mono bool
	Size unit.Sp
}

// Layout draws the entry for e.
func (en Entry) Layout(gtx layout.Context, th *Theme, e *widget.Editor) layout.Dimensions {
	p := th.Pal
	e.SingleLine = true
	e.Submit = true // Enter sends a widget.SubmitEvent
	e.ReadOnly = en.ReadOnly || en.Disabled
	e.Mask = 0
	if en.Password {
		e.Mask = '•'
	}
	if en.Disabled {
		gtx = gtx.Disabled()
	}
	switch {
	case en.Width < 0:
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	default:
		w := en.Width
		if w == 0 {
			w = 228
		}
		px := min(gtx.Dp(w), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
	}
	ed := material.Editor(th.Theme, e, en.Placeholder)
	if en.Mono {
		ed.Font.Typeface = fonts.Monospace
	}
	if en.Size > 0 {
		ed.TextSize = en.Size
	}
	ed.Color, ed.HintColor = p.TextColor, placeholderColor(th)
	ed.SelectionColor = faded(p.SelectedBg, 0.45)
	// CSS: padding 3px 6px, border 1px, radius 3px. Gio's editor line is
	// 1px higher than the web view's, taken off the bottom padding.
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 7, Right: 7, Top: 4, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return ed.Layout(gtx)
		})
	})
	border, fill := p.BorderColor, p.BaseColor
	switch {
	case gtx.Focused(e) && !en.Disabled:
		// input:focus colors the border, .gtk-entry:focus adds a 1px ring
		// (box-shadow) around it.
		b := gtx.Dp(1)
		ring := image.Rectangle{Min: image.Pt(-b, -b), Max: d.Size.Add(image.Pt(b, b))}
		paint.FillShape(gtx.Ops, p.SelectedBg, clip.UniformRRect(ring, gtx.Dp(3)+b).Op(gtx.Ops))
		border = p.FocusRing
	case en.Disabled:
		fill, border = faded(fill, 0.8), faded(border, 0.8)
	}
	box(gtx, d.Size, gtx.Dp(3), fill, border)
	call.Add(gtx.Ops)
	if !en.Disabled {
		pointer.CursorText.Add(gtx.Ops)
	}
	return d
}

// placeholderColor is the web view's placeholder gray.
func placeholderColor(th *Theme) color.NRGBA {
	if th.Pal.Name == "light" {
		return color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xff}
	}
	return color.NRGBA{R: 0x8f, G: 0x8f, B: 0x8f, A: 0xff}
}
