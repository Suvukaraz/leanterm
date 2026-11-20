package gtk

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

// FileChooserButton shows a chosen file or folder (FileChooserButton.svelte):
// an icon and the base name; a click asks the GUI for a native file dialog
// (Clicked), "Clear" empties it.
type FileChooserButton struct {
	Path      string
	Folder    bool
	ShowClear bool

	click, clear widget.Clickable
	cleared      bool
}

// Clicked reports whether the button was clicked: open the file dialog.
func (f *FileChooserButton) Clicked(gtx layout.Context) bool { return f.click.Clicked(gtx) }

// Cleared reports (once) that "Clear" emptied the path.
func (f *FileChooserButton) Cleared() bool {
	c := f.cleared
	f.cleared = false
	return c
}

// Layout draws the button (and "Clear").
func (f *FileChooserButton) Layout(gtx layout.Context, th *Theme, disabled bool) layout.Dimensions {
	p := th.Pal
	if f.clear.Clicked(gtx) && f.Path != "" {
		f.Path, f.cleared = "", true
	}
	if disabled {
		gtx = gtx.Disabled()
	}
	name := "(None)"
	if f.Path != "" {
		name = filepath.Base(f.Path)
	}
	icon := rune(0xf15b) // nf-fa-file
	if f.Folder {
		icon = 0xf07b // nf-fa-folder
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
				// CSS: padding 2px 6px, gap 6px, the name in 0.9em.
				call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 3, Bottom: 3, Left: 7, Right: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.IconBox(gtx, icon, 16, p.FgColor) }),
							layout.Rigid(layout.Spacer{Width: 6}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = 0
								return Text{S: name, Size: Em(0.9), Color: p.FgColor, MaxLines: 1}.Layout(gtx, th)
							}),
						)
					})
				})
				bg, border := p.BtnBg, p.BtnBorder
				if f.click.Hovered() && !disabled {
					bg, border = p.BtnHoverBg, p.BtnHoverBorder
				}
				if disabled {
					bg, border = faded(bg, 0.5), faded(border, 0.5)
				}
				box(gtx, d.Size, gtx.Dp(4), bg, border)
				call.Add(gtx.Ops)
				if !disabled {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				return d
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !f.ShowClear || f.Path == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return Button{Label: "Clear", Disabled: disabled}.Layout(gtx, th, &f.clear)
			})
		}),
	)
}

// ---- Color chooser -------------------------------------------------------

// colorPalette is GTK's color chooser palette: a column per hue, light to
// dark, and the grays.
var colorPalette = [][]uint32{
	{0x99c1f1, 0x62a0ea, 0x3584e4, 0x1c71d8, 0x1a5fb4},
	{0x8ff0a4, 0x57e389, 0x33d17a, 0x2ec27e, 0x26a269},
	{0xf9f06b, 0xf8e45c, 0xf6d32d, 0xf5c211, 0xe5a50a},
	{0xffbe6f, 0xffa348, 0xff7800, 0xe66100, 0xc64600},
	{0xf66151, 0xed333b, 0xe01b24, 0xc01c28, 0xa51d2d},
	{0xdc8add, 0xc061cb, 0x9141ac, 0x813d9c, 0x613583},
	{0xcdab8f, 0xb5835a, 0x986a44, 0x865e3c, 0x63452c},
	{0xffffff, 0xf6f5f4, 0xdeddda, 0xc0bfbc, 0x9a9996},
	{0x77767b, 0x5e5c64, 0x3d3846, 0x241f31, 0x000000},
}

// ColorChooser is the dialog behind a ColorButton: GTK's palette and a hex
// entry for a custom color.
type ColorChooser struct {
	Value color.NRGBA

	dlg        Dialog
	swatches   [9][5]widget.Clickable
	hex        widget.Editor
	ok, cancel widget.Clickable
	shownHex   string
	result     int // 0 none, 1 chosen, 2 cancelled
}

// Open starts the dialog with c.
func (cc *ColorChooser) Open(c color.NRGBA) {
	cc.Value, cc.result = c, 0
	cc.hex.SetText(hexOf(c))
	cc.shownHex = hexOf(c)
}

// Result reports (once) the chosen color, or ok=false for Cancel; done is
// false while the dialog is open.
func (cc *ColorChooser) Result() (c color.NRGBA, ok, done bool) {
	r := cc.result
	cc.result = 0
	return cc.Value, r == 1, r != 0
}

