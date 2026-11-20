package gtk

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Dialog is a modal dialog (Dialog.svelte): a backdrop over the window, the
// dialog box centered with a title bar, the body and an action bar.
type Dialog struct {
	Title string
	// Banner replaces the title bar with the gradient banner (Dialog.svelte's
	// bannerText): light themes gradient left-to-right #007aff to #2fedde,
	// dark themes #1f5fa8 to #1a8f88, 2.2em bold #eee text.
	Banner string
	// Width of the dialog (0 = 500px), at most 96% of the window.
	Width unit.Dp
	// Height fixes the dialog's height (0 = as high as the content), at most
	// 94% of the window; the body takes the room.
	Height unit.Dp
	// Closable allows Escape and the × button to close the dialog.
	Closable bool

	close    widget.Clickable
	closeReq bool
	backdrop int
}

// CloseRequested reports (once) that Escape or × asked to close the dialog.
func (d *Dialog) CloseRequested() bool {
	c := d.closeReq
	d.closeReq = false
	return c
}

// Layout draws the dialog over the whole area; body is the content, actions
// the buttons of the action bar (right aligned, 6px apart).
func (d *Dialog) Layout(gtx layout.Context, th *Theme, body layout.Widget, actions ...layout.Widget) layout.Dimensions {
	p := th.Pal
	if d.Closable {
		if d.close.Clicked(gtx) {
			d.closeReq = true
		}
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				d.closeReq = true
			}
		}
		// Callers read CloseRequested before Layout: ask for that frame.
		if d.closeReq {
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	size := gtx.Constraints.Max
	// The backdrop takes all pointer input for the window below.
	bd := clip.Rect{Max: size}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, p.Backdrop, clip.Rect{Max: size}.Op())
	event.Op(gtx.Ops, &d.backdrop)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &d.backdrop, Kinds: pointer.Press | pointer.Release | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1e6, Max: 1e6}}); !ok {
			break
		}
	}
	bd.Pop()

	w := d.Width
	if w == 0 {
		w = 500
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		px := min(gtx.Dp(w), size.X*96/100)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		gtx.Constraints.Min.Y = 0
		gtx.Constraints.Max.Y = size.Y * 94 / 100
		bodyChild := layout.Rigid
		if d.Height > 0 {
			h := min(gtx.Dp(d.Height), gtx.Constraints.Max.Y)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			bodyChild = func(w layout.Widget) layout.FlexChild { return layout.Flexed(1, w) }
		}
		call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.header(gtx, th) }),
				bodyChild(func(gtx layout.Context) layout.Dimensions {
					// .gtk-dialog-body: padding 12px 16px.
					return layout.Inset{Top: 12, Bottom: 12, Left: 16, Right: 16}.Layout(gtx, body)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return actionBar(gtx, th, actions) }),
			)
		})
		r := image.Rectangle{Max: dims.Size}
		// box-shadow 0 10px 32px, border 1px, radius 6px.
		for i := 1; i <= 6; i++ {
			s := gtx.Dp(unit.Dp(i * 4))
			paint.FillShape(gtx.Ops, faded(p.Shadow, 0.12), clip.UniformRRect(image.Rectangle{Min: image.Pt(-s/2, -s/2+gtx.Dp(10)), Max: dims.Size.Add(image.Pt(s/2, s/2+gtx.Dp(10)))}, gtx.Dp(6)+s/2).Op(gtx.Ops))
		}
		box(gtx, dims.Size, gtx.Dp(6), p.BgColor, p.BorderColor)
		cl := clip.UniformRRect(r, gtx.Dp(6)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		cl.Pop()
		return dims
	})
}

// header picks the banner or the title bar.
func (d *Dialog) header(gtx layout.Context, th *Theme) layout.Dimensions {
	if d.Banner != "" {
		return d.banner(gtx, th)
	}
	return d.titlebar(gtx, th)
}

// banner: the .banner-fill gradient with the banner text (2.2em bold, #eee)
// and the × close button.
func (d *Dialog) banner(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	left, right := c(0x007aff, 0xff), c(0x2fedde, 0xff)
	if p.Name != "light" {
		left, right = c(0x1f5fa8, 0xff), c(0x1a8f88, 0xff)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 10, Bottom: 10, Left: 16, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, FillWidth(func(gtx layout.Context) layout.Dimensions {
					// Long banners wrap (QuickConnectDialog.svelte).
					return Text{S: d.Banner, Size: Em(2.2), Color: c(0xeeeeee, 0xff), Weight: font.Bold, LineHeight: 1}.Layout(gtx, th)
				})),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !d.Closable {
						return layout.Dimensions{}
					}
					return d.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						call, cd := record(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return Text{S: "×", Size: Em(1.4), Color: c(0xeeeeee, 0xff), Weight: font.Bold, LineHeight: 1, MaxLines: 1}.Layout(gtx, th)
							})
						})
						if d.close.Hovered() {
							box(gtx, cd.Size, gtx.Dp(3), faded(c(0x000000, 0x55), 1), c(0, 0))
						}
						call.Add(gtx.Ops)
						pointer.CursorPointer.Add(gtx.Ops)
						return cd
					})
				}),
			)
		})
	})
	// Linear gradient left to right, one vertical strip per pixel column.
	for x := 0; x < dims.Size.X; x++ {
		t := float32(x) / float32(max(dims.Size.X-1, 1))
		FillRect(gtx, image.Rectangle{Min: image.Pt(x, 0), Max: image.Pt(x+1, dims.Size.Y)}, lerp(left, right, t))
	}
	call.Add(gtx.Ops)
	return dims
}

