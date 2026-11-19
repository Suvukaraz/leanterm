// Package gtk draws LeanTerm's GTK/Adwaita-style widgets with Gio. Sizes and
// colors follow the web GUI's GTK components (frontend/src/components/gtk,
// frontend/src/lib/theme/gtk.css), so both GUIs look the same.
package gtk

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

// FontSize is the GUI font size (10pt, --gtk-font-size).
const FontSize = unit.Sp(10 * 96.0 / 72)

// Theme is the GUI's look: the palette of the color scheme, the fonts and
// a material theme for Gio's own widgets (editors, labels).
type Theme struct {
	*material.Theme
	Pal *Palette
}

// New creates the theme for a color scheme ("light", else dark) with the
// system fonts.
func New(scheme string) *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(fonts.Collection()))
	th.Face = fonts.UI
	th.TextSize = FontSize
	t := &Theme{Theme: th}
	t.SetScheme(scheme)
	return t
}

// SetScheme switches the color scheme ("light", else dark).
func (th *Theme) SetScheme(scheme string) {
	th.Pal = &Dark
	if scheme == "light" {
		th.Pal = &Light
	}
	th.Palette = material.Palette{Bg: th.Pal.BgColor, Fg: th.Pal.FgColor, ContrastBg: th.Pal.Accent, ContrastFg: th.Pal.SelectedFg}
}

// Em scales the GUI font size like a CSS em (0.9em, 0.85em, ...).
func Em(f float32) unit.Sp { return FontSize * unit.Sp(f) }

// Fill paints the constraints' maximum area.
func Fill(gtx layout.Context, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

// FillRect paints r.
func FillRect(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

// HLine draws a 1dp horizontal line across the width.
func HLine(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	h := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

// box fills a rounded rectangle of size sz with a 1dp border of color border
// (none when its alpha is 0).
func box(gtx layout.Context, sz image.Point, radius int, fill, border color.NRGBA) {
	r := image.Rectangle{Max: sz}
	if border.A == 0 {
		paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r, radius).Op(gtx.Ops))
		return
	}
	paint.FillShape(gtx.Ops, border, clip.UniformRRect(r, radius).Op(gtx.Ops))
	b := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r.Inset(b), max(0, radius-b)).Op(gtx.Ops))
}

// faded applies CSS opacity to a color.
func faded(c color.NRGBA, opacity float32) color.NRGBA {
	c.A = uint8(float32(c.A)*opacity + 0.5)
	return c
}

// Text is a label: s in size with color c (weight font.Normal = 400).
type Text struct {
	S      string
	Size   unit.Sp
	Color  color.NRGBA
	Weight font.Weight
	Mono   bool
	// LineHeight scales the line like CSS line-height (0 = normal).
	LineHeight float32
	MaxLines   int
}

// Layout draws the text.
func (t Text) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	size := t.Size
	if size == 0 {
		size = FontSize
	}
	// Gio shapes text at whole pixel sizes (10pt = 13.33px becomes 13px,
	// 2.5% narrower than the web view). Shape at k times the size where that
	// is (nearly) whole, and scale the result down: glyphs are vector paths.
	px := float32(size) * gtx.Metric.PxPerSp
	k := exactScale(px)
	if k == 1 {
		return t.layout(gtx, th, size, px)
	}
	g := gtx
	g.Metric.PxPerSp *= k
	g.Metric.PxPerDp *= k
	g.Constraints.Min = scalePt(gtx.Constraints.Min, k)
	g.Constraints.Max = scalePt(gtx.Constraints.Max, k)
	m := op.Record(gtx.Ops)
	d := t.layout(g, th, size, px*k)
	call := m.Stop()
	aff := op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(1/k, 1/k))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	aff.Pop()
	// Widths round up (no clipped glyphs), heights to the nearest pixel like
	// the web view's line boxes.
	h := int(math.Round(float64(float32(d.Size.Y) / k)))
	return layout.Dimensions{
		Size:     image.Pt(int(math.Ceil(float64(float32(d.Size.X)/k))), h),
		Baseline: int(math.Round(float64(float32(d.Baseline) / k))),
	}
}

// exactScale returns the smallest k in 1..6 for which px*k is within 0.02 of
// a whole number (1 if there is none).
func exactScale(px float32) float32 {
	for k := float32(1); k <= 6; k++ {
		v := px * k
		if math.Abs(float64(v-float32(math.Round(float64(v))))) < 0.02 {
			return k
		}
	}
	return 1
}

func scalePt(p image.Point, k float32) image.Point {
	sc := func(v int) int {
		if v >= 1<<24 {
			return v
		}
		return int(float32(v) * k)
	}
	return image.Pt(sc(p.X), sc(p.Y))
}

func (t Text) layout(gtx layout.Context, th *Theme, size unit.Sp, px float32) layout.Dimensions {
	l := material.Label(th.Theme, size, t.S)
	l.Color = t.Color
	l.Font.Weight = t.Weight
	if t.Mono {
		l.Font.Typeface = fonts.Monospace
	}
	l.MaxLines = t.MaxLines
	if t.LineHeight <= 0 || t.MaxLines != 1 {
		return l.Layout(gtx)
	}
	// CSS line-height: the line box is size*LineHeight high with the text
	// centered in it (Gio sizes a single line by its font's ascent and
	// descent, which is CSS's "normal").
	call, d := record(gtx, l.Layout)
	target := int(px*t.LineHeight + 0.5)
	dy := (target - d.Size.Y) / 2
	o := offset(gtx, 0, dy)
	call.Add(gtx.Ops)
	o.Pop()
	h0 := d.Size.Y
	d.Size.Y = target
	d.Baseline += target - dy - h0 // the baseline counts from the bottom
	return d
}

// Label draws s in the GUI font and the given color.
func (th *Theme) Label(gtx layout.Context, s string, c color.NRGBA) layout.Dimensions {
	return Text{S: s, Color: c}.Layout(gtx, th)
}

// Icon draws a Nerd Fonts icon (see fonts.Icon*) in the given size and color.
func (th *Theme) Icon(gtx layout.Context, r rune, size unit.Sp, c color.NRGBA) layout.Dimensions {
	l := material.Label(th.Theme, size, string(r))
	l.Font = font.Font{Typeface: fonts.Symbols}
	l.Color = c
	l.MaxLines = 1
	return l.Layout(gtx)
}

// IconBox draws icon r centered in a square of side px (like a 16px <img>).
func (th *Theme) IconBox(gtx layout.Context, r rune, side unit.Dp, c color.NRGBA) layout.Dimensions {
	sz := gtx.Dp(side)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(sz, sz))
	return layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return th.Icon(gtx, r, unit.Sp(float32(side)*0.85), c)
	})
}

// record lays w out into a macro and returns its size, to draw a background
// of that size first.
func record(gtx layout.Context, w layout.Widget) (op.CallOp, layout.Dimensions) {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	return m.Stop(), d
}
