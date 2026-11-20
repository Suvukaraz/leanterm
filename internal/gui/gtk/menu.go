package gtk

import (
	"image"

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

// MenuItem is an entry of a context menu.
type MenuItem struct {
	ID    string
	Label string
	// Icon is a Nerd Fonts icon (0 = none).
	Icon      rune
	Shortcut  string
	Disabled  bool
	Separator bool
	// Checked shows a check mark in the icon column.
	Checked bool
	Submenu []MenuItem
}

// Menu is a popup menu (ContextMenu.svelte) opened at a point; Activated
// reports the chosen item.
type Menu struct {
	Items []MenuItem

	pos       image.Point
	open      bool
	clicks    []widget.Clickable
	sub       *Menu
	subIndex  int
	focus     int
	activated string
	dismiss   int
	tag       int
	grab      bool
	hovered   int // 1 + the row under the pointer (0: none), to follow its changes
}

// Open shows the menu with its top left corner at pos (in the coordinates
// of the place where Layout is called).
func (m *Menu) Open(pos image.Point) {
	m.pos, m.open, m.focus, m.sub = pos, true, -1, nil
	// hovered stays: a pointer still resting on a row (the one that opened
	// the menu again) has not entered it.
	m.grab = true
}

// Close hides the menu.
func (m *Menu) Close() { m.open, m.sub = false, nil }

// Opened reports whether the menu is shown.
func (m *Menu) Opened() bool { return m.open }

// Activated returns the ID of the item chosen since the last call.
func (m *Menu) Activated() (string, bool) {
	a := m.activated
	m.activated = ""
	if a == "" && m.sub != nil {
		return m.sub.Activated()
	}
	return a, a != ""
}

func (m *Menu) selectable(i int) bool {
	return i >= 0 && i < len(m.Items) && !m.Items[i].Separator && !m.Items[i].Disabled
}

func (m *Menu) activate(i int) {
	it := m.Items[i]
	if len(it.Submenu) > 0 {
		m.openSub(i)
		return
	}
	m.activated = it.ID
	m.open = false
}

func (m *Menu) openSub(i int) {
	if m.sub != nil && m.subIndex == i {
		return
	}
	m.sub = &Menu{Items: m.Items[i].Submenu}
	m.subIndex = i
}

// Layout handles input and draws the open menu on top of everything (it
// must be called where the menu's position is meant).
func (m *Menu) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	if !m.open {
		return layout.Dimensions{}
	}
	m.update(gtx)
	if !m.open {
		return layout.Dimensions{}
	}
	rec := op.Record(gtx.Ops)
	// Clicks outside the menu close it.
	big := clip.Rect{Min: image.Pt(-1<<20, -1<<20), Max: image.Pt(1<<20, 1<<20)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &m.dismiss)
	big.Pop()
	m.draw(gtx, th, m.pos, true)
	op.Defer(gtx.Ops, rec.Stop())
	return layout.Dimensions{}
}

func (m *Menu) update(gtx layout.Context) {
	if len(m.clicks) != len(m.Items) {
		m.clicks = make([]widget.Clickable, len(m.Items))
	}
	for i := range m.clicks {
		if m.clicks[i].Clicked(gtx) && m.selectable(i) {
			m.activate(i)
			gtx.Execute(key.FocusCmd{Tag: &m.tag})
		}
	}
	if m.sub != nil {
		m.sub.update(gtx)
		if a, ok := m.sub.Activated(); ok {
			m.activated, m.open = a, false
			return
		}
	}
	// An open menu takes the keyboard (else the terminal below would).
	if m.grab {
		m.grab = false
		gtx.Execute(key.FocusCmd{Tag: &m.tag})
	}
	for {
		ev, ok := gtx.Event(
			pointer.Filter{Target: &m.dismiss, Kinds: pointer.Press},
			key.FocusFilter{Target: &m.tag},
			key.Filter{Focus: &m.tag, Name: key.NameEscape},
			key.Filter{Focus: &m.tag, Name: key.NameUpArrow},
			key.Filter{Focus: &m.tag, Name: key.NameDownArrow},
			key.Filter{Focus: &m.tag, Name: key.NameReturn},
			key.Filter{Focus: &m.tag, Name: key.NameRightArrow},
			key.Filter{Focus: &m.tag, Name: key.NameLeftArrow},
		)
		if !ok {
			break
		}
		switch e := ev.(type) {
		case pointer.Event:
			m.Close()
		case key.Event:
			if e.State != key.Press {
				continue
			}
			target := m
			for target.sub != nil && target.sub.focus >= 0 {
				target = target.sub
			}
			target.key(e.Name, m)
		}
	}
}