func hexOf(c color.NRGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// ParseHex reads #rrggbb (or rrggbb).
func ParseHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, true
}

// Layout draws the dialog over the whole area.
func (cc *ColorChooser) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	for i := range colorPalette {
		for j := range colorPalette[i] {
			if cc.swatches[i][j].Clicked(gtx) {
				v := colorPalette[i][j]
				cc.Value = color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
				cc.hex.SetText(hexOf(cc.Value))
			}
		}
	}
	for {
		ev, ok := cc.hex.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			if c, ok := ParseHex(cc.hex.Text()); ok {
				cc.Value = c
			}
		case widget.SubmitEvent:
			cc.result = 1
		}
	}
	if cc.ok.Clicked(gtx) {
		cc.result = 1
	}
	cc.dlg.Title, cc.dlg.Width, cc.dlg.Closable = "Select Color", 420, true
	if cc.cancel.Clicked(gtx) || cc.dlg.CloseRequested() {
		cc.result = 2
	}
	body := func(gtx layout.Context) layout.Dimensions {
		var cols []layout.FlexChild
		for i := range colorPalette {
			i := i
			cols = append(cols, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var cells []layout.FlexChild
				for j := range colorPalette[i] {
					j := j
					cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return cc.swatches[i][j].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								v := colorPalette[i][j]
								c := color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
								sz := image.Pt(gtx.Dp(30), gtx.Dp(30))
								border := p.BorderColor
								if c == cc.Value {
									border = p.Accent
									box(gtx, sz, gtx.Dp(4), border, color.NRGBA{})
									o := offset(gtx, gtx.Dp(2), gtx.Dp(2))
									box(gtx, sz.Sub(image.Pt(gtx.Dp(4), gtx.Dp(4))), gtx.Dp(3), c, p.BgColor)
									o.Pop()
								} else {
									box(gtx, sz, gtx.Dp(4), c, border)
								}
								pointer.CursorPointer.Add(gtx.Ops)
								return layout.Dimensions{Size: sz}
							})
						})
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, cells...)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, cols...)
				})
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Label(gtx, "Custom:", p.FgColor) }),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return Entry{Width: 110}.Layout(gtx, th, &cc.hex) }),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sz := image.Pt(gtx.Dp(48), gtx.Dp(24))
						box(gtx, sz, gtx.Dp(3), cc.Value, p.BorderColor)
						return layout.Dimensions{Size: sz}
					}),
				)
			}),
		)
	}
	return cc.dlg.Layout(gtx, th, body,
		func(gtx layout.Context) layout.Dimensions { return Button{Label: "Cancel"}.Layout(gtx, th, &cc.cancel) },
		func(gtx layout.Context) layout.Dimensions {
			return Button{Label: "Select", Default: true}.Layout(gtx, th, &cc.ok)
		},
	)
}

// ---- Fonts ---------------------------------------------------------------

// FontFamily is an installed font family (services.FontFamily).
type FontFamily struct {
	Name      string
	Styles    []string
	Monospace bool
}

// FontSetting is a chosen font.
type FontSetting struct {
	Family   string
	Style    string
	Size     float64
	Features string
}

// Label is how font buttons show the font: "Family Style Size".
func (f FontSetting) Label() string {
	if f.Family == "" {
		return "(Default system font)"
	}
	s := f.Family
	switch strings.ToLower(f.Style) {
	case "", "regular", "book", "normal", "roman":
	default:
		s += " " + f.Style
	}
	if f.Size != 0 {
		s += " " + strconv.FormatFloat(f.Size, 'f', -1, 64)
	}
	return s
}

// previewShaper shapes text in one installed face (font previews).
type previewShaper struct {
	key    string
	shaper *text.Shaper
	face   font.Font
	ok     bool
}

func (ps *previewShaper) get(family, style string) (*text.Shaper, font.Font, bool) {
	key := family + "\x00" + style
	if ps.key != key {
		ps.key = key
		ff, ok := fonts.Face(family, style)
		ps.ok = ok
		if ok {
			ps.face = ff.Font
			ps.shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{ff}))
		}
	}
	return ps.shaper, ps.face, ps.ok
}

