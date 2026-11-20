package termview

import (
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// fakeEngine records the input the view sends; lines is its screen text
// (no history), modes and links what the program set.
type fakeEngine struct {
	mu      sync.Mutex
	log     []string
	resized [2]int
	lines   []string
	modes   term.Modes
	links   map[image.Point]string
}

func (f *fakeEngine) rec(s string) { f.mu.Lock(); f.log = append(f.log, s); f.mu.Unlock() }

func (f *fakeEngine) Name() string { return "fake" }
func (f *fakeEngine) Feed([]byte)  {}
func (f *fakeEngine) Resize(c, r, _, _ int) {
	f.resized = [2]int{c, r}
}
func (f *fakeEngine) SendKey(ev term.KeyEvent) {
	var sb strings.Builder
	sb.WriteString("key:")
	if ev.Mods&term.ModCtrl != 0 {
		sb.WriteString("ctrl+")
	}
	if ev.Mods&term.ModAlt != 0 {
		sb.WriteString("alt+")
	}
	if ev.Mods&term.ModShift != 0 {
		sb.WriteString("shift+")
	}
	if ev.Key != term.KeyNone {
		sb.WriteString([]string{term.KeyEnter: "enter", term.KeyTab: "tab", term.KeySpace: "space", term.KeyUp: "up"}[ev.Key])
	} else {
		sb.WriteRune(ev.Rune)
	}
	f.rec(sb.String())
}
func (f *fakeEngine) SendText(s string) { f.rec("text:" + s) }
func (f *fakeEngine) Paste(s string)    { f.rec("paste:" + s) }
func (f *fakeEngine) Scroll(d int) {
	if d < 0 {
		f.rec("scroll:up")
	} else {
		f.rec("scroll:down")
	}
}
func (f *fakeEngine) ScrollToBottom() {}
func (f *fakeEngine) Snapshot(fr *term.Frame) {
	fr.Fg, fr.Bg = term.DefaultColors().Fg, term.DefaultColors().Bg
	c, r := max(f.resized[0], 1), max(f.resized[1], 1)
	fr.Cols, fr.Rows = c, r
	fr.Cells = make([]term.Cell, c*r)
	for y := 0; y < r; y++ {
		copy(fr.Cells[y*c:(y+1)*c], f.Line(y, nil))
	}
}
func (f *fakeEngine) Modes() term.Modes { return f.modes }
func (f *fakeEngine) Lines() int        { return max(f.resized[1], 1) }
func (f *fakeEngine) Line(row int, dst []term.Cell) []term.Cell {
	s := ""
	if row >= 0 && row < len(f.lines) {
		s = f.lines[row]
	}
	cells := cellsOf(s, max(f.resized[0], 1))
	for i := range cells {
		cells[i].Fg, cells[i].Bg = term.DefaultColors().Fg, term.DefaultColors().Bg
	}
	return append(dst, cells...)
}
func (f *fakeEngine) LinkAt(x, y int) string { return f.links[image.Pt(x, y)] }
func (f *fakeEngine) SendRaw(p []byte)       { f.rec("raw:" + string(p)) }
func (f *fakeEngine) Reset()                 {}
func (f *fakeEngine) Clear()                 {}
func (f *fakeEngine) Close()                 {}

func (f *fakeEngine) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.log
	f.log = nil
	return out
}

type harness struct {
	now time.Time
	r   input.Router
	ops op.Ops
	sh  *text.Shaper
	v   *View
	eng *fakeEngine
}

func newHarness() *harness {
	h := &harness{eng: &fakeEngine{}, sh: text.NewShaper(text.NoSystemFonts(), text.WithCollection(fonts.Collection()))}
	h.v = New(h.eng)
	return h
}

func (h *harness) frame() {
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Constraints: layout.Exact(image.Pt(800, 400)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Source:      h.r.Source(),
		Now:         h.now,
	}
	h.v.Layout(gtx, h.sh)
	h.r.Frame(&h.ops)
}

