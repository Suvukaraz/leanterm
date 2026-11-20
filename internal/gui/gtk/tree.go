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
	"gioui.org/op"
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
	// TreeDrop is emitted when a drag ends on a group (or the root):
	// ID is the drop target, IDs the dragged nodes.
	TreeDrop
)

// TreeEvent is an event of a Tree.
type TreeEvent struct {
	Kind TreeEventKind
	ID   string
	// IDs carries the dragged nodes of a TreeDrop event.
	IDs []string
	// Pos is where a context click happened, relative to the tree.
	Pos image.Point
}

// Tree is the connection tree (TreeView.svelte): 22px rows indented 16px per
// level, an expander, an icon, the name (groups bold) and the favourite star.
// Rows can be dragged onto a group (or the root) to move them, like in the
// web GUI.
type Tree struct {
	Root     *TreeNode
	Expanded map[string]bool
	Selected map[string]bool
	// ProtectedColor highlights protected entries (zero: none).
	ProtectedColor color.NRGBA

	list    layout.List
	rows    map[string]*treeRow
	events  []TreeEvent
	focus   string
	tag     int
	dragTag int

	drag treeDrag
}

// treeDrag is the state of an ongoing drag in a Tree.
type treeDrag struct {
	// armed is a press that may become a drag (set on Press, cleared on
	// release or when the drag is over).
	armed  bool
	fromID string
	pid    pointer.ID
	start  image.Point
	// active means the pointer has moved far enough to drag.
	active bool
	cur    image.Point
	// target is the group (or root) under the cursor; targetOn is false
	// when there is no valid drop target.
	target   string
	targetOn bool
	ids      []string
	// desc caches the descendants of the dragged nodes (invalid targets).
	desc map[string]bool
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
	// Expanding or collapsing changes the rows of this very frame.
	rows = t.visible()
	// Callers read the events before Layout: ask for the frame that does.
	if len(t.events) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}

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
	// The drag area covers the list and sits before its rows, so the rows
	// are its children; while a drag is active (or the press may become
	// one) all pointer events for it come to this tag.
	event.Op(gtx.Ops, &t.dragTag)
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
			case c.Modifiers.Contain(key.ModShift):
				// Shift adds to the selection without toggling (the web GUI).
				if !t.Selected[n.ID] {
					t.Selected[n.ID] = true
					t.focus = n.ID
					t.events = append(t.events, TreeEvent{Kind: TreeSelect, ID: n.ID})
				}
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
	// Drag & drop: arm on press, track while dragging, drop on a group.
	t.dragInput(gtx, rows)

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

// nodeMap indexes the whole tree by node id.
func (t *Tree) nodeMap() map[string]*TreeNode {
	m := map[string]*TreeNode{}
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		m[n.ID] = n
		for _, c := range n.Children {
			walk(c)
		}
	}
	if t.Root != nil {
		walk(t.Root)
	}
	return m
}

// dragDescendants reports whether id is a descendant of one of the dragged
// nodes (such nodes cannot be a drop target).
func (t *Tree) dragDescendants(ids []string) map[string]bool {
	m := t.nodeMap()
	desc := map[string]bool{}
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		for _, c := range n.Children {
			if !desc[c.ID] {
				desc[c.ID] = true
				walk(c)
			}
		}
	}
	for _, id := range ids {
		if n, ok := m[id]; ok {
			walk(n)
		}
	}
	return desc
}

// indexAt maps a y coordinate (in the list's coordinate system) to a global
// row index, or -1 when the point is not over a row.
func (t *Tree) indexAt(gtx layout.Context, rows []struct {
	n     *TreeNode
	depth int
}, y int) int {
	rowH := gtx.Dp(22)
	yy := t.list.Position.Offset
	for i := t.list.Position.First; i < len(rows); i++ {
		if y < yy {
			return -1
		}
		if y < yy+rowH {
			return i
		}
		yy += rowH
	}
	return -1
}

// dragIds are the nodes a drag moves: the selection when the pressed row is
// in it, else the row alone.
func (t *Tree) dragIds(rows []struct {
	n     *TreeNode
	depth int
}, fromID string) []string {
	if t.Selected[fromID] {
		ids := []string{}
		for _, r := range rows {
			if t.Selected[r.n.ID] {
				ids = append(ids, r.n.ID)
			}
		}
		return ids
	}
	return []string{fromID}
}

