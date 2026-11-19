// Package termview is the Gio widget that shows a terminal engine and turns
// keyboard, mouse and clipboard input into input for it.
package termview

import (
	"image"
	"io"
	"strings"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// View shows a terminal engine. Create it with [New]; it is laid out with
// [View.Layout] on the GUI goroutine.
type View struct {
	Engine term.Engine
	// FontSize is the text size (default 11pt as in LeanTerm).
	FontSize unit.Sp
	// Padding around the grid.
	Padding unit.Dp
	// OnResize is called when the grid size changes, e.g. to resize the PTY.
	OnResize func(cols, rows int)

	frame       term.Frame
	cols, rows  int
	cellW       int // px
	cellH       int
	ascent      int
	metricsFor  fixed.Int26_6
	focused     bool
	scrollAccum float32
	glyphs      []text.Glyph
	sh          *text.Shaper
}

// noWrap is a line width text never reaches, so runs are not wrapped or
// truncated.
const noWrap = 1 << 24

// New creates a view for eng.
func New(eng term.Engine) *View {
	return &View{Engine: eng, FontSize: 11 * 96 / 72, Padding: 4}
}

func (v *View) face(attrs term.Attr) font.Font {
	f := font.Font{Typeface: fonts.Monospace}
	if attrs&term.AttrBold != 0 {
		f.Weight = font.Bold
	}
	if attrs&term.AttrItalic != 0 {
		f.Style = font.Italic
	}
	return f
}

// metrics measures the cell size of the monospace face at the current size.
func (v *View) metrics(gtx layout.Context, sh *text.Shaper) {
	ppem := fixed.I(gtx.Sp(v.FontSize))
	if ppem == v.metricsFor && v.cellW > 0 {
		return
	}
	sh.LayoutString(text.Parameters{Font: v.face(0), PxPerEm: ppem, MaxWidth: noWrap}, "M")
	g, ok := sh.NextGlyph()
	for _, ok2 := sh.NextGlyph(); ok2; _, ok2 = sh.NextGlyph() {
	}
	if !ok {
		return
	}
	v.cellW = max(1, g.Advance.Round())
	v.ascent = g.Ascent.Ceil()
	v.cellH = max(1, g.Ascent.Ceil()+g.Descent.Ceil())
	v.metricsFor = ppem
}

// Focus asks for the keyboard focus.
func (v *View) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: v}) }

// Layout handles input and draws the terminal into the available space.
func (v *View) Layout(gtx layout.Context, sh *text.Shaper) layout.Dimensions {
	v.sh = sh
	v.metrics(gtx, sh)
	size := gtx.Constraints.Max
	pad := gtx.Dp(v.Padding)

	// Grid size from the available space.
	cols := max(1, (size.X-2*pad)/v.cellW)
	rows := max(1, (size.Y-2*pad)/v.cellH)
	if cols != v.cols || rows != v.rows {
		v.cols, v.rows = cols, rows
		v.Engine.Resize(cols, rows, v.cellW, v.cellH)
		if v.OnResize != nil {
			v.OnResize(cols, rows)
		}
	}

	v.events(gtx)

	v.Engine.Snapshot(&v.frame)
	f := &v.frame

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, f.Bg, clip.Rect{Max: size}.Op())
	// Input area for pointer, keys and pastes.
	event.Op(gtx.Ops, v)
	key.InputHintOp{Tag: v, Hint: key.HintAny}.Add(gtx.Ops)

	off := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	v.paintBackgrounds(gtx, f)
	v.paintCursor(gtx, f, false)
	v.paintText(gtx, sh, f)
	v.paintCursor(gtx, f, true)
	off.Pop()
	return layout.Dimensions{Size: size}
}

func (v *View) paintBackgrounds(gtx layout.Context, f *term.Frame) {
	for y := 0; y < f.Rows; y++ {
		row := f.Row(y)
		for x := 0; x < f.Cols; {
			bg := row[x].Bg
			end := x + 1
			for end < f.Cols && row[end].Bg == bg {
				end++
			}
			if bg != f.Bg {
				r := image.Rect(x*v.cellW, y*v.cellH, end*v.cellW, (y+1)*v.cellH)
				paint.FillShape(gtx.Ops, bg, clip.Rect(r).Op())
			}
			x = end
		}
	}
}

