package gtk

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ui drives widgets headless through Gio's input router.
type ui struct {
	t    *testing.T
	r    input.Router
	ops  op.Ops
	th   *Theme
	size image.Point
	now  time.Time
}

func newUI(t *testing.T) *ui {
	return &ui{t: t, th: New("dark"), size: image.Pt(800, 600), now: time.Now()}
}

func (u *ui) frame(w layout.Widget) {
	u.ops.Reset()
	u.now = u.now.Add(500 * time.Millisecond) // clicks far apart: no double clicks
	gtx := layout.Context{Ops: &u.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: u.size}, Source: u.r.Source(), Now: u.now}
	w(gtx)
	u.r.Frame(&u.ops)
}

func (u *ui) pointer(kind pointer.Kind, x, y float32, b pointer.Buttons) {
	u.r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: f32.Pt(x, y), Buttons: b, Time: time.Duration(u.now.UnixNano())})
}

// click presses and releases the primary button at (x, y), then draws w.
func (u *ui) click(w layout.Widget, x, y float32) {
	u.pointer(pointer.Move, x, y, 0)
	u.pointer(pointer.Press, x, y, pointer.ButtonPrimary)
	u.pointer(pointer.Release, x, y, 0)
	u.frame(w)
}

func (u *ui) press(w layout.Widget, x, y float32, b pointer.Buttons) {
	u.pointer(pointer.Move, x, y, 0)
	u.pointer(pointer.Press, x, y, b)
	u.pointer(pointer.Release, x, y, 0)
	u.frame(w)
}

func (u *ui) keys(w layout.Widget, names ...key.Name) {
	for _, n := range names {
		u.r.Queue(key.Event{Name: n, State: key.Press}, key.Event{Name: n, State: key.Release})
		u.frame(w)
	}
}

func TestButtonClicksUnlessDisabled(t *testing.T) {
	u := newUI(t)
	var c widget.Clickable
	b := Button{Label: "OK"}
	clicked := 0
	w := func(gtx layout.Context) layout.Dimensions {
		if c.Clicked(gtx) {
			clicked++
		}
		return b.Layout(gtx, u.th, &c)
	}
	u.frame(w)
	u.click(w, 10, 10)
	u.frame(w)
	if clicked != 1 {
		t.Fatalf("clicked %d times", clicked)
	}
	b.Disabled = true
	u.frame(w)
	u.click(w, 10, 10)
	u.frame(w)
	if clicked != 1 {
		t.Fatal("a disabled button was clicked")
	}
}

func TestCheckAndRadio(t *testing.T) {
	u := newUI(t)
	var b widget.Bool
	var e widget.Enum
	e.Value = "a"
	opts := []RadioOption{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	w := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.th.CheckButton(gtx, &b, "Check", false) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.th.RadioGroup(gtx, &e, opts, true, false) }),
		)
	}
	u.frame(w)
	u.click(w, 6, 10)
	if !b.Value {
		t.Fatal("check button not checked")
	}
	// The second radio row (21px check row, 17px radio row, 4px gap).
	u.click(w, 6, 21+17+4+8)
	if e.Value != "b" {
		t.Fatalf("radio value %q", e.Value)
	}
}

func TestComboOpensAndPicks(t *testing.T) {
	u := newUI(t)
	c := Combo{Value: "ssh"}
	opts := []ComboOption{{"ssh", "SSH"}, {"telnet", "Telnet"}, {"sftp", "SFTP"}}
	w := func(gtx layout.Context) layout.Dimensions { return c.Layout(gtx, u.th, opts, false) }
	u.frame(w)
	u.click(w, 10, 10)
	if !c.open {
		t.Fatal("list not open")
	}
	// The list starts below the 26px box; rows are about 20px.
	u.click(w, 10, 26+2+20+10)
	if c.Value != "telnet" || !c.Changed() || c.open {
		t.Fatalf("value %q open %v", c.Value, c.open)
	}
	// Up/Down change the value while the box has the focus.
	u.keys(w, key.NameDownArrow)
	if c.Value != "sftp" {
		t.Fatalf("after Down: %q", c.Value)
	}
	// A click outside closes an open list.
	u.click(w, 10, 10)
	u.click(w, 700, 500)
	if c.open {
		t.Fatal("list still open after a click outside")
	}
}

