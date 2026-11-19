package term

import (
	"image/color"
	"io"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func init() { register(VT, newVT) }

// vtEngine wraps charmbracelet/x/vt. The emulator writes its answers to
// queries and the encoded keys into a pipe, which a goroutine forwards to
// Callbacks.Output; it must keep reading, as the emulator blocks on it.
type vtEngine struct {
	mu     sync.Mutex
	em     *vt.Emulator
	colors Colors
	cb     Callbacks
	offset int // lines scrolled up into the history
	// bracketed is set while the program asked for bracketed paste.
	bracketed bool
}

func newVT(o Options) (Engine, error) {
	e := &vtEngine{em: vt.NewEmulator(o.Cols, o.Rows), colors: o.Colors, cb: o.Callbacks}
	e.em.SetScrollbackSize(o.Scrollback)
	e.em.SetDefaultForegroundColor(o.Colors.Fg)
	e.em.SetDefaultBackgroundColor(o.Colors.Bg)
	e.em.SetDefaultCursorColor(o.Colors.Cursor)
	for i := 0; i < 16; i++ {
		e.em.SetIndexedColor(i, o.Colors.Palette[i])
	}
	e.em.SetCallbacks(vt.Callbacks{
		Title: func(s string) {
			if e.cb.Title != nil {
				e.cb.Title(s)
			}
		},
		WorkingDirectory: func(s string) {
			if e.cb.Dir != nil {
				e.cb.Dir(dirFromOSC7(s))
			}
		},
		Bell: func() {
			if e.cb.Bell != nil {
				e.cb.Bell()
			}
		},
		// Called during Feed, with e.mu held.
		EnableMode: func(m ansi.Mode) {
			if m == ansi.ModeBracketedPaste {
				e.bracketed = true
			}
		},
		DisableMode: func(m ansi.Mode) {
			if m == ansi.ModeBracketedPaste {
				e.bracketed = false
			}
		},
	})
	go e.pump()
	return e, nil
}

func (e *vtEngine) pump() {
	buf := make([]byte, 4096)
	for {
		n, err := e.em.Read(buf)
		if n > 0 {
			out := make([]byte, n)
			copy(out, buf[:n])
			e.cb.Output(out)
		}
		if err != nil {
			return
		}
	}
}

func (e *vtEngine) Name() string { return VT }

func (e *vtEngine) Feed(p []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	before := e.em.ScrollbackLen()
	_, _ = e.em.Write(p)
	// Keep the scrolled-back view on the same lines while output arrives.
	if e.offset > 0 {
		e.offset = min(e.offset+e.em.ScrollbackLen()-before, e.em.ScrollbackLen())
	}
}

func (e *vtEngine) Resize(cols, rows, _, _ int) {
	if cols < 1 || rows < 1 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.em.Resize(cols, rows)
	e.offset = min(e.offset, e.em.ScrollbackLen())
}

var vtKeys = map[Key]rune{
	KeyEnter: uv.KeyEnter, KeyTab: uv.KeyTab, KeyBackspace: uv.KeyBackspace,
	KeyEscape: uv.KeyEscape, KeySpace: uv.KeySpace,
	KeyUp: uv.KeyUp, KeyDown: uv.KeyDown, KeyLeft: uv.KeyLeft, KeyRight: uv.KeyRight,
	KeyHome: uv.KeyHome, KeyEnd: uv.KeyEnd, KeyPageUp: uv.KeyPgUp, KeyPageDown: uv.KeyPgDown,
	KeyInsert: uv.KeyInsert, KeyDelete: uv.KeyDelete,
	KeyF1: uv.KeyF1, KeyF2: uv.KeyF2, KeyF3: uv.KeyF3, KeyF4: uv.KeyF4,
	KeyF5: uv.KeyF5, KeyF6: uv.KeyF6, KeyF7: uv.KeyF7, KeyF8: uv.KeyF8,
	KeyF9: uv.KeyF9, KeyF10: uv.KeyF10, KeyF11: uv.KeyF11, KeyF12: uv.KeyF12,
}

func (e *vtEngine) SendKey(ev KeyEvent) {
	k := uv.KeyPressEvent{Code: ev.Rune}
	if ev.Key != KeyNone {
		code, ok := vtKeys[ev.Key]
		if !ok {
			return
		}
		k.Code = code
	}
	if ev.Mods&ModShift != 0 {
		k.Mod |= uv.ModShift
	}
	if ev.Mods&ModCtrl != 0 {
		k.Mod |= uv.ModCtrl
	}
	if ev.Mods&ModAlt != 0 {
		k.Mod |= uv.ModAlt
	}
	// The emulator writes the sequence into its pipe, which pump drains
	// without taking the lock.
	e.mu.Lock()
	defer e.mu.Unlock()
	e.offset = 0
	e.em.SendKey(k)
}

func (e *vtEngine) SendText(text string) {
	e.mu.Lock()
	e.offset = 0
	e.mu.Unlock()
	e.cb.Output([]byte(text))
}

func (e *vtEngine) Paste(text string) {
	// As in Ghostty: without bracketed paste line breaks are sent as Enter
	// (CR); with it they stay, and the end-of-paste sequence is removed so
	// pasted text cannot end the paste.
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bracketed {
		text = strings.ReplaceAll(text, ansi.BracketedPasteEnd, "")
	} else {
		text = strings.ReplaceAll(text, "\r\n", "\r")
		text = strings.ReplaceAll(text, "\n", "\r")
	}
	e.offset = 0
	e.em.Paste(text)
}

func (e *vtEngine) Scroll(delta int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.em.IsAltScreen() {
		return
	}
	e.offset = max(0, min(e.offset-delta, e.em.ScrollbackLen()))
}

func (e *vtEngine) ScrollToBottom() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.offset = 0
}

