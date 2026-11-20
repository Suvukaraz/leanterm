package term

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// harness runs one engine and records what it sends to the program.
type harness struct {
	t   *testing.T
	eng Engine

	mu    sync.Mutex
	out   bytes.Buffer
	title string
	dir   string
}

func newHarness(t *testing.T, name string, cols, rows int) *harness {
	t.Helper()
	h := &harness{t: t}
	eng, err := New(name, Options{Cols: cols, Rows: rows, Callbacks: Callbacks{
		Output: func(p []byte) { h.mu.Lock(); h.out.Write(p); h.mu.Unlock() },
		Title:  func(s string) { h.mu.Lock(); h.title = s; h.mu.Unlock() },
		Dir:    func(s string) { h.mu.Lock(); h.dir = s; h.mu.Unlock() },
	}})
	if err != nil {
		t.Fatalf("New(%s): %v", name, err)
	}
	t.Cleanup(eng.Close)
	h.eng = eng
	return h
}

// waitOutput waits until the program received want, then clears the record.
func (h *harness) waitOutput(want string) {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.mu.Lock()
		got := h.out.String()
		if strings.Contains(got, want) {
			h.out.Reset()
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()
		if time.Now().After(deadline) {
			h.t.Fatalf("program got %q, want %q", got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *harness) waitState(what string, get func() string, want string) {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.mu.Lock()
		got := get()
		h.mu.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("%s = %q, want %q", what, got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func rowText(f *Frame, y int) string {
	var sb strings.Builder
	for _, c := range f.Row(y) {
		switch {
		case c.Width == 0:
		case c.Text == "":
			sb.WriteByte(' ')
		default:
			sb.WriteString(c.Text)
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

// forEachEngine runs fn against every engine compiled into this build.
func forEachEngine(t *testing.T, fn func(t *testing.T, name string)) {
	for _, name := range Available() {
		t.Run(name, func(t *testing.T) { fn(t, name) })
	}
}

func TestDefaultEngine(t *testing.T) {
	want := VT
	for _, n := range Available() {
		if n == Ghostty {
			want = Ghostty
		}
	}
	if got := Default(); got != want {
		t.Fatalf("Default() = %q, want %q", got, want)
	}
	if _, err := New("nope", Options{Callbacks: Callbacks{Output: func([]byte) {}}}); err == nil {
		t.Fatal("New(unknown engine) succeeded")
	}
}

func TestTextColorsAndCursor(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 40, 5)
		h.eng.Feed([]byte("Hello \x1b[31mred\x1b[0m \x1b[1;44mx\x1b[0m\r\nab"))
		var f Frame
		h.eng.Snapshot(&f)
		if f.Cols != 40 || f.Rows != 5 {
			t.Fatalf("size %dx%d", f.Cols, f.Rows)
		}
		if got := rowText(&f, 0); got != "Hello red x" {
			t.Fatalf("row 0 = %q", got)
		}
		pal := DefaultColors().Palette
		if c := f.Row(0)[6]; c.Fg != pal[1] || c.Bg != f.Bg {
			t.Fatalf("red cell fg %v bg %v", c.Fg, c.Bg)
		}
		x := f.Row(0)[10]
		if x.Bg != pal[4] || x.Attrs&AttrBold == 0 {
			t.Fatalf("bold blue-background cell %+v", x)
		}
		if f.Row(0)[0].Fg != f.Fg {
			t.Fatalf("default fg %v, frame fg %v", f.Row(0)[0].Fg, f.Fg)
		}
		if c := f.Cursor; c.X != 2 || c.Y != 1 || !c.Visible {
			t.Fatalf("cursor %+v", c)
		}
	})
}

func TestReverseVideo(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 10, 2)
		h.eng.Feed([]byte("\x1b[7mR\x1b[0m"))
		var f Frame
		h.eng.Snapshot(&f)
		if c := f.Row(0)[0]; c.Fg != f.Bg || c.Bg != f.Fg {
			t.Fatalf("reverse cell fg %v bg %v, defaults fg %v bg %v", c.Fg, c.Bg, f.Fg, f.Bg)
		}
	})
}

func TestWideCharacters(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 10, 2)
		h.eng.Feed([]byte("日本x"))
		var f Frame
		h.eng.Snapshot(&f)
		r := f.Row(0)
		if r[0].Text != "日" || r[0].Width != 2 || r[1].Width != 0 || r[2].Text != "本" || r[4].Text != "x" {
			t.Fatalf("cells %+v", r[:5])
		}
		if got := rowText(&f, 0); got != "日本x" {
			t.Fatalf("row = %q", got)
		}
	})
}