func TestKeyboardInput(t *testing.T) {
	h := newHarness()
	h.frame()
	h.r.Queue(key.FocusEvent{Focus: true})
	h.frame()
	h.v.Focus(layout.Context{Ops: &h.ops, Source: h.r.Source()})
	h.frame()

	steps := []struct {
		events []key.Event
		edit   string
		want   []string
	}{
		// Typed characters arrive as key events and as text: only the text counts.
		{[]key.Event{{Name: "A", State: key.Press}}, "a", []string{"text:a"}},
		{[]key.Event{{Name: key.NameSpace, State: key.Press}}, " ", []string{"text: "}},
		{[]key.Event{{Name: "C", Modifiers: key.ModCtrl, State: key.Press}}, "", []string{"key:ctrl+c"}},
		{[]key.Event{{Name: key.NameSpace, Modifiers: key.ModCtrl, State: key.Press}}, "", []string{"key:ctrl+space"}},
		{[]key.Event{{Name: "X", Modifiers: key.ModAlt, State: key.Press}}, "", []string{"key:alt+x"}},
		{[]key.Event{{Name: key.NameReturn, State: key.Press}, {Name: key.NameReturn, State: key.Release}}, "", []string{"key:enter"}},
		{[]key.Event{{Name: key.NameUpArrow, State: key.Press}}, "", []string{"key:up"}},
		{[]key.Event{{Name: key.NameTab, State: key.Press}}, "", []string{"key:tab"}},
		{[]key.Event{{Name: key.NamePageUp, Modifiers: key.ModShift, State: key.Press}}, "", []string{"scroll:up"}},
	}
	for i, s := range steps {
		for _, e := range s.events {
			h.r.Queue(e)
		}
		if s.edit != "" {
			h.r.Queue(key.EditEvent{Text: s.edit})
		}
		h.frame()
		got := h.eng.take()
		if strings.Join(got, "|") != strings.Join(s.want, "|") {
			t.Errorf("step %d (%v %q): engine got %q, want %q", i, s.events[0].Name, s.edit, got, s.want)
		}
	}
}

func TestGridFollowsSize(t *testing.T) {
	h := newHarness()
	h.frame()
	cols, rows := h.eng.resized[0], h.eng.resized[1]
	if cols < 60 || rows < 15 {
		t.Fatalf("grid %dx%d for 800x400 px", cols, rows)
	}
	if h.v.cellW <= 0 || h.v.cellH <= h.v.cellW {
		t.Fatalf("cell %dx%d", h.v.cellW, h.v.cellH)
	}
}

// at is the pixel position of the middle of a viewport cell.
func (h *harness) at(col, row int) f32.Point {
	return f32.Pt(float32(6+col*h.v.cellW+h.v.cellW/2), float32(4+row*h.v.cellH+h.v.cellH/2))
}

func (h *harness) mouse(kind pointer.Kind, p f32.Point, b pointer.Buttons, mods key.Modifiers) {
	h.r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: p, Buttons: b, Modifiers: mods})
}

func (h *harness) click(p f32.Point, b pointer.Buttons, mods key.Modifiers) {
	h.mouse(pointer.Move, p, 0, mods)
	h.mouse(pointer.Press, p, b, mods)
	h.mouse(pointer.Release, p, 0, mods)
	h.now = h.now.Add(100 * time.Millisecond)
	h.frame()
}

func TestMouseSelection(t *testing.T) {
	h := newHarness()
	h.eng.lines = []string{"hello world", "/usr/local/bin and more"}
	h.now = time.Now()
	h.frame()
	// Drag from "h" to "o".
	h.mouse(pointer.Move, h.at(0, 0), 0, 0)
	h.mouse(pointer.Press, h.at(0, 0), pointer.ButtonPrimary, 0)
	h.frame()
	h.mouse(pointer.Move, h.at(4, 0), pointer.ButtonPrimary, 0) // a Drag while pressed
	h.mouse(pointer.Release, h.at(4, 0), 0, 0)
	h.frame()
	if got := h.v.SelectionText(); got != "hello" {
		t.Fatalf("drag selected %q", got)
	}
	// A double click selects a word with its path characters.
	h.now = h.now.Add(time.Second)
	h.click(h.at(5, 1), pointer.ButtonPrimary, 0)
	h.click(h.at(5, 1), pointer.ButtonPrimary, 0)
	if got := h.v.SelectionText(); got != "/usr/local/bin" {
		t.Fatalf("double click selected %q", got)
	}
	// A third click selects the line.
	h.click(h.at(5, 1), pointer.ButtonPrimary, 0)
	if got := h.v.SelectionText(); got != "/usr/local/bin and more" {
		t.Fatalf("triple click selected %q", got)
	}
	// Typing ends the selection.
	h.r.Queue(key.EditEvent{Text: "x"})
	h.frame()
	if h.v.SelectionText() != "" {
		t.Fatal("selection kept after typing")
	}
}

