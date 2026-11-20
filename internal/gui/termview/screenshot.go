package termview

import (
	"image"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// Screenshot draws what the terminal shows into an image, in software (Gio
// cannot read its window back): the cell backgrounds, the selection and the
// text in the monospace font at the view's cell size. Characters the font
// lacks are left out.
func (v *View) Screenshot() (image.Image, error) {
	var f term.Frame
	v.Engine.Snapshot(&f)
	cw, ch := max(v.cellW, 1), max(v.cellH, 1)
	img := image.NewNRGBA(image.Rect(0, 0, f.Cols*cw+v.origin.X*2, f.Rows*ch+v.origin.Y*2))
	draw.Draw(img, img.Bounds(), image.NewUniform(f.Bg), image.Point{}, draw.Src)

	ft, err := opentype.Parse(fonts.MonospaceData())
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: float64(ch) / 1.2, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	ascent := face.Metrics().Ascent.Ceil()
	base := (ch-face.Metrics().Height.Ceil())/2 + ascent

	var from, to Pos
	if v.sel != nil {
		from, to = v.sel.bounds(v.line, v.WordChars, f.Cols)
	}
	top := f.History - f.Offset
	o := v.origin
	for y := 0; y < f.Rows; y++ {
		var sp span
		sel := false
		if v.sel != nil {
			sp, sel = v.sel.rowSpan(top+y, f.Cols, from, to)
		}
		for x, c := range f.Row(y) {
			r := image.Rect(o.X+x*cw, o.Y+y*ch, o.X+(x+1)*cw, o.Y+(y+1)*ch)
			if c.Bg != f.Bg {
				draw.Draw(img, r, image.NewUniform(c.Bg), image.Point{}, draw.Src)
			}
			if sel && x >= sp.Start && x <= sp.End {
				draw.Draw(img, r, image.NewUniform(selectionColor), image.Point{}, draw.Over)
			}
			if c.Text == "" || c.Width == 0 || c.Attrs&term.AttrInvisible != 0 {
				continue
			}
			d := font.Drawer{Dst: img, Src: image.NewUniform(c.Fg), Face: face, Dot: fixed.P(r.Min.X, r.Min.Y+base)}
			d.DrawString(c.Text)
		}
	}
	return img, nil
}
