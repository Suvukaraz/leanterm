package gtk

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// TreeNode is a group or a connection of a Tree.
type TreeNode struct {
	ID        string
	Name      string
	Group     bool
	Method    string // ssh, telnet, ... (connections)
	Protected bool
	Favourite bool
	Children  []*TreeNode
}

// TreeEventKind is what happened in a Tree.
type TreeEventKind uint8

const (
	TreeSelect TreeEventKind = iota
	TreeActivate
	TreeMiddleClick
	TreeContextClick
	TreeExpand
)

// TreeEvent is an event of a Tree.
type TreeEvent struct {
	Kind TreeEventKind
	ID   string
	// Pos is where a context click happened, relative to the tree.
	Pos image.Point
}

// Tree is the connection tree (TreeView.svelte): 22px rows indented 16px per
// level, an expander, an icon, the name (groups bold) and the favourite star.
type Tree struct {
	Root     *TreeNode
	Expanded map[string]bool
	Selected map[string]bool
	// ProtectedColor highlights protected entries (zero: none).
	ProtectedColor color.NRGBA

	list   layout.List
	rows   map[string]*treeRow
	events []TreeEvent
	focus  string
	tag    int
}

type treeRow struct {
	click    widget.Clickable
	expander widget.Clickable
	hover    Hover
}

// Events returns and clears what happened since the last call.
func (t *Tree) Events() []TreeEvent {
	ev := t.events
	t.events = nil
	return ev
}

func (t *Tree) visible() []struct {
	n     *TreeNode
	depth int
} {
	var out []struct {
		n     *TreeNode
		depth int
	}
	var walk func(n *TreeNode, d int)
	walk = func(n *TreeNode, d int) {
		out = append(out, struct {
			n     *TreeNode
			depth int
		}{n, d})
		if n.Group && t.Expanded[n.ID] {
			for _, c := range n.Children {
				walk(c, d+1)
			}
		}
	}
	if t.Root != nil {
		walk(t.Root, 0)
	}
	return out
}

func (t *Tree) selectOnly(id string) {
	t.Selected = map[string]bool{id: true}
	t.focus = id
	t.events = append(t.events, TreeEvent{Kind: TreeSelect, ID: id})
}

func (t *Tree) toggle(id string) {
	if t.Expanded == nil {
		t.Expanded = map[string]bool{}
	}
	t.Expanded[id] = !t.Expanded[id]
	t.events = append(t.events, TreeEvent{Kind: TreeExpand, ID: id})
}

// Layout handles input and draws the tree into the available space.
func (t *Tree) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	p := th.Pal
	if t.rows == nil {
		t.rows = map[string]*treeRow{}
	}
	if t.Selected == nil {
		t.Selected = map[string]bool{}
	}
	rows := t.visible()
	t.input(gtx, rows)

	size := gtx.Constraints.Max
	// .gtk-treeview: base color, 1px border, 0.9em text.
	box(gtx, size, 0, p.BaseColor, p.BorderColor)
	area := clipRect(gtx, size)
	event.Op(gtx.Ops, &t.tag)
	key.InputHintOp{Tag: &t.tag, Hint: key.HintAny}.Add(gtx.Ops)
	area.Pop()
	inner := gtx
	inner.Constraints = layout.Exact(size.Sub(image.Pt(2*gtx.Dp(1), 2*gtx.Dp(1))))
	o := offset(gtx, gtx.Dp(1), gtx.Dp(1))
	defer o.Pop()
	cl := clipRect(inner, inner.Constraints.Max)
	defer cl.Pop()
	t.list.Axis = layout.Vertical
	return t.list.Layout(inner, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return t.row(gtx, th, rows[i].n, rows[i].depth)
	})
}

