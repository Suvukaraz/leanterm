// Package dialogs holds the GUI's dialogs that the backend asks for.
package dialogs

import (
	"context"
	"image"
	"strings"
	"sync"

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
	"gioui.org/widget/material"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/session"
)

type pending struct {
	req  session.PromptRequest
	resp chan session.PromptResponse
	// gone is closed when the asking side stopped waiting.
	gone chan struct{}
}

// Prompter shows the backend's questions (passwords, host keys, ...) as
// modal dialogs, one after the other. It implements session.Prompter: Prompt
// blocks its caller until the dialog is answered. Layout runs on the GUI
// goroutine.
type Prompter struct {
	invalidate func()

	mu    sync.Mutex
	queue []*pending

	// Widgets of the dialog on top, reset for each request.
	shown      *pending
	values     []widget.Editor
	options    []widget.Clickable
	choice     widget.Enum
	ok, cancel widget.Clickable
	show       widget.Bool
	remember   widget.Bool
	focusDone  bool
}

// NewPrompter creates a prompter; invalidate redraws the window.
func NewPrompter(invalidate func()) *Prompter { return &Prompter{invalidate: invalidate} }

// Prompt queues a dialog and waits for the answer (or ctx).
func (p *Prompter) Prompt(ctx context.Context, req session.PromptRequest) (session.PromptResponse, error) {
	pd := &pending{req: req, resp: make(chan session.PromptResponse, 1), gone: make(chan struct{})}
	p.mu.Lock()
	p.queue = append(p.queue, pd)
	p.mu.Unlock()
	p.invalidate()
	select {
	case r := <-pd.resp:
		if r.Cancelled {
			return r, session.ErrPromptCancelled
		}
		return r, nil
	case <-ctx.Done():
		close(pd.gone)
		p.invalidate()
		return session.PromptResponse{Cancelled: true}, ctx.Err()
	}
}

// Active reports whether a dialog is shown (it takes all input then).
func (p *Prompter) Active() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queue) > 0
}

func (p *Prompter) top() *pending {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Drop requests whose caller stopped waiting.
	for len(p.queue) > 0 {
		select {
		case <-p.queue[0].gone:
			p.queue = p.queue[1:]
			continue
		default:
		}
		return p.queue[0]
	}
	return nil
}

func (p *Prompter) finish(pd *pending, r session.PromptResponse) {
	p.mu.Lock()
	if len(p.queue) > 0 && p.queue[0] == pd {
		p.queue = p.queue[1:]
	}
	p.mu.Unlock()
	pd.resp <- r
	p.shown = nil
	p.invalidate()
}

func isHostKey(req session.PromptRequest) bool { return strings.HasPrefix(req.Kind, "hostkey") }

// Layout draws the dialog on top (if any) over the whole window.
func (p *Prompter) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	pd := p.top()
	if pd == nil {
		p.shown = nil
		return layout.Dimensions{}
	}
	req := pd.req
	if p.shown != pd {
		p.shown = pd
		p.values = make([]widget.Editor, len(req.Fields))
		for i, f := range req.Fields {
			p.values[i].SetText(f.Value)
		}
		p.options = make([]widget.Clickable, len(req.Options))
		p.choice.Value = ""
		if len(req.Options) > 0 {
			p.choice.Value = req.Options[0]
		}
		p.show.Value, p.remember.Value, p.focusDone = false, false, false
	}

	answer := func(cancelled bool, choice string) {
		r := session.PromptResponse{Cancelled: cancelled, Choice: choice, Values: map[string]string{}, Remember: p.remember.Value}
		for i, f := range req.Fields {
			r.Values[f.Name] = p.values[i].Text()
		}
		p.finish(pd, r)
	}
	submit := func() {
		if len(req.Options) > 0 && !isHostKey(req) {
			answer(false, p.choice.Value)
		} else {
			answer(false, "ok")
		}
	}

	// Input: buttons, Enter in a field, Escape.
	for i := range p.options {
		if p.options[i].Clicked(gtx) {
			answer(false, req.Options[i])
			return layout.Dimensions{}
		}
	}
	if p.ok.Clicked(gtx) {
		submit()
		return layout.Dimensions{}
	}
	if p.cancel.Clicked(gtx) {
		answer(true, "")
		return layout.Dimensions{}
	}
	for i := range p.values {
		for {
			ev, ok := p.values[i].Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.SubmitEvent); ok {
				submit()
				return layout.Dimensions{}
			}
		}
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape}, key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameEscape:
			answer(true, "")
			return layout.Dimensions{}
		case isHostKey(req):
			// The first option is the default button, except for a changed
			// host key, which must be decided with the mouse.
			if req.Kind != "hostkey-changed" && len(req.Options) > 0 {
				answer(false, req.Options[0])
				return layout.Dimensions{}
			}
		default:
			submit()
			return layout.Dimensions{}
		}
	}

	// Backdrop that swallows pointer input for the window below.
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, th.Pal.Backdrop, clip.Rect{Max: size}.Op())
	event.Op(gtx.Ops, p)
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: p, Kinds: pointer.Press | pointer.Release | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1e6, Max: 1e6}}); !ok {
			break
		}
	}

	width := 420
	if isHostKey(req) {
		width = 560
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Dp(unit.Dp(width)), gtx.Constraints.Max.X-gtx.Dp(32))
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
		gtx.Constraints.Min.Y = 0
		m := op.Record(gtx.Ops)
		dims := p.dialog(gtx, th, req)
		call := m.Stop()
		r := image.Rectangle{Max: dims.Size}
		paint.FillShape(gtx.Ops, th.Pal.BorderColor, clip.UniformRRect(r, gtx.Dp(6)).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, th.Pal.BgColor, clip.UniformRRect(r.Inset(max(1, gtx.Dp(1))), gtx.Dp(5)).Op(gtx.Ops))
		call.Add(gtx.Ops)
		return dims
	})
}