// paintText draws the text in runs of cells with the same style. ASCII runs
// are shaped together (the face is monospace); other characters are placed
// one by one on their column, as their glyphs may come from a fallback face
// with other advances.
func (v *View) paintText(gtx layout.Context, sh *text.Shaper, f *term.Frame) {
	ppem := fixed.I(gtx.Sp(v.FontSize))
	var sb strings.Builder
	cursorCell := -1
	if f.Cursor.Visible && v.focused && f.Cursor.Shape == term.CursorBlock {
		cursorCell = f.Cursor.Y*f.Cols + f.Cursor.X
	}
	for y := 0; y < f.Rows; y++ {
		row := f.Row(y)
		for x := 0; x < f.Cols; {
			c := row[x]
			if c.Text == "" || c.Width == 0 || c.Attrs&term.AttrInvisible != 0 || y*f.Cols+x == cursorCell {
				x++
				continue
			}
			end := x + 1
			sb.Reset()
			sb.WriteString(c.Text)
			if isASCII(c.Text) {
				for end < f.Cols {
					n := row[end]
					if n.Text == "" || !isASCII(n.Text) || n.Fg != c.Fg || n.Attrs != c.Attrs || y*f.Cols+end == cursorCell {
						break
					}
					sb.WriteString(n.Text)
					end++
				}
			}
			v.drawRun(gtx, sh, ppem, sb.String(), c, x, y)
			v.decorate(gtx, c, x, end, y)
			x = end
		}
	}
}

func (v *View) drawRun(gtx layout.Context, sh *text.Shaper, ppem fixed.Int26_6, s string, c term.Cell, x, y int) {
	sh.LayoutString(text.Parameters{Font: v.face(c.Attrs), PxPerEm: ppem, MaxWidth: noWrap, DisableSpaceTrim: true}, s)
	v.glyphs = v.glyphs[:0]
	for g, ok := sh.NextGlyph(); ok; g, ok = sh.NextGlyph() {
		v.glyphs = append(v.glyphs, g)
	}
	if len(v.glyphs) == 0 {
		return
	}
	// Snap ASCII runs to the cell grid: glyph advances are fractional, the
	// columns are whole pixels.
	if isASCII(s) && len(v.glyphs) == len(s) {
		x0 := v.glyphs[0].X
		for i := range v.glyphs {
			v.glyphs[i].X = x0 + fixed.I(i*v.cellW)
		}
	}
	fg := c.Fg
	if c.Attrs&term.AttrFaint != 0 {
		fg.A = fg.A / 2
	}
	// Shaped paths are relative to the baseline of the line.
	t := op.Offset(image.Pt(x*v.cellW, y*v.cellH+v.ascent)).Push(gtx.Ops)
	path := sh.Shape(v.glyphs)
	outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	if call := sh.Bitmaps(v.glyphs); call != (op.CallOp{}) {
		call.Add(gtx.Ops)
	}
	t.Pop()
}

func (v *View) decorate(gtx layout.Context, c term.Cell, x0, x1, y int) {
	thick := max(1, v.cellH/14)
	line := func(yy int) {
		r := image.Rect(x0*v.cellW, yy, x1*v.cellW, yy+thick)
		paint.FillShape(gtx.Ops, c.Fg, clip.Rect(r).Op())
	}
	top := y * v.cellH
	if c.Attrs&term.AttrUnderline != 0 {
		line(top + v.ascent + thick)
	}
	if c.Attrs&term.AttrStrikethrough != 0 {
		line(top + v.ascent*2/3)
	}
	if c.Attrs&term.AttrOverline != 0 {
		line(top)
	}
}

