package termview

import (
	"image"
	"strings"
	"sync"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// fakeEngine records the input the view sends.
type fakeEngine struct {
	mu      sync.Mutex
	log     []string
	resized [2]int
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
}
func (f *fakeEngine) Close() {}

func (f *fakeEngine) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.log
	f.log = nil
	return out
}

type harness struct {
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