// previewLabel draws s in the given installed font (else the GUI font).
func (th *Theme) previewLabel(gtx layout.Context, ps *previewShaper, f FontSetting, size unit.Sp, s string, c color.NRGBA, maxLines int) layout.Dimensions {
	sh, face, ok := ps.get(f.Family, f.Style)
	if !ok {
		// FontButton.svelte falls back to monospace.
		return Text{S: s, Size: size, Color: c, MaxLines: maxLines, Mono: true}.Layout(gtx, th)
	}
	l := material.Label(th.Theme, size, s)
	l.Shaper, l.Font, l.Color, l.MaxLines = sh, face, c, maxLines
	return l.Layout(gtx)
}

// FontButton shows a font in itself (FontButton.svelte); a click opens the
// font chooser (the GUI lays out a FontChooser while it is open).
type FontButton struct {
	click   widget.Clickable
	preview previewShaper
}

// Clicked reports whether the button was clicked.
func (fb *FontButton) Clicked(gtx layout.Context) bool { return fb.click.Clicked(gtx) }

// Layout draws the button with the font's label in that font.
func (fb *FontButton) Layout(gtx layout.Context, th *Theme, f FontSetting, disabled bool) layout.Dimensions {
	p := th.Pal
	if disabled {
		gtx = gtx.Disabled()
	}
	// A parent that stretches the button passes its width as the minimum.
	minW := gtx.Constraints.Min.X - gtx.Dp(2)
	return layout.UniformInset(1).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fb.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4, Left: 9, Right: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.previewLabel(gtx, &fb.preview, f, FontSize, f.Label(), p.FgColor, 1)
				})
			})
			d.Size.X = max(d.Size.X, minW)
			bg, border := p.BtnBg, p.BtnBorder
			if fb.click.Hovered() && !disabled {
				bg, border = p.BtnHoverBg, p.BtnHoverBorder
			}
			box(gtx, d.Size, gtx.Dp(4), bg, border)
			call.Add(gtx.Ops)
			if !disabled {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			return d
		})
	})
}

// FontSizes are the sizes listed by the font chooser.
var FontSizes = []int{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24, 26, 28, 32, 36, 40, 48, 56, 64, 72}

// FontChooser is the "Select Font" dialog (FontChooserDialog.svelte):
// families, styles and sizes side by side, font features, a sample in the
// chosen font and "Show only monospaced fonts".
type FontChooser struct {
	Families []FontFamily

	dlg        Dialog
	families   ListBox
	styles     ListBox
	sizes      ListBox
	size       SpinButton
	features   widget.Editor
	sample     widget.Editor
	onlyMono   widget.Bool
	ok, cancel widget.Clickable
	preview    previewShaper

	shown    []FontFamily
	value    FontSetting
	result   int
	lastMono bool
}

// Open starts the dialog with font f; monospace checks "Show only
// monospaced fonts" (terminal and editor fonts).
func (fc *FontChooser) Open(f FontSetting, monospace bool) {
	fc.value, fc.result = f, 0
	fc.onlyMono.Value, fc.lastMono = monospace, !monospace
	fc.size = SpinButton{Value: f.Size, Min: 6, Max: 72, Step: 1, Width: -1}
	fc.features.SetText(f.Features)
	if fc.sample.Text() == "" {
		fc.sample.SetText("The quick brown fox jumps over the lazy dog 0123456789 {}[]()<>=!")
	}
}

// Result reports (once) the chosen font, or ok=false for Cancel; done is
// false while the dialog is open.
func (fc *FontChooser) Result() (f FontSetting, ok, done bool) {
	r := fc.result
	fc.result = 0
	return fc.value, r == 1, r != 0
}

func (fc *FontChooser) refilter() {
	fc.shown = fc.shown[:0]
	for _, f := range fc.Families {
		if !fc.onlyMono.Value || f.Monospace {
			fc.shown = append(fc.shown, f)
		}
	}
	fc.families.Select(0)
	for i, f := range fc.shown {
		if strings.EqualFold(f.Name, fc.value.Family) {
			fc.families.Select(i)
		}
	}
}

func (fc *FontChooser) styleList() []string {
	if i := fc.families.Selected; i >= 0 && i < len(fc.shown) && len(fc.shown[i].Styles) > 0 {
		return fc.shown[i].Styles
	}
	return []string{"Regular"}
}