// key moves the focus within this (sub)menu; root is the outermost menu.
func (m *Menu) key(name key.Name, root *Menu) {
	switch name {
	case key.NameEscape:
		root.Close()
	case key.NameDownArrow, key.NameUpArrow:
		step := 1
		if name == key.NameUpArrow {
			step = -1
		}
		start := m.focus + step
		if m.focus < 0 && step < 0 {
			start = len(m.Items) - 1 // Up without a focused row: the last one
		}
		for i, n := start, 0; n < len(m.Items); i, n = i+step, n+1 {
			i = (i + len(m.Items)) % len(m.Items)
			if m.selectable(i) {
				m.focus = i
				break
			}
		}
	case key.NameReturn, key.NameRightArrow:
		if m.selectable(m.focus) {
			if len(m.Items[m.focus].Submenu) > 0 {
				m.openSub(m.focus)
				m.sub.focus = 0
			} else if name == key.NameReturn {
				m.activate(m.focus)
				root.activated, root.open = m.activated, false
			}
		}
	case key.NameLeftArrow:
		// Back to the parent: find it from the root.
		for p := root; p.sub != nil; p = p.sub {
			if p.sub == m {
				p.sub = nil
				break
			}
		}
	}
}

// draw lays the menu out at pos (and its open submenu next to its row).
func (m *Menu) draw(gtx layout.Context, th *Theme, pos image.Point, root bool) {
	p := th.Pal
	// A submenu opened by a key this frame has not been updated yet.
	if len(m.clicks) != len(m.Items) {
		m.clicks = make([]widget.Clickable, len(m.Items))
	}
	orig := gtx.Constraints
	// min-width 200px, max-width 420px, else as wide as the widest row.
	width := gtx.Dp(200)
	for i, it := range m.Items {
		if it.Separator {
			continue
		}
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(420), 1<<20)}
		_, d := record(g, func(gtx layout.Context) layout.Dimensions { return m.rowContent(gtx, th, i, it, false, true) })
		width = max(width, min(d.Size.X, gtx.Dp(420)))
	}
	gtx.Constraints = layout.Exact(image.Pt(width, 0))
	gtx.Constraints.Max.Y = 1 << 20
	var subAt image.Point
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		var rows []layout.FlexChild
		y := gtx.Dp(4)
		for i, it := range m.Items {
			i, it := i, it
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if it.Separator {
					return layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return HLine(gtx, p.BorderColor)
					})
				}
				dims := m.row(gtx, th, i, it)
				if m.sub != nil && m.subIndex == i {
					subAt = image.Pt(0, y)
				}
				y += dims.Size.Y
				return dims
			}))
		}
		return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
	gtx.Constraints = orig
	// Keep the open menu inside the window (the position is usually the
	// pointer, which can be near the edge).
	if max := gtx.Constraints.Max; max.X > 0 && max.Y > 0 {
		if pos.X+d.Size.X > max.X {
			pos.X = max.X - d.Size.X
		}
		if pos.Y+d.Size.Y > max.Y {
			pos.Y = max.Y - d.Size.Y
		}
		if pos.X < 0 {
			pos.X = 0
		}
		if pos.Y < 0 {
			pos.Y = 0
		}
	}
	o := offset(gtx, pos.X, pos.Y)
	defer o.Pop()
	// box-shadow 0 6px 18px, border 1px, radius 5px.
	for i := 1; i <= 4; i++ {
		s := gtx.Dp(unit.Dp(i * 3))
		r := image.Rectangle{Min: image.Pt(-s/2, -s/2+gtx.Dp(6)), Max: d.Size.Add(image.Pt(s/2, s/2+gtx.Dp(6)))}
		paint.FillShape(gtx.Ops, faded(p.Shadow, 0.15), clip.UniformRRect(r, gtx.Dp(5)+s/2).Op(gtx.Ops))
	}
	box(gtx, d.Size, gtx.Dp(5), p.MenuBg, p.BorderColor)
	// Clicks between the rows stay in the menu.
	area := clipRect(gtx, d.Size)
	event.Op(gtx.Ops, &m.tag)
	area.Pop()
	call.Add(gtx.Ops)
	if m.sub != nil {
		// Submenus open next to their row (anchor.right - 2, anchor.top - 4).
		m.sub.draw(gtx, th, image.Pt(d.Size.X-gtx.Dp(2), subAt.Y-gtx.Dp(4)), false)
	}
}