func (p *Prompter) dialog(gtx layout.Context, th *gtk.Theme, req session.PromptRequest) layout.Dimensions {
	title := req.Title
	if title == "" {
		title = "LeanTerm"
	}
	icon, iconColor := fonts.IconLock, th.Pal.WarningFg
	switch req.Kind {
	case "hostkey-changed":
		icon, iconColor = fonts.IconWarning, th.Pal.ErrorFg
	case "hostkey-unknown", "confirm":
		icon, iconColor = '', th.Pal.Accent // nf-fa-question_circle
	}

	var body []layout.FlexChild
	body = append(body, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		l := material.Label(th.Theme, gtk.FontSize, req.Message)
		l.Color = th.Pal.FgColor
		return l.Layout(gtx)
	}))
	switch {
	case len(req.Options) > 0 && !isHostKey(req):
		for _, opt := range req.Options {
			opt := opt
			body = append(body, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					rb := material.RadioButton(th.Theme, &p.choice, opt, opt)
					rb.Color, rb.IconColor = th.Pal.FgColor, th.Pal.Accent
					return rb.Layout(gtx)
				})
			}))
		}
	case !isHostKey(req):
		secret := false
		for i, f := range req.Fields {
			i, f := i, f
			secret = secret || f.Secret
			body = append(body, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Label(gtx, f.Label, th.Pal.FgColor) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !p.focusDone && i == 0 {
								gtx.Execute(key.FocusCmd{Tag: &p.values[0]})
								p.focusDone = true
							}
							return layout.Inset{Top: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return gtk.Entry{Password: f.Secret && !p.show.Value, Width: -1}.Layout(gtx, th, &p.values[i])
							})
						}),
					)
				})
			}))
		}
		if secret {
			body = append(body, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, &p.show, "Show", false) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !req.Remember {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return th.CheckButton(gtx, &p.remember, "Remember (store encrypted)", false)
							})
						}),
					)
				})
			}))
		}
	}

	var buttons []layout.FlexChild
	buttons = append(buttons, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }))
	if isHostKey(req) {
		for i, opt := range req.Options {
			i, opt := i, opt
			buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return gtk.Button{Label: opt, Default: i == 0 && req.Kind != "hostkey-changed", MinWidth: 80}.Layout(gtx, th, &p.options[i])
				})
			}))
		}
	} else {
		buttons = append(buttons,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: "Cancel", Default: false, MinWidth: 80}.Layout(gtx, th, &p.cancel)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return gtk.Button{Label: "OK", Default: true, MinWidth: 80}.Layout(gtx, th, &p.ok)
				})
			}),
		)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header bar with the title, like GTK dialogs.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 10, Bottom: 10, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th.Theme, gtk.FontSize, title)
				l.Color, l.Font.Weight = th.Pal.FgColor, font.Bold
				return layout.Center.Layout(gtx, l.Layout)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.HLine(gtx, th.Pal.BorderColor) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return th.Icon(gtx, icon, unit.Sp(26), iconColor)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, body...)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 16, Right: 16, Bottom: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
			})
		}),
	)
}