func TestSpinButtonStepsAndClamps(t *testing.T) {
	u := newUI(t)
	s := SpinButton{Value: 22, Min: 1, Max: 100, Step: 1}
	w := func(gtx layout.Context) layout.Dimensions { return s.Layout(gtx, u.th, false) }
	u.frame(w)
	u.click(w, 10, 10)
	u.keys(w, key.NameUpArrow, key.NameUpArrow)
	if s.Value != 24 || !s.Changed() {
		t.Fatalf("after Up Up: %v", s.Value)
	}
	s.editor.SetText("5000")
	u.keys(w, key.NameReturn)
	if s.Value != 100 || s.editor.Text() != "100" {
		t.Fatalf("not clamped: %v %q", s.Value, s.editor.Text())
	}
	s.editor.SetText("abc")
	u.keys(w, key.NameReturn)
	if s.Value != 100 {
		t.Fatalf("invalid input changed the value: %v", s.Value)
	}
}

func TestMenuKeyboardMouseAndSubmenu(t *testing.T) {
	u := newUI(t)
	m := Menu{Items: []MenuItem{
		{ID: "a", Label: "Connect"},
		{Separator: true},
		{ID: "b", Label: "Disabled", Disabled: true},
		{ID: "c", Label: "Edit"},
		{ID: "d", Label: "More", Submenu: []MenuItem{{ID: "e", Label: "Sub"}}},
	}}
	w := func(gtx layout.Context) layout.Dimensions { return m.Layout(gtx, u.th) }
	m.Open(image.Pt(10, 10))
	u.frame(w)
	// Down skips the separator and the disabled item.
	u.keys(w, key.NameDownArrow, key.NameDownArrow, key.NameReturn)
	if id, ok := m.Activated(); !ok || id != "c" || m.Opened() {
		t.Fatalf("activated %q %v, open %v", id, ok, m.Opened())
	}
	// Mouse: the first row is at 10+4..10+4+24.
	m.Open(image.Pt(10, 10))
	u.frame(w)
	u.click(w, 60, 10+4+12)
	if id, _ := m.Activated(); id != "a" {
		t.Fatalf("click activated %q", id)
	}
	// Submenu by keyboard: Right opens it, Return picks its item (with the
	// pointer away; entering a row moves the focus).
	u.pointer(pointer.Move, 390, 390, 0)
	m.Open(image.Pt(10, 10))
	u.frame(w)
	u.keys(w, key.NameUpArrow, key.NameRightArrow)
	if m.sub == nil {
		t.Fatal("submenu not open")
	}
	u.keys(w, key.NameReturn)
	if id, _ := m.Activated(); id != "e" {
		t.Fatalf("submenu activated %q", id)
	}
	// Escape and clicks outside close.
	m.Open(image.Pt(10, 10))
	u.frame(w)
	u.keys(w, key.NameEscape)
	if m.Opened() {
		t.Fatal("Escape did not close")
	}
	m.Open(image.Pt(10, 10))
	u.frame(w)
	u.click(w, 700, 500)
	if m.Opened() {
		t.Fatal("a click outside did not close")
	}
}

