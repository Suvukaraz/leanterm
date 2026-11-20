package form

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// Fonts lists the installed families for font fields (nil: the chooser's
// built-in list).
var Fonts func() []gtk.FontFamily

type pickers struct {
	color       gtk.ColorChooser
	colorFor    *string
	colorOpen   bool
	font        gtk.FontChooser
	fontFor     *config.FontConfig
	fontOpen    bool
	fontsLoaded bool
}

func (f *Form) pick() *pickers {
	return get(f, "pickers", func() *pickers { return &pickers{} })
}

// Color edits a "#rrggbb" value with a color button and the color dialog.
func (f *Form) Color(p *string, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		c := f.Clicked(fmt.Sprintf("color:%p", p))
		val, ok := gtk.ParseHex(*p)
		if !ok {
			val = color.NRGBA{A: 0} // none
		}
		if c.Clicked(gtx) && !f.Disabled && !disabled {
			pk := f.pick()
			start := val
			if !ok {
				start = color.NRGBA{A: 0xff}
			}
			pk.color.Open(start)
			pk.colorFor, pk.colorOpen = p, true
		}
		return f.Th.ColorButton(gtx, c, val, f.Disabled || disabled)
	}
}

// Font edits a font setting with a font button and the font dialog.
func (f *Form) Font(p *config.FontConfig, monospace, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := get(f, p, func() *gtk.FontButton { return &gtk.FontButton{} })
		fs := gtk.FontSetting{Family: p.Family, Style: p.Style, Size: float64(p.Size), Features: p.Features}
		if b.Clicked(gtx) && !f.Disabled && !disabled {
			pk := f.pick()
			if !pk.fontsLoaded && Fonts != nil {
				pk.font.Families = Fonts()
				pk.fontsLoaded = true
			}
			pk.font.Open(fs, monospace)
			pk.fontFor, pk.fontOpen = p, true
		}
		return b.Layout(gtx, f.Th, fs, f.Disabled || disabled)
	}
}