func (e *vtEngine) Snapshot(f *Frame) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cols, rows := e.em.Width(), e.em.Height()
	f.Fg, f.Bg = e.colors.Fg, e.colors.Bg
	if c := e.em.ForegroundColor(); c != nil {
		f.Fg = toNRGBA(c)
	}
	if c := e.em.BackgroundColor(); c != nil {
		f.Bg = toNRGBA(c)
	}
	f.reset(cols, rows)
	hist := e.em.ScrollbackLen()
	if e.em.IsAltScreen() {
		hist = 0
	}
	offset := min(e.offset, hist)
	f.History, f.Offset = hist, offset

	for y := 0; y < rows; y++ {
		g := hist - offset + y // line index in history+screen
		row := f.Row(y)
		for x := 0; x < cols; x++ {
			var c *uv.Cell
			if g < hist {
				c = e.em.ScrollbackCellAt(x, g)
			} else {
				c = e.em.CellAt(x, g-hist)
			}
			e.convert(c, &row[x], f)
		}
	}

	pos := e.em.CursorPosition()
	style, blink := e.em.CursorStyle()
	f.Cursor = Cursor{X: pos.X, Y: pos.Y + offset, Visible: !e.em.CursorHidden() && pos.Y+offset < rows, Blink: blink}
	switch style {
	case vt.CursorBar:
		f.Cursor.Shape = CursorBar
	case vt.CursorUnderline:
		f.Cursor.Shape = CursorUnderline
	default:
		f.Cursor.Shape = CursorBlock
	}
}

func (e *vtEngine) convert(c *uv.Cell, out *Cell, f *Frame) {
	if c == nil {
		return
	}
	out.Text = c.Content
	if out.Text == " " {
		out.Text = ""
	}
	out.Width = uint8(c.Width)
	if c.Content == "" && c.Width == 0 {
		out.Width = 0
	}
	s := c.Style
	fg, bg := f.Fg, f.Bg
	if s.Fg != nil {
		fg = e.resolve(s.Fg)
	}
	if s.Bg != nil {
		bg = e.resolve(s.Bg)
	}
	if s.Attrs&uv.AttrBold != 0 {
		out.Attrs |= AttrBold
	}
	if s.Attrs&uv.AttrFaint != 0 {
		out.Attrs |= AttrFaint
	}
	if s.Attrs&uv.AttrItalic != 0 {
		out.Attrs |= AttrItalic
	}
	if s.Attrs&(uv.AttrBlink|uv.AttrRapidBlink) != 0 {
		out.Attrs |= AttrBlink
	}
	if s.Attrs&uv.AttrConceal != 0 {
		out.Attrs |= AttrInvisible
	}
	if s.Attrs&uv.AttrStrikethrough != 0 {
		out.Attrs |= AttrStrikethrough
	}
	if s.Underline != uv.UnderlineNone {
		out.Attrs |= AttrUnderline
	}
	if s.Attrs&uv.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	out.Fg, out.Bg = fg, bg
}

func (e *vtEngine) resolve(c color.Color) color.NRGBA {
	switch v := c.(type) {
	case ansi.BasicColor:
		return e.colors.Palette[v]
	case ansi.IndexedColor:
		return e.colors.Palette[v]
	}
	return toNRGBA(c)
}

// Close ends the output pump. Emulator.Close is not used: it sets a field
// that Read checks without synchronisation; closing the pipe ends Read as well.
func (e *vtEngine) Close() {
	if pw, ok := e.em.InputPipe().(*io.PipeWriter); ok {
		_ = pw.CloseWithError(io.EOF)
	}
}