func TestKeys(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 80, 24)
		h.eng.SendKey(KeyEvent{Rune: 'c', Mods: ModCtrl})
		h.waitOutput("\x03")
		h.eng.SendKey(KeyEvent{Key: KeyEnter})
		h.waitOutput("\r")
		h.eng.SendKey(KeyEvent{Key: KeyUp})
		h.waitOutput("\x1b[A")
		// Application cursor keys (DECCKM).
		h.eng.Feed([]byte("\x1b[?1h"))
		h.eng.SendKey(KeyEvent{Key: KeyUp})
		h.waitOutput("\x1bOA")
		h.eng.SendKey(KeyEvent{Rune: 'x', Mods: ModAlt})
		h.waitOutput("\x1bx")
		h.eng.SendText("äb")
		h.waitOutput("äb")
	})
}

func TestBracketedPaste(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 80, 24)
		// Without bracketed paste a line break is Enter (CR).
		h.eng.Paste("ls\nid")
		h.waitOutput("ls\rid")
		h.eng.Feed([]byte("\x1b[?2004h"))
		h.eng.Paste("echo 1\necho 2")
		h.waitOutput("\x1b[200~echo 1\necho 2\x1b[201~")
	})
}

func TestQueriesAreAnswered(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 80, 24)
		h.eng.Feed([]byte("abc\x1b[6n")) // cursor position report
		h.waitOutput("\x1b[1;4R")
		h.eng.Feed([]byte("\x1b[c")) // primary device attributes
		h.waitOutput("\x1b[?")
	})
}

func TestTitleAndDirectory(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 80, 24)
		h.eng.Feed([]byte("\x1b]0;my title\x07"))
		h.waitState("title", func() string { return h.title }, "my title")
		h.eng.Feed([]byte("\x1b]7;file://host/srv/www\x1b\\"))
		h.waitState("dir", func() string { return h.dir }, "/srv/www")
	})
}

func TestScrollback(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 20, 5)
		for i := 1; i <= 30; i++ {
			h.eng.Feed([]byte("line " + itoa(i) + "\r\n"))
		}
		var f Frame
		h.eng.Snapshot(&f)
		// The screen shows lines 27..30 and the empty prompt line.
		if got := rowText(&f, 0); got != "line 27" || f.Offset != 0 {
			t.Fatalf("bottom: row 0 = %q, offset %d", got, f.Offset)
		}
		if f.History < 26 {
			t.Fatalf("history %d", f.History)
		}
		h.eng.Scroll(-10)
		h.eng.Snapshot(&f)
		if got := rowText(&f, 0); got != "line 17" || f.Offset != 10 {
			t.Fatalf("scrolled: row 0 = %q, offset %d", got, f.Offset)
		}
		if f.Cursor.Visible {
			t.Fatal("cursor visible while scrolled back")
		}
		h.eng.ScrollToBottom()
		h.eng.Snapshot(&f)
		if got := rowText(&f, 0); got != "line 27" {
			t.Fatalf("back at bottom: row 0 = %q", got)
		}
	})
}