// Layout draws the dialog over the whole area.
func (fc *FontChooser) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	if fc.onlyMono.Value != fc.lastMono {
		fc.lastMono = fc.onlyMono.Value
		fc.refilter()
	}
	if fc.families.Changed() && fc.families.Selected < len(fc.shown) {
		fc.value.Family = fc.shown[fc.families.Selected].Name
	}
	styles := fc.styleList()
	si := 0
	for i, s := range styles {
		if strings.EqualFold(s, fc.value.Style) {
			si = i
		}
	}
	if fc.styles.Changed() {
		si = fc.styles.Selected
	}
	if si < len(styles) {
		fc.value.Style = styles[si]
	}
	fc.styles.Selected = si
	if fc.sizes.Changed() {
		fc.size.Value = float64(FontSizes[fc.sizes.Selected])
	}
	fc.size.Changed()
	fc.value.Size = fc.size.Value
	fc.sizes.Selected = -1
	for i, n := range FontSizes {
		if float64(n) == fc.value.Size {
			fc.sizes.Selected = i
		}
	}
	fc.value.Features = fc.features.Text()
	if fc.ok.Clicked(gtx) && fc.value.Family != "" {
		fc.result = 1
	}
	fc.dlg.Title, fc.dlg.Width, fc.dlg.Height, fc.dlg.Closable = "Select Font", 760, 560, true
	if fc.cancel.Clicked(gtx) || fc.dlg.CloseRequested() {
		fc.result = 2
	}

	column := func(label string, w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Text{S: label, Size: Em(0.95), Color: p.FgColor}.Layout(gtx, th)
				}),
				layout.Rigid(layout.Spacer{Height: 4}.Layout),
				layout.Flexed(1, w),
			)
		}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				// Columns 2.4fr : 1fr : 1.4fr with 8px between them.
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(2.4, column("Font:", func(gtx layout.Context) layout.Dimensions {
						return fc.families.Layout(gtx, th, len(fc.shown), func(i int) string { return fc.shown[i].Name })
					})),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1, column("Font style:", func(gtx layout.Context) layout.Dimensions {
						return fc.styles.Layout(gtx, th, len(styles), func(i int) string { return styles[i] })
					})),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1.4, column("Size:", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								fc.size.Width = unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
								return fc.size.Layout(gtx, th, false)
							}),
							layout.Rigid(layout.Spacer{Height: 4}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return fc.sizes.Layout(gtx, th, len(FontSizes), func(i int) string { return strconv.Itoa(FontSizes[i]) })
							}),
						)
					})),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Text{S: "Comma-separated list of font features (e.g. liga or calt):", Size: Em(0.95), Color: p.FgColor}.Layout(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Entry{Placeholder: "liga, calt, -dlig, ss01=2", Width: -1}.Layout(gtx, th, &fc.features)
			}),
			layout.Rigid(layout.Spacer{Height: 12}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Text{S: "Sample:", Size: Em(0.95), Color: p.FgColor}.Layout(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// The sample box: 96px, base color, border, padding 6px 8px.
				sz := image.Pt(gtx.Constraints.Max.X, gtx.Dp(96))
				box(gtx, sz, gtx.Dp(3), p.BaseColor, p.BorderColor)
				g := gtx
				g.Constraints = layout.Exact(sz)
				cl := clipRect(g, sz)
				layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(g, func(gtx layout.Context) layout.Dimensions {
					return th.previewLabel(gtx, &fc.preview, fc.value, unit.Sp(float32(fc.value.Size)*96/72), fc.sample.Text(), p.TextColor, 0)
				})
				cl.Pop()
				return layout.Dimensions{Size: sz}
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.CheckButton(gtx, &fc.onlyMono, "Show only monospaced fonts", false)
			}),
		)
	}
	return fc.dlg.Layout(gtx, th, body,
		func(gtx layout.Context) layout.Dimensions {
			return Button{Label: "OK", Icon: 0xf00c, Default: true, Disabled: fc.value.Family == "" || fc.value.Size < 6 || fc.value.Size > 72}.Layout(gtx, th, &fc.ok)
		},
		func(gtx layout.Context) layout.Dimensions {
			return Button{Label: "Cancel", Icon: 0xf00d}.Layout(gtx, th, &fc.cancel)
		},
	)
}