func TestMouseReportsAndWheel(t *testing.T) {
	h := newHarness()
	h.now = time.Now()
	h.frame()
	h.eng.modes = term.Modes{Mouse: term.MouseNormal, SGR: true}
	h.click(h.at(2, 1), pointer.ButtonPrimary, 0)
	if got := strings.Join(h.eng.take(), "|"); got != "raw:\x1b[<0;3;2M|raw:\x1b[<0;3;2m" {
		t.Fatalf("click reported as %q", got)
	}
	if h.v.SelectionText() != "" {
		t.Fatal("a reported click selected")
	}
	// Shift selects even while the program tracks the mouse.
	h.eng.lines = []string{"abc"}
	h.click(h.at(1, 0), pointer.ButtonPrimary, key.ModShift)
	if got := h.eng.take(); len(got) != 0 {
		t.Fatalf("shift+click reported: %q", got)
	}
	// The wheel is reported too.
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: h.at(0, 0), Scroll: f32.Pt(0, -float32(h.v.cellH))})
	h.frame()
	if got := strings.Join(h.eng.take(), "|"); got != "raw:\x1b[<64;1;1M" {
		t.Fatalf("wheel reported as %q", got)
	}
	// On the alternate screen without tracking the wheel sends arrow keys.
	h.eng.modes = term.Modes{AltScreen: true}
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: h.at(0, 0), Scroll: f32.Pt(0, -2*float32(h.v.cellH))})
	h.frame()
	if got := strings.Join(h.eng.take(), "|"); got != "key:up|key:up" {
		t.Fatalf("alternate screen wheel: %q", got)
	}
	// Else it scrolls the history.
	h.eng.modes = term.Modes{}
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: h.at(0, 0), Scroll: f32.Pt(0, float32(h.v.cellH))})
	h.frame()
	if got := strings.Join(h.eng.take(), "|"); got != "scroll:down" {
		t.Fatalf("wheel: %q", got)
	}
}

func TestLinksContextMenuAndZoom(t *testing.T) {
	h := newHarness()
	h.now = time.Now()
	h.eng.lines = []string{"see https://example.org/a. ok", "osc link"}
	h.eng.links = map[image.Point]string{{X: 1, Y: 1}: "https://osc.example/"}
	var opened []string
	h.v.OnOpenLink = func(u string) { opened = append(opened, u) }
	var menu []image.Point
	h.v.OnContextMenu = func(p image.Point) { menu = append(menu, p) }
	h.frame()
	h.click(h.at(10, 0), pointer.ButtonPrimary, key.ModCtrl)
	h.click(h.at(1, 1), pointer.ButtonPrimary, key.ModCtrl)
	h.click(h.at(1, 0), pointer.ButtonPrimary, key.ModCtrl) // no link there
	if strings.Join(opened, " ") != "https://example.org/a https://osc.example/" {
		t.Fatalf("opened %q", opened)
	}
	h.click(h.at(3, 3), pointer.ButtonSecondary, 0)
	if len(menu) != 1 {
		t.Fatalf("context menu calls %v", menu)
	}
	// Ctrl+= zooms in, Ctrl+0 resets.
	h.frame()
	h.r.Queue(key.FocusEvent{Focus: true})
	h.v.Focus(layout.Context{Ops: &h.ops, Source: h.r.Source()})
	h.frame()
	w := h.v.cellW
	for i := 0; i < 4; i++ {
		h.r.Queue(key.Event{Name: "=", Modifiers: key.ModCtrl, State: key.Press})
	}
	h.frame()
	if h.v.cellW <= w {
		t.Fatalf("cell width %d after zooming in, was %d", h.v.cellW, w)
	}
	h.r.Queue(key.Event{Name: "0", Modifiers: key.ModCtrl, State: key.Press})
	h.frame()
	if h.v.cellW != w {
		t.Fatalf("cell width %d after reset, want %d", h.v.cellW, w)
	}
}

func TestScreenshot(t *testing.T) {
	h := newHarness()
	h.eng.lines = []string{"hello"}
	h.frame()
	img, err := h.v.Screenshot()
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < h.eng.resized[0]*h.v.cellW || b.Dy() < h.eng.resized[1]*h.v.cellH {
		t.Fatalf("image %v for a %dx%d grid", b, h.eng.resized[0], h.eng.resized[1])
	}
	// Some pixel of the first cell is drawn in the text color.
	bg := term.DefaultColors().Bg
	lit := false
	for y := 4; y < 4+h.v.cellH; y++ {
		for x := 6; x < 6+h.v.cellW; x++ {
			if r, g, bb, _ := img.At(x, y).RGBA(); uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(bb>>8) != bg.B {
				lit = true
			}
		}
	}
	if !lit {
		t.Fatal("no text drawn in the first cell")
	}
}