func lerp(a, b color.NRGBA, t float32) color.NRGBA {
	mix := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t + 0.5) }
	return color.NRGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}

// titlebar: 6px 10px padding, header color, a border below, the title in
// 0.95em bold and the × button.
func (d *Dialog) titlebar(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 7, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, FillWidth(func(gtx layout.Context) layout.Dimensions {
					return Text{S: d.Title, Size: Em(0.95), Color: p.FgColor, Weight: font.Bold, MaxLines: 1}.Layout(gtx, th)
				})),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !d.Closable {
						return layout.Dimensions{}
					}
					return d.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						fg := p.FgColor
						call, cd := record(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return Text{S: "×", Size: Em(1.2), Color: fg, LineHeight: 1, MaxLines: 1}.Layout(gtx, th)
							})
						})
						if d.close.Hovered() {
							box(gtx, cd.Size, gtx.Dp(3), p.BtnHoverBg, p.BtnHoverBorder)
						}
						call.Add(gtx.Ops)
						pointer.CursorPointer.Add(gtx.Ops)
						return cd
					})
				}),
			)
		})
	})
	FillRect(gtx, image.Rectangle{Max: dims.Size}, p.HeaderBg)
	call.Add(gtx.Ops)
	o := offset(gtx, 0, dims.Size.Y-gtx.Dp(1))
	HLine(gtx, p.BorderColor)
	o.Pop()
	return dims
}

// actionBar: right-aligned buttons, 6px apart, 8px 12px padding, a border on
// top, header color.
func actionBar(gtx layout.Context, th *Theme, actions []layout.Widget) layout.Dimensions {
	if len(actions) == 0 {
		return layout.Dimensions{}
	}
	p := th.Pal
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 9, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// The flexible spacer must take its space for the buttons to go right.
			kids := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})}
			for i, a := range actions {
				if i > 0 {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: 6}.Layout))
				}
				kids = append(kids, layout.Rigid(a))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
		})
	})
	FillRect(gtx, image.Rectangle{Max: dims.Size}, p.HeaderBg)
	HLine(gtx, p.BorderColor)
	call.Add(gtx.Ops)
	return dims
}

// MessageType chooses a message dialog's icon.
type MessageType uint8

const (
	MessageInfo MessageType = iota
	MessageWarning
	MessageError
	MessageQuestion
)

// MessageButtons are a message dialog's answers.
type MessageButtons uint8

const (
	ButtonsOK MessageButtons = iota
	ButtonsOKCancel
	ButtonsYesNo
	ButtonsYesNoCancel
)

// MessageDialog asks a question or shows a message (MessageDialog.svelte):
// a 32px icon, the bold primary text, the secondary text and the answers,
// the main one in the selection colors.
type MessageDialog struct {
	Type      MessageType
	Title     string
	Primary   string
	Secondary string
	Buttons   MessageButtons

	dlg      Dialog
	btns     [3]widget.Clickable
	response string
}

// Response returns the answer ("ok", "cancel", "yes", "no") once given.
func (m *MessageDialog) Response() (string, bool) {
	r := m.response
	m.response = ""
	return r, r != ""
}

func (m *MessageDialog) icon(th *Theme) (rune, color.NRGBA) {
	switch m.Type {
	case MessageWarning:
		return '\uf071', color.NRGBA{R: 0xf5, G: 0xc2, B: 0x11, A: 0xff} // nf-fa-warning
	case MessageError:
		return '\uf057', color.NRGBA{R: 0xe0, G: 0x1b, B: 0x24, A: 0xff} // nf-fa-times_circle
	case MessageQuestion:
		return '\uf059', th.Pal.MutedFg // nf-fa-question_circle
	}
	return '\uf05a', th.Pal.Accent // nf-fa-info_circle
}

// Layout draws the dialog over the whole area.
func (m *MessageDialog) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	type answer struct {
		label, value string
		primary      bool
	}
	var answers []answer
	switch m.Buttons {
	case ButtonsOK:
		answers = []answer{{"OK", "ok", true}}
	case ButtonsOKCancel:
		answers = []answer{{"Cancel", "cancel", false}, {"OK", "ok", true}}
	case ButtonsYesNo:
		answers = []answer{{"No", "no", false}, {"Yes", "yes", true}}
	case ButtonsYesNoCancel:
		answers = []answer{{"Cancel", "cancel", false}, {"No", "no", false}, {"Yes", "yes", true}}
	}
	for i, a := range answers {
		if m.btns[i].Clicked(gtx) {
			m.response = a.value
		}
	}
	m.dlg.Title, m.dlg.Width, m.dlg.Closable = m.Title, 440, true
	icon, iconColor := m.icon(th)
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 2, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return th.IconBox(gtx, icon, 32, iconColor)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if m.Primary == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return Text{S: m.Primary, Size: Em(0.95 * 1.05), Color: p.FgColor, Weight: font.Bold}.Layout(gtx, th)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return Text{S: m.Secondary, Size: Em(0.95), Color: p.FgColor}.Layout(gtx, th)
						}),
					)
				}),
			)
		})
	}
	var acts []layout.Widget
	for i, a := range answers {
		i, a := i, a
		acts = append(acts, func(gtx layout.Context) layout.Dimensions {
			return Button{Label: a.label, Primary: a.primary}.Layout(gtx, th, &m.btns[i])
		})
	}
	d := m.dlg.Layout(gtx, th, body, acts...)
	if m.dlg.CloseRequested() {
		m.response = "cancel"
	}
	return d
}
