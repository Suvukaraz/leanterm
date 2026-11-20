// Package picker is a modal fuzzy picker: a text field with a list of
// items below it, centered near the top of its area (Go to Symbol; meant
// for a command palette or Quick Open too). The owner supplies the items
// for a query, so the picker knows nothing about what it picks.
package picker

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
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// Item is one row: a badge, the label with its matched characters, a
// muted detail after it and a right-aligned note.
type Item struct {
	Badge   string
	Label   string
	Matched []int // rune indexes of Label to highlight
	Detail  string
	Right   string
	Value   any
}

// Picker is the modal picker. Set Items, OnSelect (and Placeholder, Empty,
// OnClose), then Open it and lay it out over its area every frame.
type Picker struct {
	Placeholder string
	// Empty is shown when there are no items (for any query).
	Empty string
	// NoMatch is shown when the query matches nothing.
	NoMatch string
	// Items are the items for a query (called when the query changes).
	Items func(query string) []Item
	// OnSelect is called with the chosen item (the picker is closed).
	OnSelect func(Item)
	// OnClose is called when the picker closes without a choice.
	OnClose func()
	// MaxRows is the number of rows shown before the list scrolls (10).
	MaxRows int

	open   bool
	focus  bool
	ed     widget.Editor
	query  string
	items  []Item
	index  int
	clicks []widget.Clickable
	list   widget.List
	back   int
	scroll bool
}

// Open shows the picker with an empty query and the item at selected
// chosen.
func (p *Picker) Open(selected int) {
	p.open, p.focus, p.scroll = true, true, true
	p.ed.SetText("")
	p.query = ""
	p.refresh()
	p.index = max(0, min(selected, len(p.items)-1))
}

// Opened reports whether the picker is shown.
func (p *Picker) Opened() bool { return p.open }

// Close hides the picker (OnClose is called).
func (p *Picker) Close() {
	if !p.open {
		return
	}
	p.open = false
	if p.OnClose != nil {
		p.OnClose()
	}
}

// Query is the text in the field.
func (p *Picker) Query() string { return p.ed.Text() }

// SetQuery replaces the query (for tests and callers that prefill it).
func (p *Picker) SetQuery(q string) {
	p.ed.SetText(q)
	p.query = q
	p.refresh()
	p.index = 0
}

// Visible are the items shown and the chosen one.
func (p *Picker) Visible() ([]Item, int) { return p.items, p.index }

// Move moves the choice by d rows, wrapping around.
func (p *Picker) Move(d int) {
	if n := len(p.items); n > 0 {
		p.index = ((p.index+d)%n + n) % n
		p.scroll = true
	}
}

// Choose picks the chosen item.
func (p *Picker) Choose() {
	if p.index < 0 || p.index >= len(p.items) {
		return
	}
	it := p.items[p.index]
	p.open = false
	if p.OnSelect != nil {
		p.OnSelect(it)
	}
}

func (p *Picker) refresh() {
	p.items = nil
	if p.Items != nil {
		p.items = p.Items(p.query)
	}
	if len(p.clicks) < len(p.items) {
		p.clicks = append(p.clicks, make([]widget.Clickable, len(p.items)-len(p.clicks))...)
	}
}

// Layout handles the picker's input and draws it over gtx's area.
func (p *Picker) Layout(gtx layout.Context, th *gtk.Theme) {
	if !p.open {
		return
	}
	pal := th.Pal
	if q := p.ed.Text(); q != p.query {
		p.query = q
		p.refresh()
		p.index, p.scroll = 0, true
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: &p.ed, Name: key.NameEscape},
			key.Filter{Focus: &p.ed, Name: key.NameUpArrow},
			key.Filter{Focus: &p.ed, Name: key.NameDownArrow},
			key.Filter{Focus: &p.ed, Name: key.NameReturn},
			key.Filter{Focus: &p.ed, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameEscape:
			p.Close()
			return
		case key.NameDownArrow:
			p.Move(1)
		case key.NameUpArrow:
			p.Move(-1)
		default:
			p.Choose()
			return
		}
	}
	for i := range p.items {
		if p.clicks[i].Clicked(gtx) {
			p.index = i
			p.Choose()
			return
		}
	}
	// A click outside closes it.
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &p.back, Kinds: pointer.Press}); !ok {
			break
		}
		p.Close()
		return
	}
	p.ed.SingleLine = true
	p.ed.Submit = false
	if p.focus {
		p.focus = false
		gtx.Execute(key.FocusCmd{Tag: &p.ed})
	}
	func() {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &p.back)
	}()

	rows := p.MaxRows
	if rows <= 0 {
		rows = 10
	}
	w := min(gtx.Dp(620), gtx.Constraints.Max.X*9/10)
	x := (gtx.Constraints.Max.X - w) / 2
	defer op.Offset(image.Pt(x, gtx.Dp(48))).Push(gtx.Ops).Pop()
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
	m := op.Record(gtx.Ops)
	d := layout.UniformInset(6).Layout(g, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return gtk.Entry{Placeholder: p.Placeholder, Width: -1}.Layout(gtx, th, &p.ed)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(p.items) == 0 {
					msg := p.Empty
					if p.query != "" && p.NoMatch != "" {
						msg = p.NoMatch
					}
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, gtk.Text{S: msg, Color: pal.MutedFg}.Widget(th))
				}
				rowH := gtx.Dp(26)
				gtx.Constraints.Max.Y = rowH * rows
				p.list.Axis = layout.Vertical
				if p.scroll {
					p.scroll = false
					first, n := p.list.Position.First, max(1, p.list.Position.Count-1)
					if p.index < first || p.index >= first+n {
						p.list.Position.First = max(0, p.index-rows/2)
						p.list.Position.Offset = 0
					}
				}
				return p.list.Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
					return p.row(gtx, th, i, rowH)
				})
			}),
		)
	})
	call := m.Stop()
	gtk.Box(gtx, d.Size, 6, pal.MenuBg, pal.BorderColor)
	call.Add(gtx.Ops)
}