// paintCursor draws the cursor: before the text (block cursor background) or
// after it (the character under a block cursor in the background color, and
// the bar, underline and unfocused outline shapes).
func (v *View) paintCursor(gtx layout.Context, f *term.Frame, after bool) {
	c := f.Cursor
	if !c.Visible || c.X >= f.Cols || c.Y >= f.Rows {
		return
	}
	cell := f.Row(c.Y)[c.X]
	w := v.cellW
	if cell.Width == 2 {
		w *= 2
	}
	r := image.Rect(c.X*v.cellW, c.Y*v.cellH, c.X*v.cellW+w, (c.Y+1)*v.cellH)
	col := f.Fg
	thick := max(1, v.cellH/12)
	shape := c.Shape
	if !v.focused {
		shape = term.CursorBlockHollow
	}
	switch shape {
	case term.CursorBlock:
		if !after {
			paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
			return
		}
		if cell.Text != "" && cell.Attrs&term.AttrInvisible == 0 {
			inv := cell
			inv.Fg = f.Bg
			inv.Attrs &^= term.AttrFaint
			v.drawRun(gtx, v.sh, fixed.I(gtx.Sp(v.FontSize)), cell.Text, inv, c.X, c.Y)
		}
	case term.CursorBar:
		if after {
			paint.FillShape(gtx.Ops, col, clip.Rect(image.Rect(r.Min.X, r.Min.Y, r.Min.X+thick, r.Max.Y)).Op())
		}
	case term.CursorUnderline:
		if after {
			paint.FillShape(gtx.Ops, col, clip.Rect(image.Rect(r.Min.X, r.Max.Y-thick, r.Max.X, r.Max.Y)).Op())
		}
	case term.CursorBlockHollow:
		if after {
			paint.FillShape(gtx.Ops, col, clip.Stroke{Path: clip.Rect(r).Path(), Width: float32(max(1, gtx.Dp(1)))}.Op())
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// events turns pending input into terminal input.
func (v *View) events(gtx layout.Context) {
	filters := []event.Filter{
		key.FocusFilter{Target: v},
		pointer.Filter{Target: v, Kinds: pointer.Press | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		transfer.TargetFilter{Target: v, Type: "application/text"},
		// Without Optional modifiers a filter only matches unmodified keys.
		key.Filter{Focus: v, Name: "", Optional: allMods},
		// Keys with a system meaning are only delivered when named.
		key.Filter{Focus: v, Name: key.NameTab, Optional: allMods},
		key.Filter{Focus: v, Name: key.NameEscape, Optional: allMods},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch e := ev.(type) {
		case key.FocusEvent:
			v.focused = e.Focus
		case pointer.Event:
			switch e.Kind {
			case pointer.Press:
				gtx.Execute(key.FocusCmd{Tag: v})
				if e.Buttons == pointer.ButtonTertiary {
					gtx.Execute(clipboard.ReadCmd{Tag: v})
				}
			case pointer.Scroll:
				// Positive Y scrolls down, towards the newest output.
				v.scrollAccum += e.Scroll.Y
				if lines := int(v.scrollAccum / float32(v.cellH)); lines != 0 {
					v.scrollAccum -= float32(lines * v.cellH)
					v.Engine.Scroll(lines)
				}
			}
		case transfer.DataEvent:
			r := e.Open()
			data, _ := io.ReadAll(io.LimitReader(r, 16<<20))
			r.Close()
			v.Engine.Paste(string(data))
		case key.EditEvent:
			if e.Text != "" {
				v.Engine.SendText(e.Text)
			}
		case key.Event:
			if e.State == key.Press {
				v.key(gtx, e)
			}
		}
	}
}

const allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand

var namedKeys = map[key.Name]term.Key{
	key.NameReturn:         term.KeyEnter,
	key.NameEnter:          term.KeyEnter,
	key.NameTab:            term.KeyTab,
	key.NameDeleteBackward: term.KeyBackspace,
	key.NameEscape:         term.KeyEscape,
	key.NameUpArrow:        term.KeyUp,
	key.NameDownArrow:      term.KeyDown,
	key.NameLeftArrow:      term.KeyLeft,
	key.NameRightArrow:     term.KeyRight,
	key.NameHome:           term.KeyHome,
	key.NameEnd:            term.KeyEnd,
	key.NamePageUp:         term.KeyPageUp,
	key.NamePageDown:       term.KeyPageDown,
	key.NameDeleteForward:  term.KeyDelete,
	key.NameF1:             term.KeyF1,
	key.NameF2:             term.KeyF2,
	key.NameF3:             term.KeyF3,
	key.NameF4:             term.KeyF4,
	key.NameF5:             term.KeyF5,
	key.NameF6:             term.KeyF6,
	key.NameF7:             term.KeyF7,
	key.NameF8:             term.KeyF8,
	key.NameF9:             term.KeyF9,
	key.NameF10:            term.KeyF10,
	key.NameF11:            term.KeyF11,
	key.NameF12:            term.KeyF12,
}

func (v *View) key(gtx layout.Context, e key.Event) {
	var mods term.Mods
	if e.Modifiers.Contain(key.ModShift) {
		mods |= term.ModShift
	}
	if e.Modifiers.Contain(key.ModCtrl) {
		mods |= term.ModCtrl
	}
	if e.Modifiers.Contain(key.ModAlt) {
		mods |= term.ModAlt
	}
	if e.Modifiers.Contain(key.ModSuper) {
		mods |= term.ModSuper
	}

	// LeanTerm shortcuts: Ctrl+Shift+V pastes, Shift+PageUp/Down scroll.
	switch {
	case mods == term.ModCtrl|term.ModShift && e.Name == "V":
		gtx.Execute(clipboard.ReadCmd{Tag: v})
		return
	case mods == term.ModShift && e.Name == key.NamePageUp:
		v.Engine.Scroll(-(v.rows - 1))
		return
	case mods == term.ModShift && e.Name == key.NamePageDown:
		v.Engine.Scroll(v.rows - 1)
		return
	}

	// Space also arrives as an EditEvent; it is only a key with Ctrl or Alt.
	if e.Name == key.NameSpace {
		if mods&(term.ModCtrl|term.ModAlt) != 0 {
			v.Engine.SendKey(term.KeyEvent{Key: term.KeySpace, Mods: mods})
		}
		return
	}
	if k, ok := namedKeys[e.Name]; ok {
		v.Engine.SendKey(term.KeyEvent{Key: k, Mods: mods})
		return
	}
	// Characters arrive as EditEvents; only Ctrl/Alt combinations are keys.
	if mods&(term.ModCtrl|term.ModAlt) == 0 {
		return
	}
	r, size := utf8.DecodeRuneInString(string(e.Name))
	if r == utf8.RuneError || size != len(e.Name) {
		return
	}
	v.Engine.SendKey(term.KeyEvent{Rune: toLower(r), Mods: mods &^ term.ModShift})
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