func TestTreeSelectExpandActivate(t *testing.T) {
	u := newUI(t)
	tr := Tree{Root: &TreeNode{ID: "root", Name: "My Connections", Group: true, Children: []*TreeNode{
		{ID: "g", Name: "Group", Group: true, Children: []*TreeNode{{ID: "c1", Name: "inner", Method: "ssh"}}},
		{ID: "c2", Name: "web", Method: "ssh"},
	}}, Expanded: map[string]bool{"root": true}}
	u.size = image.Pt(300, 200)
	w := func(gtx layout.Context) layout.Dimensions { return tr.Layout(gtx, u.th) }
	u.frame(w)
	row := func(i int) float32 { return float32(1 + 22*i + 11) }
	// Click "web" (row 2): selected.
	u.click(w, 100, row(2))
	if !tr.Selected["c2"] || len(tr.Selected) != 1 {
		t.Fatalf("selection %v", tr.Selected)
	}
	// The expander of "Group" (row 1, depth 1: x = 1+16+4+7).
	u.click(w, 1+16+4+7, row(1))
	if !tr.Expanded["g"] {
		t.Fatal("group not expanded")
	}
	// Keyboard: Up from "web" (now row 3) goes to "inner".
	u.click(w, 100, row(3))
	u.keys(w, key.NameUpArrow)
	if !tr.Selected["c1"] {
		t.Fatalf("after Up: %v", tr.Selected)
	}
	tr.Events()
	u.keys(w, key.NameReturn)
	found := false
	for _, e := range tr.Events() {
		found = found || (e.Kind == TreeActivate && e.ID == "c1")
	}
	if !found {
		t.Fatal("Enter did not activate the connection")
	}
	// Right click: context event.
	u.press(w, 100, row(3), pointer.ButtonSecondary)
	found = false
	for _, e := range tr.Events() {
		found = found || (e.Kind == TreeContextClick && e.ID == "c2")
	}
	if !found {
		t.Fatal("no context click event")
	}
}

func TestNotebookEvents(t *testing.T) {
	u := newUI(t)
	n := Notebook{Tabs: []*Tab{{Title: "One"}, {Title: "Two", Closable: true}}}
	w := func(gtx layout.Context) layout.Dimensions { return n.Layout(gtx, u.th) }
	u.frame(w)
	// "One" is about 45px wide; "Two" follows with its close button.
	u.click(w, 60, 13)
	ev := n.Events()
	if n.Selected != 1 || len(ev) != 1 || ev[0].Kind != TabSelected {
		t.Fatalf("selected %d, events %v", n.Selected, ev)
	}
	// The close button near the right end of "Two".
	closed := false
	for x := float32(70); x < 130 && !closed; x += 2 {
		u.click(w, x, 13)
		for _, e := range n.Events() {
			closed = closed || (e.Kind == TabClosed && e.Tab == 1)
		}
	}
	if !closed {
		t.Fatal("the close button never closed the tab")
	}
	u.press(w, 15, 13, pointer.ButtonTertiary)
	ev = n.Events()
	if len(ev) != 1 || ev[0].Kind != TabMiddleClick || ev[0].Tab != 0 {
		t.Fatalf("middle click events %v", ev)
	}
}

func TestPanedDrag(t *testing.T) {
	u := newUI(t)
	p := Paned{Pos: 200}
	nop := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Max} }
	w := func(gtx layout.Context) layout.Dimensions { return p.Layout(gtx, u.th, nop, nop) }
	u.frame(w)
	u.pointer(pointer.Move, 203, 100, 0)
	u.pointer(pointer.Press, 203, 100, pointer.ButtonPrimary)
	u.frame(w)
	// A move with the button held is a drag.
	u.pointer(pointer.Move, 253, 100, pointer.ButtonPrimary)
	u.frame(w)
	u.pointer(pointer.Move, 283, 100, pointer.ButtonPrimary)
	u.pointer(pointer.Release, 283, 100, 0)
	u.frame(w)
	if p.Pos != 280 {
		t.Fatalf("handle at %d after dragging 80px", p.Pos)
	}
}

func TestListBoxKeys(t *testing.T) {
	u := newUI(t)
	items := []string{"a", "b", "c", "d"}
	l := ListBox{}
	u.size = image.Pt(200, 200)
	w := func(gtx layout.Context) layout.Dimensions {
		return l.Layout(gtx, u.th, len(items), func(i int) string { return items[i] })
	}
	u.frame(w)
	u.click(w, 20, 1+22+11)
	if l.Selected != 1 || !l.Changed() {
		t.Fatalf("selected %d", l.Selected)
	}
	u.keys(w, key.NameDownArrow, key.NameEnd)
	if l.Selected != 3 {
		t.Fatalf("after Down End: %d", l.Selected)
	}
	u.keys(w, key.NameHome)
	if l.Selected != 0 {
		t.Fatalf("after Home: %d", l.Selected)
	}
}

