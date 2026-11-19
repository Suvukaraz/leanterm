package gtk

import (
	"math"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// SpinButton is a number entry (SpinButton.svelte, an <input type=number>):
// typed values are clamped to [Min, Max] when confirmed, Up/Down and the
// mouse wheel step by Step.
type SpinButton struct {
	Value, Min, Max, Step float64
	// Width defaults to 70px as in the web GUI.
	Width unit.Dp

	editor  widget.Editor
	shown   float64
	inited  bool
	changed bool
	focused bool
	scroll  float32
	hover   Hover
}

// Changed reports whether the user changed the value since the last call.
func (s *SpinButton) Changed() bool {
	c := s.changed
	s.changed = false
	return c
}

func (s *SpinButton) format() string {
	if s.Step != 0 && s.Step == math.Trunc(s.Step) {
		return strconv.FormatFloat(s.Value, 'f', 0, 64)
	}
	return strconv.FormatFloat(s.Value, 'f', -1, 64)
}

func (s *SpinButton) set(v float64) {
	if s.Max > s.Min {
		v = math.Max(s.Min, math.Min(s.Max, v))
	}
	if v != s.Value {
		s.Value = v
		s.changed = true
	}
	s.editor.SetText(s.format())
	s.shown = s.Value
}

func (s *SpinButton) commit() {
	t := strings.TrimSpace(s.editor.Text())
	if v, err := strconv.ParseFloat(t, 64); err == nil {
		s.set(v)
		return
	}
	s.set(s.Value) // invalid input: back to the value
}

func (s *SpinButton) step(n float64) {
	st := s.Step
	if st == 0 {
		st = 1
	}
	s.commit()
	s.set(s.Value + n*st)
}

// Layout handles input and draws the spin button.
func (s *SpinButton) Layout(gtx layout.Context, th *Theme, disabled bool) layout.Dimensions {
	p := th.Pal
	if !s.inited || (s.Value != s.shown && !gtx.Focused(&s.editor)) {
		s.inited = true
		s.editor.SetText(s.format())
		s.shown = s.Value
	}
	s.editor.SingleLine, s.editor.Submit = true, true
	s.editor.Filter = "0123456789.-"
	s.editor.ReadOnly = disabled
	if disabled {
		gtx = gtx.Disabled()
	}
	for {
		ev, ok := s.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			s.commit()
		}
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &s.editor, Name: key.NameUpArrow},
			key.Filter{Focus: &s.editor, Name: key.NameDownArrow},
			pointer.Filter{Target: s, Kinds: pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1e6, Max: 1e6}},
		)
		if !ok {
			break
		}
		switch e := ev.(type) {
		case key.Event:
			if e.State != key.Press {
				continue
			}
			if e.Name == key.NameUpArrow {
				s.step(1)
			} else {
				s.step(-1)
			}
		case pointer.Event:
			// The wheel steps while the entry has the focus, as in the web view.
			if gtx.Focused(&s.editor) {
				s.scroll += e.Scroll.Y
				for s.scroll <= -1 {
					s.scroll++
					s.step(1)
				}
				for s.scroll >= 1 {
					s.scroll--
					s.step(-1)
				}
			}
		}
	}
	focused := gtx.Focused(&s.editor)
	if s.focused && !focused {
		s.commit() // leaving the entry confirms the value
	}
	s.focused = focused

	w := s.Width
	if w == 0 {
		w = 70
	}
	px := min(gtx.Dp(w), gtx.Constraints.Max.X)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
	ed := material.Editor(th.Theme, &s.editor, "")
	ed.TextSize = Em(0.9)
	ed.Color = p.FgColor
	ed.SelectionColor = faded(p.SelectedBg, 0.45)
	// CSS: padding 2px 4px, border 1px, no radius; header color, base on
	// hover, the focus color while focused.
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 5, Right: 5, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return ed.Layout(gtx)
		})
	})
	fill := p.HeaderBg
	switch {
	case focused:
		fill = p.BtnFocusBg
	case s.hover.Update(gtx):
		fill = p.BaseColor
	}
	border := p.BorderColor
	if disabled {
		fill, border = faded(fill, 0.5), faded(border, 0.5)
	}
	box(gtx, d.Size, 0, fill, border)
	call.Add(gtx.Ops)
	area := clipRect(gtx, d.Size)
	pointer.CursorText.Add(gtx.Ops)
	eventArea(gtx, s)
	s.hover.Add(gtx)
	area.Pop()
	return d
}