// row draws an item: padding 4px 10px, gap 8px, a 16px icon column, the
// label, the shortcut (0.85em, muted) and the submenu arrow.
func (m *Menu) row(gtx layout.Context, th *Theme, i int, it MenuItem) layout.Dimensions {
	c := &m.clicks[i]
	// The pointer moves the focus when it enters a row; while it rests,
	// the keyboard keeps the focus it moved.
	switch h := c.Hovered(); {
	case h && m.hovered != i+1:
		m.hovered = i + 1
		if m.selectable(i) {
			m.focus = i
			if len(it.Submenu) > 0 {
				m.openSub(i)
			} else {
				m.sub = nil
			}
		}
	case !h && m.hovered == i+1:
		m.hovered = 0
	}
	hot := m.selectable(i) && (m.focus == i || (m.sub != nil && m.subIndex == i))
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return m.rowContent(gtx, th, i, it, hot, false)
	})
}

// rowContent draws a row; measure lays it out at its natural width.
func (m *Menu) rowContent(gtx layout.Context, th *Theme, i int, it MenuItem, hot, measure bool) layout.Dimensions {
	p := th.Pal
	fg, muted := p.FgColor, p.MutedFg
	if hot {
		fg, muted = p.SelectedFg, faded(p.SelectedFg, 0.8)
	}
	if it.Disabled {
		fg, muted = faded(fg, 0.45), faded(muted, 0.45)
	}
	call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !measure {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
			}
			label := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Text{S: it.Label, Size: Em(0.95), Color: fg, MaxLines: 1}.Layout(gtx, th)
			})
			if !measure {
				label = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					d := Text{S: it.Label, Size: Em(0.95), Color: fg, MaxLines: 1}.Layout(gtx, th)
					d.Size.X = gtx.Constraints.Max.X
					return d
				})
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					switch {
					case it.Checked:
						return th.IconBox(gtx, '\uf00c', 16, fg) // nf-fa-check
					case it.Icon != 0:
						return th.IconBox(gtx, it.Icon, 16, fg)
					}
					return layout.Dimensions{Size: image.Pt(gtx.Dp(16), gtx.Dp(16))}
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				label,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var meta []layout.FlexChild
					if it.Shortcut != "" {
						meta = append(meta, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return Text{S: it.Shortcut, Size: Em(0.95 * 0.85), Color: muted, MaxLines: 1}.Layout(gtx, th)
						}))
					}
					if len(it.Submenu) > 0 {
						if len(meta) > 0 {
							meta = append(meta, layout.Rigid(layout.Spacer{Width: 6}.Layout))
						}
						meta = append(meta, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return th.Icon(gtx, '\uf0da', Em(0.95*0.75), fg) // nf-fa-caret_right
						}))
					}
					if len(meta) == 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx, meta...)
					})
				}),
			)
		})
	})
	if hot {
		FillRect(gtx, image.Rectangle{Max: d.Size}, p.SelectedBg)
	}
	call.Add(gtx.Ops)
	return d
}
