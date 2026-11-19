package gtk

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

// TextArea describes a multiline text area (a <textarea>): it takes all the
// space its parent gives it, scrolls internally and keeps the same look as
// Entry. The info page's notes field uses it.
type TextArea struct {
	Placeholder string
	Disabled    bool
	ReadOnly    bool
	// Mono sets the monospace font (the info page's notes).
	Mono bool
}

// Layout draws the text area for e. The size is the parent's size; the
// caller gives the area its room.
func (ta TextArea) Layout(gtx layout.Context, th *Theme, e *widget.Editor) layout.Dimensions {
	p := th.Pal
	e.SingleLine = false
	e.Submit = false
	e.ReadOnly = ta.ReadOnly || ta.Disabled
	e.Mask = 0
	if ta.Disabled {
		gtx = gtx.Disabled()
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	ed := material.Editor(th.Theme, e, ta.Placeholder)
	if ta.Mono {
		ed.Font.Typeface = fonts.Monospace
	}
	ed.Color, ed.HintColor = p.TextColor, placeholderColor(th)
	ed.SelectionColor = faded(p.SelectedBg, 0.45)
	// CSS: padding 8px, border 1px, radius 3px.
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 8, Right: 8, Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return ed.Layout(gtx)
		})
	})
	border, fill := p.BorderColor, p.BaseColor
	switch {
	case gtx.Focused(e) && !ta.Disabled:
		// textarea:focus colors the border like the inputs.
		b := gtx.Dp(1)
		ring := image.Rectangle{Min: image.Pt(-b, -b), Max: d.Size.Add(image.Pt(b, b))}
		paint.FillShape(gtx.Ops, p.SelectedBg, clip.UniformRRect(ring, gtx.Dp(3)+b).Op(gtx.Ops))
		border = p.FocusRing
	case ta.Disabled:
		fill, border = faded(fill, 0.8), faded(border, 0.8)
	}
	box(gtx, d.Size, gtx.Dp(3), fill, border)
	call.Add(gtx.Ops)
	if !ta.Disabled {
		pointer.CursorText.Add(gtx.Ops)
	}
	return d
}
