package gtk

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// logo colors of LeanTerm_Icon.svg.
var (
	logoBgStart   = color.NRGBA{R: 0x22, G: 0x11, B: 0x0c, A: 0xff}
	logoBgEnd     = color.NRGBA{R: 0x14, G: 0x0a, B: 0x07, A: 0xff}
	logoSideStart = color.NRGBA{R: 0x3d, G: 0x21, B: 0x17, A: 0xff}
	logoSideEnd   = color.NRGBA{R: 0x24, G: 0x13, B: 0x0d, A: 0xff}
	logoFrame     = color.NRGBA{R: 0x5c, G: 0x38, B: 0x26, A: 0xff}
	logoRowOrange = color.NRGBA{R: 0xd9, G: 0x5d, B: 0x1e, A: 0xff}
	logoRowSand   = color.NRGBA{R: 0xf4, G: 0xa2, B: 0x61, A: 0xff}
	logoRowGold   = color.NRGBA{R: 0xe9, G: 0xc4, B: 0x6a, A: 0xff}
	logoPrompt    = color.NRGBA{R: 0xe7, G: 0x6f, B: 0x51, A: 0xff}
)

// Logo draws LeanTerm's icon (LeanTerm_Icon.svg) with Gio paths: the espresso
// rounded square with the SFTP sidebar on the left and the SSH prompt ">_" on
// the right. The size is the smaller of the constraints; the logo is centered.
type Logo struct{}

func (Logo) Layout(gtx layout.Context) layout.Dimensions {
	size := min(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	if size <= 0 {
		size = gtx.Dp(64)
	}
	// The SVG's coordinate system (256x256, the rounded square at
	// (16,16)-(240,240) with a 44px radius).
	s := float32(size) / 256
	// Center the square in the constraints.
	dx, dy := (gtx.Constraints.Max.X-size)/2, (gtx.Constraints.Max.Y-size)/2
	o := offset(gtx, dx, dy)
	defer o.Pop()
	pt := func(x, y float32) f32.Point { return f32.Pt(x*s, y*s) }

	outerRect := image.Rect(int(16*s), int(16*s), int(240*s), int(240*s))
	outerR := int(44 * s)
	outer := clip.UniformRRect(outerRect, outerR)
	// Background: a diagonal gradient from the upper left to the lower right.
	{
		stack := outer.Push(gtx.Ops)
		paint.LinearGradientOp{Stop1: pt(16, 16), Color1: logoBgStart, Stop2: pt(240, 240), Color2: logoBgEnd}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		stack.Pop()
	}
	// The sidebar (left of x=94), clipped to the rounded square.
	{
		stack := outer.Push(gtx.Ops)
		side := clip.Rect{Min: image.Pt(int(16*s), int(16*s)), Max: image.Pt(int(94*s), int(240*s))}.Push(gtx.Ops)
		paint.LinearGradientOp{Stop1: pt(0, 16), Color1: logoSideStart, Stop2: pt(0, 240), Color2: logoSideEnd}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		side.Pop()
		stack.Pop()
	}
	// The separator between the sidebar and the terminal area.
	FillRect(gtx, image.Rect(int(92*s), int(16*s), int(96*s), int(240*s)), logoFrame)
	// The SFTP tree rows (pumpkin, sand with .8 opacity, wheat with .85).
	row := func(x, y, w, h, r float32, c color.NRGBA) {
		rrect := clip.UniformRRect(image.Rect(int(x*s), int(y*s), int((x+w)*s), int((y+h)*s)), int(r*s))
		stack := rrect.Push(gtx.Ops)
		paint.ColorOp{Color: c}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		stack.Pop()
	}
	row(36, 64, 38, 11, 5.5, logoRowOrange)
	row(46, 90, 28, 9, 4.5, faded(logoRowSand, 0.8))
	row(46, 114, 28, 9, 4.5, faded(logoRowSand, 0.8))
	row(36, 138, 38, 11, 5.5, logoRowOrange)
	row(46, 164, 22, 9, 4.5, faded(logoRowGold, 0.85))
	// The SSH prompt: ">" in terracotta and "_" in warm gold, round
	// caps and joins.
	{
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(pt(126, 96))
		path.LineTo(pt(162, 128))
		path.LineTo(pt(126, 160))
		paint.FillShape(gtx.Ops, logoPrompt, clip.Stroke{Path: path.End(), Width: 18 * s}.Op())
	}
	{
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(pt(174, 160))
		path.LineTo(pt(206, 160))
		paint.FillShape(gtx.Ops, logoRowSand, clip.Stroke{Path: path.End(), Width: 18 * s}.Op())
	}
	// The frame on top of everything.
	paint.FillShape(gtx.Ops, logoFrame, clip.Stroke{Path: clip.UniformRRect(outerRect, outerR).Path(gtx.Ops), Width: 6 * s}.Op())
	return layout.Dimensions{Size: image.Pt(size, size)}
}
