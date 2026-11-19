//go:build ghostty

package term

import (
	"image/color"
	"io"
	"strings"
	"sync"
	"unicode"

	gt "go.mitchellh.com/libghostty"
)

func init() { register(Ghostty, newGhostty) }

// ghosttyEngine wraps Ghostty's terminal core (libghostty-vt). All calls into
// the library go through mu: the terminal, its render state and the encoders
// are not safe for concurrent use. The write-pty callback runs inside those
// calls and only forwards the bytes to the program.
type ghosttyEngine struct {
	mu    sync.Mutex
	t     *gt.Terminal
	rs    *gt.RenderState
	ri    *gt.RenderStateRowIterator
	rc    *gt.RenderStateRowCells
	keys  *gt.KeyEncoder
	kev   *gt.KeyEvent
	cb    Callbacks
	cellW uint32
	cellH uint32
	gbuf  []uint32
}

func newGhostty(o Options) (Engine, error) {
	e := &ghosttyEngine{cb: o.Callbacks, cellW: 8, cellH: 16}
	t, err := gt.NewTerminal(
		gt.WithSize(uint16(o.Cols), uint16(o.Rows)),
		gt.WithMaxScrollbackLines(uint(o.Scrollback)),
		gt.WithWritePty(func(_ *gt.Terminal, data []byte) {
			out := make([]byte, len(data))
			copy(out, data)
			e.cb.Output(out)
		}),
		gt.WithTitleChanged(func(t *gt.Terminal) {
			if e.cb.Title != nil {
				if s, err := t.Title(); err == nil {
					e.cb.Title(s)
				}
			}
		}),
		gt.WithPwdChanged(func(t *gt.Terminal) {
			if e.cb.Dir != nil {
				if s, err := t.Pwd(); err == nil {
					e.cb.Dir(dirFromOSC7(s))
				}
			}
		}),
		gt.WithBell(func(*gt.Terminal) {
			if e.cb.Bell != nil {
				e.cb.Bell()
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	e.t = t
	fg, bg, cur := rgb(o.Colors.Fg), rgb(o.Colors.Bg), rgb(o.Colors.Cursor)
	var pal gt.Palette
	for i, c := range o.Colors.Palette {
		pal[i] = rgb(c)
	}
	_ = t.SetColorForeground(&fg)
	_ = t.SetColorBackground(&bg)
	_ = t.SetColorCursor(&cur)
	_ = t.SetColorPalette(&pal)

	if e.rs, err = gt.NewRenderState(); err == nil {
		if e.ri, err = gt.NewRenderStateRowIterator(); err == nil {
			if e.rc, err = gt.NewRenderStateRowCells(); err == nil {
				if e.keys, err = gt.NewKeyEncoder(); err == nil {
					e.kev, err = gt.NewKeyEvent()
				}
			}
		}
	}
	if err != nil {
		e.Close()
		return nil, err
	}
	return e, nil
}

func rgb(c color.NRGBA) gt.ColorRGB { return gt.ColorRGB{R: c.R, G: c.G, B: c.B} }

func nrgba(c gt.ColorRGB) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xff} }

func (e *ghosttyEngine) Name() string { return Ghostty }

func (e *ghosttyEngine) Feed(p []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	e.t.VTWrite(p)
}

func (e *ghosttyEngine) Resize(cols, rows, cellW, cellH int) {
	if cols < 1 || rows < 1 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	if cellW > 0 && cellH > 0 {
		e.cellW, e.cellH = uint32(cellW), uint32(cellH)
	}
	_ = e.t.Resize(uint16(cols), uint16(rows), e.cellW, e.cellH)
}

var ghosttyNamed = map[Key]gt.Key{
	KeyEnter: gt.KeyEnter, KeyTab: gt.KeyTab, KeyBackspace: gt.KeyBackspace,
	KeyEscape: gt.KeyEscape, KeySpace: gt.KeySpace,
	KeyUp: gt.KeyArrowUp, KeyDown: gt.KeyArrowDown, KeyLeft: gt.KeyArrowLeft, KeyRight: gt.KeyArrowRight,
	KeyHome: gt.KeyHome, KeyEnd: gt.KeyEnd, KeyPageUp: gt.KeyPageUp, KeyPageDown: gt.KeyPageDown,
	KeyInsert: gt.KeyInsert, KeyDelete: gt.KeyDelete,
	KeyF1: gt.KeyF1, KeyF2: gt.KeyF2, KeyF3: gt.KeyF3, KeyF4: gt.KeyF4,
	KeyF5: gt.KeyF5, KeyF6: gt.KeyF6, KeyF7: gt.KeyF7, KeyF8: gt.KeyF8,
	KeyF9: gt.KeyF9, KeyF10: gt.KeyF10, KeyF11: gt.KeyF11, KeyF12: gt.KeyF12,
}

// ghosttyRunes maps the unshifted characters of a US layout to physical keys,
// which the encoder needs for Ctrl/Alt combinations.
var ghosttyRunes = map[rune]gt.Key{
	'a': gt.KeyA, 'b': gt.KeyB, 'c': gt.KeyC, 'd': gt.KeyD, 'e': gt.KeyE, 'f': gt.KeyF,
	'g': gt.KeyG, 'h': gt.KeyH, 'i': gt.KeyI, 'j': gt.KeyJ, 'k': gt.KeyK, 'l': gt.KeyL,
	'm': gt.KeyM, 'n': gt.KeyN, 'o': gt.KeyO, 'p': gt.KeyP, 'q': gt.KeyQ, 'r': gt.KeyR,
	's': gt.KeyS, 't': gt.KeyT, 'u': gt.KeyU, 'v': gt.KeyV, 'w': gt.KeyW, 'x': gt.KeyX,
	'y': gt.KeyY, 'z': gt.KeyZ,
	'0': gt.KeyDigit0, '1': gt.KeyDigit1, '2': gt.KeyDigit2, '3': gt.KeyDigit3, '4': gt.KeyDigit4,
	'5': gt.KeyDigit5, '6': gt.KeyDigit6, '7': gt.KeyDigit7, '8': gt.KeyDigit8, '9': gt.KeyDigit9,
	'[': gt.KeyBracketLeft, ']': gt.KeyBracketRight, '\\': gt.KeyBackslash, ';': gt.KeySemicolon,
	'\'': gt.KeyQuote, ',': gt.KeyComma, '.': gt.KeyPeriod, '/': gt.KeySlash, '`': gt.KeyBackquote,
	'-': gt.KeyMinus, '=': gt.KeyEqual, ' ': gt.KeySpace,
}

func (e *ghosttyEngine) SendKey(ev KeyEvent) {
	key := gt.KeyUnidentified
	var cp rune
	if ev.Key != KeyNone {
		k, ok := ghosttyNamed[ev.Key]
		if !ok {
			return
		}
		key = k
	} else {
		cp = unicode.ToLower(ev.Rune)
		if k, ok := ghosttyRunes[cp]; ok {
			key = k
		}
	}
	var mods gt.Mods
	if ev.Mods&ModShift != 0 {
		mods |= gt.ModShift
	}
	if ev.Mods&ModCtrl != 0 {
		mods |= gt.ModCtrl
	}
	if ev.Mods&ModAlt != 0 {
		mods |= gt.ModAlt
	}
	if ev.Mods&ModSuper != 0 {
		mods |= gt.ModSuper
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	_ = e.kev.SetAction(gt.KeyActionPress)
	_ = e.kev.SetKey(key)
	_ = e.kev.SetMods(mods)
	_ = e.kev.SetUnshiftedCodepoint(cp)
	// Text is only attached without Ctrl: the encoder derives the control
	// character from the key, and Alt+x is sent as ESC x.
	text := ""
	if ev.Key == KeyNone && mods&gt.ModCtrl == 0 && ev.Rune != 0 {
		text = string(ev.Rune)
	}
	_ = e.kev.SetUTF8(text)
	_ = e.keys.SetFromTerminal(e.t)
	seq, err := e.keys.Encode(e.kev)
	if err != nil || len(seq) == 0 {
		return
	}
	_ = e.t.ScrollViewportBottom()
	e.cb.Output(seq)
}

func (e *ghosttyEngine) SendText(text string) {
	if text == "" {
		return
	}
	e.mu.Lock()
	closed := e.t == nil
	if !closed {
		_ = e.t.ScrollViewportBottom()
	}
	e.mu.Unlock()
	if !closed {
		e.cb.Output([]byte(text))
	}
}

func (e *ghosttyEngine) Paste(text string) {
	if text == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	_ = e.t.ScrollViewportBottom()
	// Confirming risky pastes is up to the GUI; the engine pastes as asked.
	_, _ = e.t.Paste(gt.Paste{
		Location:    gt.ClipboardLocationStandard,
		Source:      gt.PasteSourceText,
		MIMEs:       []string{"text/plain"},
		Reader:      func(_ string, w io.Writer) error { _, err := io.WriteString(w, text); return err },
		AllowUnsafe: true,
	})
}

func (e *ghosttyEngine) Scroll(delta int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	_ = e.t.ScrollViewportDelta(delta)
}

func (e *ghosttyEngine) ScrollToBottom() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	_ = e.t.ScrollViewportBottom()
}

func (e *ghosttyEngine) Snapshot(f *Frame) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.t == nil {
		return
	}
	if err := e.rs.Update(e.t); err != nil {
		return
	}
	cols, _ := e.rs.Cols()
	rows, _ := e.rs.Rows()
	colors, err := e.rs.Colors()
	if err != nil {
		return
	}
	f.Fg, f.Bg = nrgba(colors.Foreground), nrgba(colors.Background)
	f.reset(int(cols), int(rows))
	if sb, err := e.t.Scrollbar(); err == nil && sb.Total >= sb.Len {
		f.History = int(sb.Total - sb.Len)
		f.Offset = f.History - int(sb.Offset)
	}

	if err := e.rs.RowIterator(e.ri); err != nil {
		return
	}
	for y := 0; e.ri.Next() && y < f.Rows; y++ {
		if err := e.ri.Cells(e.rc); err != nil {
			continue
		}
		row := f.Row(y)
		for x := 0; e.rc.Next() && x < f.Cols; x++ {
			e.cell(&row[x], colors, f)
		}
		_ = e.ri.SetDirty(false)
	}
	_ = e.rs.Clean()

	if c, err := e.rs.Cursor(); err == nil {
		f.Cursor = Cursor{
			X: int(c.ViewportX), Y: int(c.ViewportY),
			Visible: c.Visible && c.ViewportHasValue, Blink: c.Blinking,
		}
		switch c.VisualStyle {
		case gt.CursorStyleBar:
			f.Cursor.Shape = CursorBar
		case gt.CursorStyleUnderline:
			f.Cursor.Shape = CursorUnderline
		case gt.CursorStyleBlockHollow:
			f.Cursor.Shape = CursorBlockHollow
		default:
			f.Cursor.Shape = CursorBlock
		}
	}
}

func (e *ghosttyEngine) cell(out *Cell, colors *gt.RenderStateColors, f *Frame) {
	if raw, err := e.rc.Raw(); err == nil {
		if w, err := raw.Wide(); err == nil {
			switch w {
			case gt.CellWideWide:
				out.Width = 2
			case gt.CellWideSpacerTail, gt.CellWideSpacerHead:
				out.Width = 0
				return
			}
		}
	}
	if g, err := e.rc.GraphemesInto(e.gbuf[:0]); err == nil && len(g) > 0 {
		e.gbuf = g
		var sb strings.Builder
		for _, cp := range g {
			sb.WriteRune(rune(cp))
		}
		if s := sb.String(); s != " " {
			out.Text = s
		}
	}
	st, err := e.rc.Style()
	if err != nil || st == nil {
		return
	}
	fg := resolveGhostty(st.FgColor(), colors, f.Fg)
	bg := resolveGhostty(st.BgColor(), colors, f.Bg)
	if st.Bold() {
		out.Attrs |= AttrBold
	}
	if st.Faint() {
		out.Attrs |= AttrFaint
	}
	if st.Italic() {
		out.Attrs |= AttrItalic
	}
	if st.Blink() {
		out.Attrs |= AttrBlink
	}
	if st.Invisible() {
		out.Attrs |= AttrInvisible
	}
	if st.Strikethrough() {
		out.Attrs |= AttrStrikethrough
	}
	if st.Overline() {
		out.Attrs |= AttrOverline
	}
	if st.Underline() != gt.UnderlineNone {
		out.Attrs |= AttrUnderline
	}
	if st.Inverse() {
		fg, bg = bg, fg
	}
	out.Fg, out.Bg = fg, bg
}

func resolveGhostty(sc gt.StyleColor, colors *gt.RenderStateColors, fallback color.NRGBA) color.NRGBA {
	switch sc.Tag {
	case gt.StyleColorRGB:
		return nrgba(sc.RGB)
	case gt.StyleColorPalette:
		return nrgba(colors.Palette[sc.Palette])
	}
	return fallback
}

func (e *ghosttyEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.kev != nil {
		e.kev.Close()
	}
	if e.keys != nil {
		e.keys.Close()
	}
	if e.rc != nil {
		e.rc.Close()
	}
	if e.ri != nil {
		e.ri.Close()
	}
	if e.rs != nil {
		e.rs.Close()
	}
	if e.t != nil {
		e.t.Close()
		e.t = nil
	}
}
