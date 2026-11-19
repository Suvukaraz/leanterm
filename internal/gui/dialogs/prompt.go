// Package dialogs holds the GUI's dialogs that the backend asks for.
package dialogs

import (
	"context"
	"image/color"
	"sync"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

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
//
// The dialogs are gtk.Dialog like the web GUI's PromptDialog.svelte: host
// key decisions and confirmations show their option buttons, other requests
// a choice list (a <select>) or text fields.
type Prompter struct {
	invalidate func()

	mu    sync.Mutex
	queue []*pending

	// Titles names a session for the "session" line of a dialog (the web
	// GUI shows it above the message). It may be nil.
	Titles func(tmpID string) string
	// OnShow is called on the GUI goroutine when a dialog is shown (the
	// web GUI brings the affected terminal to the front). It may be nil.
	OnShow func(tmpID string)

	// Widgets of the dialog on top, reset for each request.
	shown     *pending
	dlg       gtk.Dialog
	values    []widget.Editor
	combo     gtk.Combo
	options   []widget.Clickable
	ok        widget.Clickable
	cancel    widget.Clickable
	show      widget.Bool
	remember  widget.Bool
	focusDone bool
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

// isHostKey reports whether the request shows its options as buttons (the
// web GUI's PromptDialog treats confirm the same).
func isHostKey(req session.PromptRequest) bool {
	return req.Kind == "hostkey-unknown" || req.Kind == "hostkey-changed" || req.Kind == "confirm"
}

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
		p.combo = gtk.Combo{Value: firstOrEmpty(req.Options)}
		p.show.Value, p.remember.Value, p.focusDone = false, false, false
		if p.OnShow != nil && req.TmpID != "" {
			p.OnShow(req.TmpID)
		}
	}

	answer := func(cancelled bool, choice string) {
		r := session.PromptResponse{Cancelled: cancelled, Choice: choice, Values: map[string]string{}, Remember: p.remember.Value}
		for i, f := range req.Fields {
			r.Values[f.Name] = p.values[i].Text()
		}
		p.finish(pd, r)
	}
	submit := func() {
		if !isHostKey(req) && len(req.Options) > 0 {
			answer(false, p.combo.Value)
		} else {
			answer(false, "ok")
		}
	}

	// Input: option buttons, Enter in a field, Return, Escape (the dialog's).
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
		ev, ok := gtx.Event(key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if isHostKey(req) {
			// The first option is the default button, except for a changed
			// host key, which must be decided with the mouse.
			if req.Kind != "hostkey-changed" && len(req.Options) > 0 {
				answer(false, req.Options[0])
				return layout.Dimensions{}
			}
			continue
		}
		submit()
		return layout.Dimensions{}
	}

	p.dlg.Title = req.Title
	if p.dlg.Title == "" {
		p.dlg.Title = "LeanTerm"
	}
	p.dlg.Width = 420
	if isHostKey(req) {
		p.dlg.Width = 560
	}
	p.dlg.Closable = true

	icon, iconColor := promptIcon(th, req.Kind)
	sessionTitle := ""
	if p.Titles != nil && req.TmpID != "" {
		sessionTitle = p.Titles(req.TmpID)
	}

	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 2, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.Icon(gtx, icon, unit.Sp(26), iconColor)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				var rows []layout.FlexChild
				if sessionTitle != "" {
					sessionTitle := sessionTitle
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return gtk.Text{S: sessionTitle, Size: gtk.Em(0.85), Color: th.Pal.MutedFg, MaxLines: 1}.Layout(gtx, th)
					}))
				}
				rows = append(rows,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return gtk.Text{S: req.Message, Color: th.Pal.FgColor, LineHeight: 1.4}.Layout(gtx, th)
						})
					}),
				)
				switch {
				case isHostKey(req):
					// The options are the dialog's buttons.
				case len(req.Options) > 0:
					opts := make([]gtk.ComboOption, 0, len(req.Options))
					for _, o := range req.Options {
						opts = append(opts, gtk.ComboOption{Value: o, Label: o})
					}
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return p.combo.Layout(gtx, th, opts, false)
						})
					}))
				default:
					secret := false
					for i, f := range req.Fields {
						i, f := i, f
						secret = secret || f.Secret
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			}),
		)
	}

	var acts []layout.Widget
	if isHostKey(req) {
		for i := range p.options {
			i := i
			acts = append(acts, func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: req.Options[i], Default: i == 0 && req.Kind != "hostkey-changed", MinWidth: unit.Dp(80)}.Layout(gtx, th, &p.options[i])
			})
		}
	} else {
		acts = append(acts,
			func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: "Cancel", MinWidth: unit.Dp(80)}.Layout(gtx, th, &p.cancel)
			},
			func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: "OK", Default: true, MinWidth: unit.Dp(80)}.Layout(gtx, th, &p.ok)
			},
		)
	}

	d := p.dlg.Layout(gtx, th, body, acts...)
	if p.dlg.CloseRequested() {
		answer(true, "")
		return layout.Dimensions{}
	}
	return d
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func promptIcon(th *gtk.Theme, kind string) (rune, color.NRGBA) {
	switch kind {
	case "hostkey-changed":
		return fonts.IconWarning, th.Pal.ErrorFg
	case "hostkey-unknown", "confirm":
		return '\uf059', th.Pal.Accent // nf-fa-question_circle
	}
	return fonts.IconLock, th.Pal.WarningFg
}
