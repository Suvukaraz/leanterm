// Package theme holds LeanTerm's GTK/Adwaita-style colors and sizes (the
// tokens of frontend/src/lib/theme/gtk.css) and small widgets drawn with them.
package theme

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// Dark color scheme (the default).
var (
	Bg         = rgb(0x2b2b2b)
	Fg         = rgb(0xe6e6e6)
	Base       = rgb(0x1f1f1f)
	Muted      = rgb(0xa0a0a0)
	Border     = rgb(0x3c3c3c)
	HeaderBg   = rgb(0x333333)
	Accent     = rgb(0x4a90d9)
	TerminalBg = rgb(0x000000)
	Success    = rgb(0x8ae234)
	Warning    = rgb(0xf3df8a)
	Error      = rgb(0xffb4b4)
	Hover      = color.NRGBA{R: 255, G: 255, B: 255, A: 15}
)

// FontSize is the GUI font size (10pt).
const FontSize = unit.Sp(10 * 96.0 / 72)

// Theme is the material theme set up with LeanTerm's fonts and colors.
type Theme struct {
	*material.Theme
}

// New creates the theme with the system fonts.
func New() *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(fonts.Collection()))
	th.Face = fonts.UI
	th.TextSize = FontSize
	th.Palette = material.Palette{Bg: Bg, Fg: Fg, ContrastBg: Accent, ContrastFg: rgb(0xffffff)}
	return &Theme{Theme: th}
}

// Fill paints the constraints' maximum area.
func Fill(gtx layout.Context, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

// HLine draws a 1dp horizontal line across the width.
func HLine(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	h := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, h)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

// Icon draws a Nerd Fonts icon (see fonts.Icon*) in the given size and color.
func Icon(gtx layout.Context, th *Theme, r rune, size unit.Sp, c color.NRGBA) layout.Dimensions {
	l := material.Label(th.Theme, size, string(r))
	l.Font = font.Font{Typeface: fonts.Symbols}
	l.Color = c
	l.MaxLines = 1
	return l.Layout(gtx)
}

// Tab is one tab of a [Notebook].
type Tab struct {
	Title string
	// Icon is a Nerd Fonts icon shown before the title (0 = none); IconColor
	// defaults to the text color.
	Icon      rune
	IconColor color.NRGBA
	click     widget.Clickable
}

// Notebook is a GTK notebook header: a row of tabs, the selected one with
// the window background and an accent line, like LeanTerm's terminal tabs.
type Notebook struct {
	Tabs     []*Tab
	Selected int
}

// Layout draws the tab row and handles clicks.
func (n *Notebook) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	for i, t := range n.Tabs {
		if t.click.Clicked(gtx) {
			n.Selected = i
		}
	}
	height := gtx.Dp(30)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	Fill(gtx, HeaderBg)
	children := make([]layout.FlexChild, len(n.Tabs))
	for i, t := range n.Tabs {
		i, t := i, t
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return n.tab(gtx, th, t, i == n.Selected)
			})
		})
	}
	layout.Flex{}.Layout(gtx, children...)
	// Bottom border of the header.
	b := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, Border, clip.Rect{Min: image.Pt(0, height-b), Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (n *Notebook) tab(gtx layout.Context, th *Theme, t *Tab, selected bool) layout.Dimensions {
	lbl := material.Label(th.Theme, FontSize, t.Title)
	lbl.MaxLines = 1
	lbl.Color = Muted
	if selected {
		lbl.Color = Fg
		lbl.Font.Weight = font.Medium
	}
	macro := layout.Inset{Left: 12, Right: 12, Top: 6, Bottom: 6}
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			switch {
			case selected:
				Fill(gtx, Bg)
				a := gtx.Dp(2)
				paint.FillShape(gtx.Ops, Accent, clip.Rect{Min: image.Pt(0, gtx.Constraints.Min.Y-a), Max: gtx.Constraints.Min}.Op())
			case t.click.Hovered():
				paint.FillShape(gtx.Ops, Hover, clip.Rect{Max: gtx.Constraints.Min}.Op())
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return macro.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if t.Icon == 0 {
							return layout.Dimensions{}
						}
						c := t.IconColor
						if c == (color.NRGBA{}) {
							c = lbl.Color
						}
						return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return Icon(gtx, th, t.Icon, FontSize, c)
						})
					}),
					layout.Rigid(lbl.Layout),
				)
			})
		}),
	)
	return dims
}

// Statusbar draws a GTK status bar: an optional icon and the left text, then
// right-aligned fields separated by spacing.
func Statusbar(gtx layout.Context, th *Theme, icon rune, iconColor color.NRGBA, left string, right []string) layout.Dimensions {
	height := gtx.Dp(24)
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	Fill(gtx, Bg)
	HLine(gtx, Border)
	label := func(s string) layout.Widget {
		l := material.Label(th.Theme, FontSize*0.95, s)
		l.Color = Fg
		l.MaxLines = 1
		return l.Layout
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if icon == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return Icon(gtx, th, icon, FontSize, iconColor)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			return label(left)(gtx)
		}),
	}
	for _, s := range right {
		s := s
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 22}.Layout(gtx, label(s))
		}))
	}
	return layout.Inset{Left: 12, Right: 12, Top: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
