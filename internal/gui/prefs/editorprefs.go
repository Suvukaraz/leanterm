package prefs

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// EditorPrefs is LeanEdit's settings window (EditorPrefsApp.svelte): only
// the editor settings, with Cancel, Apply and OK. It fills its own window.
type EditorPrefs struct {
	Load func() services.EditorSettings
	Save func(config.EditorConfig) error
	// Close closes the window (Cancel, OK, Escape).
	Close func()

	d                   Dialog
	loaded              bool
	err                 string
	cancel, applyB, okB widget.Clickable
}

func (p *EditorPrefs) save(close bool) {
	p.err = ""
	if err := p.Save(p.d.s.Editor); err != nil {
		p.err = "The settings could not be saved:\n" + err.Error()
		return
	}
	if close && p.Close != nil {
		p.Close()
	}
}

// Layout draws the window's content.
func (p *EditorPrefs) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	if !p.loaded {
		p.loaded = true
		s := p.Load()
		p.d.s.Editor, p.d.standalone = s.Editor, s.Standalone
	}
	p.d.F.Th = th
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && !p.d.F.PickerOpen() && p.Close != nil {
			p.Close()
		}
	}
	if p.cancel.Clicked(gtx) && p.Close != nil {
		p.Close()
	}
	if p.applyB.Clicked(gtx) {
		p.save(false)
	}
	if p.okB.Clicked(gtx) {
		p.save(true)
	}
	pal := th.Pal
	gtk.Fill(gtx, pal.BgColor)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.d.scroll(gtx, 9, 10, p.d.editorPage(th))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.err == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: 10, Right: 10, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return boxB(gtx, pal.BgColor, pal.ErrorFg, 4, layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8},
					gtk.Text{S: p.err, Size: gtk.Em(0.9), Color: pal.ErrorFg}.Widget(th))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// .prefs-footer: right-aligned, gap 8px, padding 8px 10px, border-top.
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			d := layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Button{Label: "Cancel", Icon: fonts.IconClose}.Layout(gtx, th, &p.cancel)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Button{Label: "Apply", Icon: fonts.IconRefresh}.Layout(gtx, th, &p.applyB)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return gtk.Button{Label: "OK", Icon: fonts.IconSave, Default: true}.Layout(gtx, th, &p.okB)
						}),
					)
				})
			})
			gtk.FillRect(gtx, image.Rect(0, 0, d.Size.X, max(1, gtx.Dp(1))), pal.BorderColor)
			return d
		}),
	)
	p.d.F.Overlays(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// F is the window's form (file dialogs, redraws).
func (p *EditorPrefs) F() *form.Form { return &p.d.F }