func TestResize(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 20, 5)
		h.eng.Feed([]byte("hello"))
		h.eng.Resize(30, 8, 8, 16)
		var f Frame
		h.eng.Snapshot(&f)
		if f.Cols != 30 || f.Rows != 8 || len(f.Cells) != 240 {
			t.Fatalf("after resize %dx%d (%d cells)", f.Cols, f.Rows, len(f.Cells))
		}
		if got := rowText(&f, 0); got != "hello" {
			t.Fatalf("row 0 = %q", got)
		}
	})
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

func TestLinesIncludeHistory(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 20, 3)
		for i := 1; i <= 6; i++ {
			h.eng.Feed([]byte("line " + string(rune('0'+i)) + "\r\n"))
		}
		var f Frame
		h.eng.Snapshot(&f)
		n := h.eng.Lines()
		if n != f.History+f.Rows {
			t.Fatalf("Lines() = %d, history %d + rows %d", n, f.History, f.Rows)
		}
		// The screen shows lines 5, 6 and the empty last line; the history
		// holds the rest.
		for y := 0; y < f.Rows; y++ {
			if got, want := Text(h.eng.Line(f.History-f.Offset+y, nil)), rowText(&f, y); got != want {
				t.Fatalf("line of viewport row %d = %q, row text %q", y, got, want)
			}
		}
		if got := Text(h.eng.Line(f.History-1, nil)); got != "line 4" {
			t.Fatalf("last history line %q", got)
		}
		if got := len(h.eng.Line(0, nil)); got != 20 {
			t.Fatalf("first line has %d cells", got)
		}
		if got := h.eng.Line(n+5, nil); len(got) != 0 {
			t.Fatalf("line beyond the end: %d cells", len(got))
		}
		// Colors survive in history lines.
		h.eng.Feed([]byte("\x1b[31mred\x1b[0m\r\n\r\n\r\n\r\n"))
		h.eng.Snapshot(&f)
		found := false
		for r := 0; r < h.eng.Lines(); r++ {
			l := h.eng.Line(r, nil)
			if Text(l) == "red" {
				found = l[0].Fg == DefaultColors().Palette[1]
			}
		}
		if !found {
			t.Fatal("red history line not found with its color")
		}
	})
}

func TestModesAndRaw(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 20, 3)
		h.eng.Feed([]byte("\x1b[?1000h\x1b[?1006h"))
		if m := h.eng.Modes(); m.Mouse != MouseNormal || !m.SGR || m.AltScreen {
			t.Fatalf("modes %+v", m)
		}
		h.eng.Feed([]byte("\x1b[?1049h"))
		if !h.eng.Modes().AltScreen {
			t.Fatal("alternate screen not seen")
		}
		if n := h.eng.Lines(); n != 3 {
			t.Fatalf("alternate screen has %d lines", n)
		}
		h.eng.SendRaw([]byte("\x1b[<0;1;1M"))
		h.waitOutput("\x1b[<0;1;1M")
		h.eng.Reset()
		if m := h.eng.Modes(); m.Mouse != MouseNone {
			t.Fatalf("modes after reset %+v", m)
		}
	})
}

func TestHyperlinkAndClear(t *testing.T) {
	forEachEngine(t, func(t *testing.T, name string) {
		h := newHarness(t, name, 30, 3)
		h.eng.Feed([]byte("see \x1b]8;;https://example.org/x\x1b\\here\x1b]8;;\x1b\\ now"))
		if got := h.eng.LinkAt(5, 0); got != "https://example.org/x" {
			t.Fatalf("LinkAt(link) = %q", got)
		}
		if got := h.eng.LinkAt(1, 0); got != "" {
			t.Fatalf("LinkAt(text) = %q", got)
		}
		for i := 0; i < 5; i++ {
			h.eng.Feed([]byte("\r\nmore"))
		}
		h.eng.Clear()
		var f Frame
		h.eng.Snapshot(&f)
		for y := 0; y < f.Rows; y++ {
			if s := rowText(&f, y); s != "" {
				t.Fatalf("row %d after Clear: %q", y, s)
			}
		}
		if f.History != 0 {
			t.Fatalf("history after Clear: %d", f.History)
		}
	})
}