// dragInput implements the drag & drop of rows onto a group (or the root).
func (t *Tree) dragInput(gtx layout.Context, rows []struct {
	n     *TreeNode
	depth int
}) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &t.dragTag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Source == pointer.Touch {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			if !e.Buttons.Contain(pointer.ButtonPrimary) || t.drag.armed {
				break
			}
			idx := t.indexAt(gtx, rows, int(e.Position.Y))
			if idx < 0 {
				break
			}
			n := rows[idx].n
			// The root cannot be dragged, protected nodes neither (the web GUI).
			if n.ID == "" || n.Protected {
				break
			}
			t.drag = treeDrag{
				armed:  true,
				fromID: n.ID,
				pid:    e.PointerID,
				start:  image.Pt(int(e.Position.X), int(e.Position.Y)),
				ids:    t.dragIds(rows, n.ID),
			}
		case pointer.Drag:
			if !t.drag.armed || e.PointerID != t.drag.pid {
				break
			}
			p := image.Pt(int(e.Position.X), int(e.Position.Y))
			if !t.drag.active {
				dx, dy := float32(p.X-t.drag.start.X), float32(p.Y-t.drag.start.Y)
				if slop := float32(gtx.Dp(4)); dx*dx+dy*dy < slop*slop {
					break
				}
				t.drag.active = true
				gtx.Execute(pointer.GrabCmd{Tag: &t.dragTag, ID: e.PointerID})
			}
			t.drag.cur = p
			t.drag.target, t.drag.targetOn = "", false
			if t.drag.desc == nil {
				t.drag.desc = t.dragDescendants(t.drag.ids)
			}
			if idx := t.indexAt(gtx, rows, p.Y); idx >= 0 {
				n := rows[idx].n
				if n.Group && !t.drag.desc[n.ID] && !containsID(t.drag.ids, n.ID) {
					t.drag.target, t.drag.targetOn = n.ID, true
				}
			}
		case pointer.Release:
			if !t.drag.armed || e.PointerID != t.drag.pid {
				break
			}
			if t.drag.active && t.drag.targetOn {
				t.events = append(t.events, TreeEvent{Kind: TreeDrop, ID: t.drag.target, IDs: t.drag.ids})
			}
			t.drag = treeDrag{}
		case pointer.Cancel:
			if e.PointerID == t.drag.pid {
				t.drag = treeDrag{}
			}
		}
	}
}

func containsID(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

// Icons and colors of the tree (the SVG icons of the web GUI as Nerd Fonts).
var (
	treeGroupColor = color.NRGBA{R: 0xff, G: 0xd0, B: 0x1a, A: 0xff}
	treeRootColor  = color.NRGBA{R: 0xf2, G: 0x6b, B: 0x1d, A: 0xff}
	treeSSHColor   = color.NRGBA{R: 0x2e, G: 0x9b, B: 0x36, A: 0xff}
	treeOtherColor = color.NRGBA{R: 0xe0, G: 0x1b, B: 0x24, A: 0xff}
	favColor       = color.NRGBA{R: 0xf5, G: 0xa6, B: 0x23, A: 0xff}
)

// rootIcon is the web GUI's root icon: an orange disc with a white network
// in it (16px).
func rootIcon(gtx layout.Context, th *Theme) layout.Dimensions {
	sz := gtx.Dp(16)
	d := gtx.Dp(1)
	paint.FillShape(gtx.Ops, treeRootColor, clip.Ellipse{Min: image.Pt(d, d), Max: image.Pt(sz-d, sz-d)}.Op(gtx.Ops))
	g := gtx
	g.Constraints = layout.Exact(image.Pt(sz, sz))
	layout.Center.Layout(g, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return th.Icon(gtx, '\uf0e8', unit.Sp(8), color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) // nf-fa-sitemap
	})
	return layout.Dimensions{Size: image.Pt(sz, sz)}
}

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
		case t.drag.active && t.drag.targetOn && n.ID == t.drag.target:
			// The drop target of a drag (a group or the root).
			FillRect(gtx, image.Rectangle{Max: size}, faded(p.Accent, 0.35))
		case selected:
			FillRect(gtx, image.Rectangle{Max: size}, p.SelectedBg)
		case hovered:
			FillRect(gtx, image.Rectangle{Max: size}, p.RowHover)
		}
		if t.drag.active {
			switch {
			case n.Group && t.drag.desc != nil && !t.drag.desc[n.ID] && !containsID(t.drag.ids, n.ID):
				pointer.CursorGrab.Add(gtx.Ops)
			default:
				pointer.CursorNotAllowed.Add(gtx.Ops)
			}
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
			if n == t.Root {
				return rootIcon(gtx, th)
			}
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