func (t *Tree) input(gtx layout.Context, rows []struct {
	n     *TreeNode
	depth int
}) {
	for _, r := range rows {
		n := r.n
		tr := t.rows[n.ID]
		if tr == nil {
			continue
		}
		if tr.expander.Clicked(gtx) && n.Group {
			t.toggle(n.ID)
			continue
		}
		for {
			c, ok := tr.click.Update(gtx)
			if !ok {
				break
			}
			gtx.Execute(key.FocusCmd{Tag: &t.tag})
			switch {
			case c.NumClicks >= 2 && n.Group:
				t.toggle(n.ID)
			case c.NumClicks >= 2:
				t.events = append(t.events, TreeEvent{Kind: TreeActivate, ID: n.ID})
			case c.Modifiers.Contain(key.ModShortcut):
				t.Selected[n.ID] = !t.Selected[n.ID]
				t.focus = n.ID
				t.events = append(t.events, TreeEvent{Kind: TreeSelect, ID: n.ID})
			default:
				t.selectOnly(n.ID)
			}
		}
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: tr, Kinds: pointer.Press})
			if !ok {
				break
			}
			e, ok := ev.(pointer.Event)
			if !ok {
				continue
			}
			switch {
			case e.Buttons.Contain(pointer.ButtonTertiary):
				t.events = append(t.events, TreeEvent{Kind: TreeMiddleClick, ID: n.ID})
			case e.Buttons.Contain(pointer.ButtonSecondary):
				if !t.Selected[n.ID] {
					t.selectOnly(n.ID)
				}
				t.events = append(t.events, TreeEvent{Kind: TreeContextClick, ID: n.ID, Pos: e.Position.Round()})
			}
		}
	}
	// Keyboard: Up/Down move, Left/Right collapse/expand, Enter activates.
	for {
		ev, ok := gtx.Event(
			// Only a tag with a FocusFilter can take the focus.
			key.FocusFilter{Target: &t.tag},
			key.Filter{Focus: &t.tag, Name: key.NameUpArrow},
			key.Filter{Focus: &t.tag, Name: key.NameDownArrow},
			key.Filter{Focus: &t.tag, Name: key.NameLeftArrow},
			key.Filter{Focus: &t.tag, Name: key.NameRightArrow},
			key.Filter{Focus: &t.tag, Name: key.NameReturn},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		cur := -1
		for i, r := range rows {
			if r.n.ID == t.focus {
				cur = i
			}
		}
		switch e.Name {
		case key.NameDownArrow:
			if cur+1 < len(rows) {
				t.selectOnly(rows[cur+1].n.ID)
			}
		case key.NameUpArrow:
			if cur > 0 {
				t.selectOnly(rows[cur-1].n.ID)
			}
		case key.NameRightArrow, key.NameLeftArrow:
			if cur >= 0 && rows[cur].n.Group && t.Expanded[rows[cur].n.ID] != (e.Name == key.NameRightArrow) {
				t.toggle(rows[cur].n.ID)
			}
		case key.NameReturn:
			if cur >= 0 {
				if rows[cur].n.Group {
					t.toggle(rows[cur].n.ID)
				} else {
					t.events = append(t.events, TreeEvent{Kind: TreeActivate, ID: rows[cur].n.ID})
				}
			}
		}
	}
}

// Icons and colors of the tree (the SVG icons of the web GUI as Nerd Fonts).
var (
	treeGroupColor = color.NRGBA{R: 0xff, G: 0xd0, B: 0x1a, A: 0xff}
	treeRootColor  = color.NRGBA{R: 0xf2, G: 0x6b, B: 0x1d, A: 0xff}
	treeSSHColor   = color.NRGBA{R: 0x2e, G: 0x9b, B: 0x36, A: 0xff}
	treeOtherColor = color.NRGBA{R: 0xe0, G: 0x1b, B: 0x24, A: 0xff}
	favColor       = color.NRGBA{R: 0xf5, G: 0xa6, B: 0x23, A: 0xff}
)

func (t *Tree) icon(n *TreeNode) (rune, color.NRGBA) {
	switch {
	case n == t.Root, n.Group && t.Expanded[n.ID]:
		return '\uf07c', treeRootColor // nf-fa-folder_open: the root and open groups are orange
	case n.Group:
		return '\uf07b', treeGroupColor // nf-fa-folder
	case n.Method == "" || n.Method == "ssh" || n.Method == "sftp":
		return '\uf023', treeSSHColor // nf-fa-lock
	}
	return '\uf09c', treeOtherColor // nf-fa-unlock: telnet and others
}