func (p *Picker) row(gtx layout.Context, th *gtk.Theme, i, rowH int) layout.Dimensions {
	pal := th.Pal
	it := p.items[i]
	active := i == p.index
	fg, muted, accent := pal.FgColor, pal.MutedFg, pal.Accent
	if active {
		fg, muted, accent = pal.SelectedFg, pal.SelectedFg, pal.SelectedFg
	}
	return p.clicks[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, rowH)
		gtx.Constraints.Max.Y = rowH
		m := op.Record(gtx.Ops)
		d := layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if it.Badge == "" {
						return layout.Dimensions{}
					}
					return Badge(gtx, th, it.Badge, active)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Highlight(gtx, th, it.Label, it.Matched, fg, accent, "")
				}),
				layout.Rigid(layout.Spacer{Width: 10}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Point{}
					d := gtk.Text{S: it.Detail, Size: gtk.Em(0.85), Color: muted, MaxLines: 1}.Layout(gtx, th)
					d.Size.X = gtx.Constraints.Max.X // the line number goes to the right
					return d
				}),
				layout.Rigid(gtk.Text{S: it.Right, Size: gtk.Em(0.85), Color: muted, MaxLines: 1}.Widget(th)),
			)
		})
		call := m.Stop()
		d.Size = image.Pt(gtx.Constraints.Max.X, rowH)
		if active {
			gtk.Box(gtx, d.Size, 3, pal.SelectedBg, color.NRGBA{})
		}
		call.Add(gtx.Ops)
		return d
	})
}

// Badge draws a small kind label in a rounded box.
func Badge(gtx layout.Context, th *gtk.Theme, s string, active bool) layout.Dimensions {
	pal := th.Pal
	fg, bg := pal.MutedFg, pal.HeaderBg
	if active {
		fg, bg = pal.SelectedFg, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x30}
	}
	gtx.Constraints.Min = image.Point{}
	m := op.Record(gtx.Ops)
	d := layout.Inset{Left: 4, Right: 4, Top: 1, Bottom: 1}.Layout(gtx,
		gtk.Text{S: s, Size: gtk.Em(0.75), Color: fg, Mono: true, MaxLines: 1}.Widget(th))
	call := m.Stop()
	d.Size.X = max(d.Size.X, gtx.Dp(unit.Dp(30)))
	gtk.Box(gtx, d.Size, 3, bg, color.NRGBA{})
	call.Add(gtx.Ops)
	return d
}

// Highlight draws s with the runes at matched (rune indexes) bold in the
// accent color, in face (the theme's font when empty).
func Highlight(gtx layout.Context, th *gtk.Theme, s string, matched []int, fg, accent color.NRGBA, face font.Typeface) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	type seg struct {
		s   string
		hit bool
	}
	var segs []seg
	hits := map[int]bool{}
	for _, i := range matched {
		hits[i] = true
	}
	r := []rune(s)
	start := 0
	for i := 1; i <= len(r); i++ {
		if i == len(r) || hits[i] != hits[start] {
			segs = append(segs, seg{string(r[start:i]), hits[start]})
			start = i
		}
	}
	children := make([]layout.FlexChild, 0, len(segs))
	for _, sg := range segs {
		t := gtk.Text{S: sg.s, Color: fg, Face: face, MaxLines: 1}
		if sg.hit {
			t.Color, t.Weight = accent, font.Bold
		}
		children = append(children, layout.Rigid(t.Widget(th)))
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
}
