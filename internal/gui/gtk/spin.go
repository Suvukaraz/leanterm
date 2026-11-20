package gtk

import (
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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
	up, dn  widget.Clickable
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
	// The editor's own events after ours: it would take Up/Down itself.
	for {
		ev, ok := s.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			s.commit()
		}
	}
	if s.up.Clicked(gtx) && !disabled {
		s.step(1)
	}
	if s.dn.Clicked(gtx) && !disabled {
		s.step(-1)
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
		// The right inset leaves room for WebKit's stepper.
		return layout.Inset{Left: 5, Right: 19, Top: 3, Bottom: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
	// Wheel and hover below the editor, which needs the clicks.
	area := clipRect(gtx, d.Size)
	pointer.CursorText.Add(gtx.Ops)
	eventArea(gtx, s)
	s.hover.Add(gtx)
	area.Pop()
	call.Add(gtx.Ops)
	s.stepper(gtx, th, d.Size, disabled)
	return d
}

// stepper draws the up/down arrows of an <input type=number> as WebKitGTK
// shows them: a small bordered box at the right, split in two halves.
func (s *SpinButton) stepper(gtx layout.Context, th *Theme, size image.Point, disabled bool) {
	p := th.Pal
	w, h := gtx.Dp(12), min(gtx.Dp(16), size.Y-gtx.Dp(4))
	x, y := size.X-w-gtx.Dp(4), (size.Y-h)/2
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	border, col := p.BorderColor, p.FgColor
	if disabled {
		border, col = faded(border, 0.5), faded(col, 0.5)
	}
	box(gtx, image.Pt(w, h), gtx.Dp(2), p.BaseColor, border)
	half := h / 2
	for i, c := range []*widget.Clickable{&s.up, &s.dn} {
		top := i * half
		o := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !disabled {
				pointer.CursorDefault.Add(gtx.Ops)
			}
			return layout.Dimensions{Size: image.Pt(w, half)}
		})
		if c.Hovered() && !disabled {
			FillRect(gtx, image.Rect(gtx.Dp(1), 0, w-gtx.Dp(1), half), p.RowHover)
		}
		// A filled triangle, 6x3px, centered in the half.
		cx, cy := float32(w)/2, float32(half)/2
		tw, th2 := float32(gtx.Dp(3)), float32(gtx.Dp(unit.Dp(1.5)))
		var path clip.Path
		path.Begin(gtx.Ops)
		if i == 0 {
			path.MoveTo(f32.Pt(cx-tw, cy+th2))
			path.LineTo(f32.Pt(cx+tw, cy+th2))
			path.LineTo(f32.Pt(cx, cy-th2))
		} else {
			path.MoveTo(f32.Pt(cx-tw, cy-th2))
			path.LineTo(f32.Pt(cx+tw, cy-th2))
			path.LineTo(f32.Pt(cx, cy+th2))
		}
		path.Close()
		paint.FillShape(gtx.Ops, col, clip.Outline{Path: path.End()}.Op())
		o.Pop()
	}
}