// Scale edits a float between min and max.
func (f *Form) Scale(p *float64, min, max float64, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		v := get(f, p, func() *widget.Float { return &widget.Float{} })
		if v.Update(gtx) && !f.Disabled && !disabled {
			*p = min + float64(v.Value)*(max-min)
		}
		if max > min {
			v.Value = float32((*p - min) / (max - min))
		}
		gtx.Constraints.Max.X = min2(gtx.Constraints.Max.X, gtx.Dp(200))
		return f.Th.Scale(gtx, v, f.Disabled || disabled)
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Overlays draws the open color or font dialog; call it over the dialog.
func (f *Form) Overlays(gtx layout.Context) {
	pk := f.pick()
	if pk.colorOpen {
		pk.color.Layout(gtx, f.Th)
		if c, ok, done := pk.color.Result(); done {
			pk.colorOpen = false
			if ok && pk.colorFor != nil {
				*pk.colorFor = fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
			}
		}
	}
	if pk.fontOpen {
		pk.font.Layout(gtx, f.Th)
		if fs, ok, done := pk.font.Result(); done {
			pk.fontOpen = false
			if ok && pk.fontFor != nil {
				pk.fontFor.Family, pk.fontFor.Style, pk.fontFor.Size, pk.fontFor.Features = fs.Family, fs.Style, int(fs.Size+0.5), fs.Features
			}
		}
	}
}

// PickerOpen reports whether a color or font dialog is shown.
func (f *Form) PickerOpen() bool {
	pk := f.pick()
	return pk.colorOpen || pk.fontOpen
}

// ---- editable lists -------------------------------------------------------------

// RowCtl lets a list row delete or move itself.
type RowCtl struct {
	Delete, Up, Down func()
	First, Last      bool
}

type listState struct {
	history [][]byte
	undoTag int
}

// List is EditableList.svelte: a title with "(Ctrl+Z to undo)", the rows
// in a bordered box (padding 4px 6px, 6px gaps, separated by lines), an
// empty hint and the add button. Rows change *items through RowCtl.
func List[T any](f *Form, items *[]T, title, addLabel string, create func() T, row func(i int, it *T, ctl RowCtl) layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st := get(f, items, func() *listState { return &listState{} })
		pal := f.Th.Pal
		snapshot := func() {
			st.history = append(st.history, encode(*items))
			if len(st.history) > 50 {
				st.history = st.history[1:]
			}
		}
		// Ctrl+Z restores the list before the last add, delete or move.
		for {
			ev, ok := gtx.Event(key.Filter{Name: "Z", Required: key.ModShortcut})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press && len(st.history) > 0 {
				decode(st.history[len(st.history)-1], items)
				st.history = st.history[:len(st.history)-1]
			}
		}
		add := f.Clicked(fmt.Sprintf("add:%p", items))
		if add.Clicked(gtx) && !f.Disabled {
			snapshot()
			*items = append(*items, create())
		}
		var kids []layout.Widget
		if title != "" {
			kids = append(kids, RowGap(8,
				gtk.Text{S: title, Size: gtk.Em(0.95), Weight: font.Bold, Color: pal.FgColor}.Widget(f.Th),
				gtk.Text{S: "(Ctrl+Z to undo)", Size: gtk.Em(0.8), Color: pal.MutedFg}.Widget(f.Th),
			))
		}
		n := len(*items)
		box := func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					if n == 0 {
						return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.N.Layout(gtx, gtk.Text{S: fmt.Sprintf("No entries defined. Click %q below to add one.", addLabel), Size: gtk.Em(0.9), Color: pal.MutedFg}.Widget(f.Th))
						})
					}
					var rows []layout.FlexChild
					for i := 0; i < n; i++ {
						i := i
						ctl := RowCtl{First: i == 0, Last: i == n-1,
							Delete: func() { snapshot(); *items = append((*items)[:i:i], (*items)[i+1:]...) },
							Up: func() {
								if i > 0 {
									snapshot()
									(*items)[i-1], (*items)[i] = (*items)[i], (*items)[i-1]
								}
							},
							Down: func() {
								if i < n-1 {
									snapshot()
									(*items)[i+1], (*items)[i] = (*items)[i], (*items)[i+1]
								}
							},
						}
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if i >= len(*items) {
								return layout.Dimensions{}
							}
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							d := layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, row(i, &(*items)[i], ctl))
							d.Size.X = gtx.Constraints.Max.X
							if i < n-1 {
								gtk.FillRect(gtx, image.Rect(0, d.Size.Y-max(1, gtx.Dp(1)), d.Size.X, d.Size.Y), pal.BorderColor)
							}
							return d
						}))
						if i < n-1 {
							rows = append(rows, layout.Rigid(layout.Spacer{Height: 4}.Layout))
						}
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				})
			})
			d.Size.X = gtx.Constraints.Max.X
			r := image.Rectangle{Max: d.Size}
			gtk.FillRect(gtx, r, pal.BorderColor)
			gtk.FillRect(gtx, r.Inset(max(1, gtx.Dp(1))), pal.BaseColor)
			call.Add(gtx.Ops)
			return d
		}
		kids = append(kids, box, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: addLabel, Icon: fonts.IconPlus, Disabled: f.Disabled}.Layout(gtx, f.Th, add)
			})
		})
		return Col(kids...)(gtx)
	}
}

// DeleteButton is a row's delete button (an icon).
func (f *Form) DeleteButton(key any, onClick func()) layout.Widget {
	return f.IconButton(key, fonts.IconTrash, false, onClick)
}

// IconButton is a button with only an icon.
func (f *Form) IconButton(key any, icon rune, disabled bool, onClick func()) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		c := f.Clicked(fmt.Sprintf("icon:%v:%p", icon, key))
		if c.Clicked(gtx) && !f.Disabled && !disabled && onClick != nil {
			onClick()
		}
		return gtk.Button{Icon: icon, Disabled: f.Disabled || disabled}.Layout(gtx, f.Th, c)
	}
}

// Grow lets w take the remaining width of a row (inside FlexRow).
type Grow struct{ W layout.Widget }

// FlexRow is Row where Grow children take the room left.
func FlexRow(gap unit.Dp, children ...any) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for _, c := range children {
			if c == nil {
				continue
			}
			if len(kids) > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Width: gap}.Layout))
			}
			switch w := c.(type) {
			case Grow:
				kids = append(kids, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return w.W(gtx)
				}))
			case layout.Widget:
				if w != nil {
					kids = append(kids, layout.Rigid(w))
				}
			case func(layout.Context) layout.Dimensions:
				if w != nil {
					kids = append(kids, layout.Rigid(w))
				}
			}
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	}
}