func (t *Tree) row(gtx layout.Context, th *Theme, n *TreeNode, depth int) layout.Dimensions {
	p := th.Pal
	tr := t.rows[n.ID]
	if tr == nil {
		tr = &treeRow{}
		t.rows[n.ID] = tr
	}
	selected := t.Selected[n.ID]
	hovered := tr.hover.Update(gtx)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(22))
	return tr.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// The row's own input area (middle/context clicks, hover) goes below
		// the content, so the expander gets its clicks.
		area := clipRect(gtx, size)
		event.Op(gtx.Ops, tr)
		tr.hover.Add(gtx)
		pointer.CursorPointer.Add(gtx.Ops)
		area.Pop()
		switch {
		case selected:
			FillRect(gtx, image.Rectangle{Max: size}, p.SelectedBg)
		case hovered:
			FillRect(gtx, image.Rectangle{Max: size}, p.RowHover)
		}
		fg := p.TextColor
		if selected {
			fg = p.SelectedFg
		}
		g := gtx
		g.Constraints = layout.Exact(size)
		layout.Inset{Left: unit.Dp(depth*16 + 4 + 1)}.Layout(g, func(gtx layout.Context) layout.Dimensions {
			// Flex centers its children among themselves only: W centers the
			// row in the 22px.
			return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return t.rowContent(gtx, th, tr, n, selected, fg)
			})
		})
		return layout.Dimensions{Size: size}
	})
}

// rowContent: expander, icon, name and favourite star, 4px apart.
func (t *Tree) rowContent(gtx layout.Context, th *Theme, tr *treeRow, n *TreeNode, selected bool, fg color.NRGBA) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.expander(gtx, th, tr, n, selected) }),
		layout.Rigid(layout.Spacer{Width: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			r, c := t.icon(n)
			return th.IconBox(gtx, r, 16, c)
		}),
		layout.Rigid(layout.Spacer{Width: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			weight := font.Normal
			if n.Group {
				weight = font.Bold
			}
			txt := Text{S: n.Name, Size: Em(0.9), Color: fg, Weight: weight, MaxLines: 1}
			if n.Protected && t.ProtectedColor.A != 0 && !selected {
				// Protected entries: the color as background, black text,
				// radius 2px, padding 0 3px.
				txt.Color = color.NRGBA{A: 0xff}
				call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 3, Right: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return txt.Layout(gtx, th) })
				})
				box(gtx, d.Size, gtx.Dp(2), t.ProtectedColor, color.NRGBA{})
				call.Add(gtx.Ops)
				return d
			}
			return txt.Layout(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !n.Favourite {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return th.Icon(gtx, '\uf005', Em(0.9*0.9), favColor) // nf-fa-star
				})
			})
		}),
	)
}

// expander: a 14px box with a small triangle (▶ at 0.7em, muted), pointing
// down when the group is open.
func (t *Tree) expander(gtx layout.Context, th *Theme, tr *treeRow, n *TreeNode, selected bool) layout.Dimensions {
	sz := image.Pt(gtx.Dp(14), gtx.Dp(14))
	if !n.Group {
		return layout.Dimensions{Size: sz}
	}
	return tr.expander.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		c := th.Pal.MutedFg
		if selected {
			c = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		// The triangle of ▶ at 0.7em of the tree's 0.9em: about 7px wide.
		w, h := float32(gtx.Dp(7)), float32(gtx.Dp(8))
		cx, cy := float32(sz.X)/2, float32(sz.Y)/2
		var path clip.Path
		path.Begin(gtx.Ops)
		if t.Expanded[n.ID] {
			path.MoveTo(f32.Pt(cx-h/2, cy-w/2))
			path.LineTo(f32.Pt(cx+h/2, cy-w/2))
			path.LineTo(f32.Pt(cx, cy+w/2))
		} else {
			path.MoveTo(f32.Pt(cx-w/2, cy-h/2))
			path.LineTo(f32.Pt(cx+w/2, cy))
			path.LineTo(f32.Pt(cx-w/2, cy+h/2))
		}
		path.Close()
		paint.FillShape(gtx.Ops, c, clip.Outline{Path: path.End()}.Op())
		return layout.Dimensions{Size: sz}
	})
}