func TestMessageDialogAnswers(t *testing.T) {
	u := newUI(t)
	m := MessageDialog{Title: "Quit", Primary: "Close?", Buttons: ButtonsYesNo}
	w := func(gtx layout.Context) layout.Dimensions { return m.Layout(gtx, u.th) }
	u.frame(w)
	u.keys(w, key.NameEscape)
	if r, ok := m.Response(); !ok || r != "cancel" {
		t.Fatalf("Escape gave %q %v", r, ok)
	}
}

func TestColorChooserHexEntry(t *testing.T) {
	u := newUI(t)
	var cc ColorChooser
	cc.Open(color.NRGBA{A: 255})
	w := func(gtx layout.Context) layout.Dimensions { return cc.Layout(gtx, u.th) }
	u.frame(w)
	cc.hex.SetText("#4a90d9")
	u.frame(w)
	gtxFocus := func(gtx layout.Context) layout.Dimensions {
		gtx.Execute(key.FocusCmd{Tag: &cc.hex})
		return cc.Layout(gtx, u.th)
	}
	u.frame(gtxFocus)
	u.keys(w, key.NameReturn)
	c, ok, done := cc.Result()
	if !done || !ok || c != (color.NRGBA{R: 0x4a, G: 0x90, B: 0xd9, A: 0xff}) {
		t.Fatalf("result %v ok %v done %v", c, ok, done)
	}
	if _, ok := ParseHex("#12345"); ok {
		t.Fatal("ParseHex accepted 5 digits")
	}
}

func TestTreeDragDropMovesOntoGroup(t *testing.T) {
	u := newUI(t)
	tr := Tree{Root: &TreeNode{ID: "root", Name: "My Connections", Group: true, Children: []*TreeNode{
		{ID: "g", Name: "Group", Group: true, Children: []*TreeNode{{ID: "c1", Name: "inner", Method: "ssh"}}},
		{ID: "c2", Name: "web", Method: "ssh"},
	}}, Expanded: map[string]bool{"root": true, "g": true}}
	u.size = image.Pt(300, 200)
	w := func(gtx layout.Context) layout.Dimensions { return tr.Layout(gtx, u.th) }
	u.frame(w)
	row := func(i int) float32 { return float32(1 + 22*i + 11) }
	// Rows: 0 root, 1 Group, 2 inner, 3 web. Drag "web" onto "Group".
	u.pointer(pointer.Move, 100, row(3), 0)
	u.pointer(pointer.Press, 100, row(3), pointer.ButtonPrimary)
	u.frame(w)
	for y := row(3); y >= row(1); y -= 6 {
		u.pointer(pointer.Move, 100, y, pointer.ButtonPrimary)
		u.frame(w)
	}
	u.pointer(pointer.Release, 100, row(1), 0)
	u.frame(w)
	found := false
	for _, e := range tr.Events() {
		if e.Kind == TreeDrop && e.ID == "g" && len(e.IDs) == 1 && e.IDs[0] == "c2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no drop event onto the group: %v", tr.Events())
	}
}

func TestTreeDragDropRejectsConnectionTargets(t *testing.T) {
	u := newUI(t)
	tr := Tree{Root: &TreeNode{ID: "root", Name: "My Connections", Group: true, Children: []*TreeNode{
		{ID: "c1", Name: "inner", Method: "ssh"},
		{ID: "c2", Name: "web", Method: "ssh"},
	}}, Expanded: map[string]bool{"root": true}}
	u.size = image.Pt(300, 200)
	w := func(gtx layout.Context) layout.Dimensions { return tr.Layout(gtx, u.th) }
	u.frame(w)
	row := func(i int) float32 { return float32(1 + 22*i + 11) }
	// Rows: 0 root, 1 inner, 2 web. Drag "web" onto "inner": no drop.
	u.pointer(pointer.Move, 100, row(2), 0)
	u.pointer(pointer.Press, 100, row(2), pointer.ButtonPrimary)
	u.frame(w)
	for y := row(2); y >= row(1); y -= 6 {
		u.pointer(pointer.Move, 100, y, pointer.ButtonPrimary)
		u.frame(w)
	}
	u.pointer(pointer.Release, 100, row(1), 0)
	u.frame(w)
	for _, e := range tr.Events() {
		if e.Kind == TreeDrop {
			t.Fatalf("drop onto a connection was accepted: %v", e)
		}
	}
	// And the press must not have selected "web" (it was a drag).
	if tr.Selected["c2"] {
		t.Fatal("the drag selected the row")
	}
}

func TestNotebookReorder(t *testing.T) {
	u := newUI(t)
	n := Notebook{Reorderable: true, Tabs: []*Tab{{Title: "AAA"}, {Title: "BBB"}, {Title: "CCC"}}}
	w := func(gtx layout.Context) layout.Dimensions { return n.Layout(gtx, u.th) }
	u.frame(w)
	if len(n.rects) != 3 {
		t.Fatalf("rects %v", n.rects)
	}
	right := float32(n.rects[2].Max.X - 4)
	u.pointer(pointer.Move, right, 13, 0)
	u.pointer(pointer.Press, right, 13, pointer.ButtonPrimary)
	u.frame(w)
	for x := right; x >= 2; x -= 8 {
		u.pointer(pointer.Move, x, 13, pointer.ButtonPrimary)
		u.frame(w)
	}
	u.pointer(pointer.Release, 2, 13, 0)
	u.frame(w)
	var moved *NotebookEvent
	for _, e := range n.Events() {
		e := e
		if e.Kind == TabMoved {
			moved = &e
		}
	}
	if moved == nil || moved.From != 2 || moved.To != 0 {
		t.Fatalf("TabMoved %+v, events above", moved)
	}
	if n.Tabs[0].Title != "CCC" || n.Tabs[1].Title != "AAA" || n.Tabs[2].Title != "BBB" {
		t.Fatalf("tab order %v %v %v", n.Tabs[0].Title, n.Tabs[1].Title, n.Tabs[2].Title)
	}
}

func TestTextAreaTypesAndTakesMax(t *testing.T) {
	u := newUI(t)
	var e widget.Editor
	ta := TextArea{Placeholder: "Notes", Mono: true}
	w := func(gtx layout.Context) layout.Dimensions { return ta.Layout(gtx, u.th, &e) }
	u.size = image.Pt(320, 120)
	u.frame(w)
	d := func() layout.Dimensions {
		u.ops.Reset()
		gtx := layout.Context{Ops: &u.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: u.size}, Source: u.r.Source(), Now: u.now}
		return ta.Layout(gtx, u.th, &e)
	}()
	if d.Size != u.size {
		t.Fatalf("text area size %v, want %v", d.Size, u.size)
	}
	u.click(w, 100, 60)
	u.r.Queue(key.EditEvent{Text: "hello world"})
	u.frame(w)
	if got := e.Text(); got != "hello world" {
		t.Fatalf("text %q", got)
	}
	u.r.Queue(key.EditEvent{Range: key.Range{Start: 11, End: 11}, Text: "\nsecond"})
	u.frame(w)
	if got := e.Text(); got != "hello world\nsecond" {
		t.Fatalf("text %q", got)
	}
}

func TestTextAreaDisabledTakesNoInput(t *testing.T) {
	u := newUI(t)
	var e widget.Editor
	ta := TextArea{Disabled: true}
	w := func(gtx layout.Context) layout.Dimensions { return ta.Layout(gtx, u.th, &e) }
	u.size = image.Pt(320, 120)
	u.frame(w)
	u.click(w, 100, 60)
	u.r.Queue(key.EditEvent{Text: "nope"})
	u.frame(w)
	if got := e.Text(); got != "" {
		t.Fatalf("disabled text area got %q", got)
	}
}

func TestLogoRendersSquare(t *testing.T) {
	u := newUI(t)
	u.size = image.Pt(300, 100)
	var logo Logo
	w := func(gtx layout.Context) layout.Dimensions { return logo.Layout(gtx) }
	u.frame(w)
	gtx := layout.Context{Ops: &u.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: u.size}, Source: u.r.Source(), Now: u.now}
	u.ops.Reset()
	d := logo.Layout(gtx)
	if d.Size != image.Pt(100, 100) {
		t.Fatalf("logo size %v, want 100x100", d.Size)
	}
}
