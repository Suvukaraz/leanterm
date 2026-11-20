// Package termview is the Gio widget that shows a terminal engine and turns
// keyboard, mouse and clipboard input into input for it
// (frontend/src/components/terminal/TerminalView.svelte with the engines'
// own mouse handling).
package termview

import (
	"image"
	"image/color"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
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

// DefaultFontPt is the terminal font size in points without a setting.
const DefaultFontPt = 11

// View shows a terminal engine. Create it with [New]; it is laid out with
// [View.Layout] on the GUI goroutine.
type View struct {
	Engine term.Engine
	// FontSize is the text size before zooming (default 11pt).
	FontSize unit.Sp
	// Typeface is the terminal font ("" = the system monospace font).
	Typeface font.Typeface
	// Inset is the space around the grid (the web GUI's container inset).
	Inset layout.Inset
	// CursorShape is the cursor as long as the program sets none
	// (look.CursorShape); CursorBlink makes the focused cursor blink.
	CursorShape term.CursorShape
	CursorBlink bool
	// WordChars are the characters besides letters and digits that a
	// double click selects as part of a word.
	WordChars string
	// SelectionToClipboard copies every mouse selection.
	SelectionToClipboard bool

	// OnResize is called when the grid size changes, e.g. to resize the PTY.
	OnResize func(cols, rows int)
	// OnOpenLink opens a link Ctrl+clicked in the terminal.
	OnOpenLink func(uri string)
	// OnContextMenu is called on a right click, with the position in the
	// view.
	OnContextMenu func(pos image.Point)
	// OnShortcut sees key presses first; it returns true for the keys the
	// application handled (they do not reach the terminal).
	OnShortcut func(e key.Event) bool

	frame      term.Frame
	cols, rows int
	cellW      int // px
	cellH      int
	ascent     int
	metricsFor metricsKey
	focused    bool
	glyphs     []text.Glyph
	sh         *text.Shaper
	origin     image.Point // grid offset in the view, px

	zoomPt      float32
	scrollAccum float32
	blinkStart  time.Time

	// Mouse selection.
	sel       *Selection
	selecting bool
	lastPress time.Time
	lastCell  Pos
	clicks    int
	// Mouse reports: the buttons the program saw pressed.
	reported map[int]bool
	// The link under the pointer while Ctrl is held.
	hover linkSpan

	lineBuf []term.Cell
}

type metricsKey struct {
	ppem fixed.Int26_6
	face font.Typeface
}

// noWrap is a line width text never reaches, so runs are not wrapped or
// truncated.
const noWrap = 1 << 24

// New creates a view for eng.
func New(eng term.Engine) *View {
	return &View{
		Engine:      eng,
		FontSize:    DefaultFontPt * 96 / 72,
		Inset:       layout.Inset{Top: 4, Right: 2, Bottom: 2, Left: 6},
		CursorBlink: true,
		WordChars:   DefaultWordChars,
		blinkStart:  time.Now(),
	}
}

func (v *View) face(attrs term.Attr) font.Font {
	f := font.Font{Typeface: fonts.Monospace}
	if v.Typeface != "" {
		f.Typeface = v.Typeface + "," + fonts.Monospace
	}
	if attrs&term.AttrBold != 0 {
		f.Weight = font.Bold
	}
	if attrs&term.AttrItalic != 0 {
		f.Style = font.Italic
	}
	return f
}

// textSize is the font size with the zoom applied.
func (v *View) textSize() unit.Sp {
	return v.FontSize + unit.Sp(v.zoomPt*96/72)
}

// Zoom changes the font size by deltaPt points (0 resets it), within 6 to
// 40 points like the web GUI.
func (v *View) Zoom(deltaPt float32) {
	if deltaPt == 0 {
		v.zoomPt = 0
		return
	}
	base := float32(v.FontSize) * 72 / 96
	v.zoomPt = max(6-base, min(40-base, v.zoomPt+deltaPt))
}

// metrics measures the cell size of the font at the current size.
func (v *View) metrics(gtx layout.Context, sh *text.Shaper) {
	k := metricsKey{fixed.I(gtx.Sp(v.textSize())), v.Typeface}
	if k == v.metricsFor && v.cellW > 0 {
		return
	}
	sh.LayoutString(text.Parameters{Font: v.face(0), PxPerEm: k.ppem, MaxWidth: noWrap}, "M")
	g, ok := sh.NextGlyph()
	for _, ok2 := sh.NextGlyph(); ok2; _, ok2 = sh.NextGlyph() {
	}
	if !ok {
		return
	}
	v.cellW = max(1, g.Advance.Round())
	v.ascent = g.Ascent.Ceil()
	v.cellH = max(1, g.Ascent.Ceil()+g.Descent.Ceil())
	v.metricsFor = k
}

// Focus asks for the keyboard focus.
func (v *View) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: v}) }

// Focused reports whether the terminal has the keyboard focus.
func (v *View) Focused() bool { return v.focused }

// Size is the grid size.
func (v *View) Size() (cols, rows int) { return v.cols, v.rows }

// Layout handles input and draws the terminal into the available space.
func (v *View) Layout(gtx layout.Context, sh *text.Shaper) layout.Dimensions {
	v.sh = sh
	v.metrics(gtx, sh)
	size := gtx.Constraints.Max
	v.origin = image.Pt(gtx.Dp(v.Inset.Left), gtx.Dp(v.Inset.Top))
	inner := size.Sub(v.origin).Sub(image.Pt(gtx.Dp(v.Inset.Right), gtx.Dp(v.Inset.Bottom)))

	// Grid size from the available space.
	cols := max(1, inner.X/v.cellW)
	rows := max(1, inner.Y/v.cellH)
	if cols != v.cols || rows != v.rows {
		v.cols, v.rows = cols, rows
		v.Engine.Resize(cols, rows, v.cellW, v.cellH)
		if v.OnResize != nil {
			v.OnResize(cols, rows)
		}
	}

	v.events(gtx)
	v.metrics(gtx, sh) // a zoom key changes the cell size

	v.Engine.Snapshot(&v.frame)
	f := &v.frame

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, f.Bg, clip.Rect{Max: size}.Op())
	// Input area for pointer, keys and pastes.
	event.Op(gtx.Ops, v)
	key.InputHintOp{Tag: v, Hint: key.HintAny}.Add(gtx.Ops)
	if v.hover.uri != "" {
		pointer.CursorPointer.Add(gtx.Ops)
	} else {
		pointer.CursorText.Add(gtx.Ops)
	}

	off := op.Offset(v.origin).Push(gtx.Ops)
	v.paintBackgrounds(gtx, f)
	v.paintSelection(gtx, f)
	cursorOn := v.cursorOn(gtx)
	v.paintCursor(gtx, f, false, cursorOn)
	v.paintText(gtx, sh, f, cursorOn)
	v.paintCursor(gtx, f, true, cursorOn)
	v.paintHover(gtx, f)
	off.Pop()
	return layout.Dimensions{Size: size}
}

// cursorOn is the blink phase of the focused cursor (shown half of every
// 1.2s, from the last input on); it asks for the frame of the next change.
func (v *View) cursorOn(gtx layout.Context) bool {
	if !v.CursorBlink || !v.focused {
		return true
	}
	const half = 600 * time.Millisecond
	el := gtx.Now.Sub(v.blinkStart)
	if el < 0 {
		return true
	}
	n := el / half
	gtx.Execute(op.InvalidateCmd{At: v.blinkStart.Add((n + 1) * half)})
	return n%2 == 0
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

// selectionColor is look.ts's selection tint, blended over the cells.
var selectionColor = color.NRGBA{R: 120, G: 160, B: 220, A: 115}

func (v *View) paintSelection(gtx layout.Context, f *term.Frame) {
	if v.sel == nil {
		return
	}
	from, to := v.sel.bounds(v.line, v.WordChars, f.Cols)
	top := f.History - f.Offset
	for y := 0; y < f.Rows; y++ {
		sp, ok := v.sel.rowSpan(top+y, f.Cols, from, to)
		if !ok {
			continue
		}
		sp.Start, sp.End = max(0, sp.Start), min(f.Cols-1, sp.End)
		if sp.Start > sp.End {
			continue
		}
		r := image.Rect(sp.Start*v.cellW, y*v.cellH, (sp.End+1)*v.cellW, (y+1)*v.cellH)
		paint.FillShape(gtx.Ops, selectionColor, clip.Rect(r).Op())
	}
}

// line reads a buffer line (reusing a buffer: valid until the next call).
func (v *View) line(row int) []term.Cell {
	v.lineBuf = v.Engine.Line(row, v.lineBuf[:0])
	return v.lineBuf
}

// paintText draws the text in runs of cells with the same style. ASCII runs
// are shaped together (the face is monospace); other characters are placed
// one by one on their column, as their glyphs may come from a fallback face
// with other advances.
func (v *View) paintText(gtx layout.Context, sh *text.Shaper, f *term.Frame, cursorOn bool) {
	ppem := fixed.I(gtx.Sp(v.textSize()))
	var sb strings.Builder
	cursorCell := -1
	if f.Cursor.Visible && v.focused && cursorOn && v.cursorShape(f) == term.CursorBlock {
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

func (v *View) underline(gtx layout.Context, c color.NRGBA, x0, x1, y int) {
	thick := max(1, v.cellH/14)
	yy := y*v.cellH + v.ascent + thick
	paint.FillShape(gtx.Ops, c, clip.Rect(image.Rect(x0*v.cellW, yy, x1*v.cellW, yy+thick)).Op())
}

func (v *View) decorate(gtx layout.Context, c term.Cell, x0, x1, y int) {
	thick := max(1, v.cellH/14)
	line := func(yy int) {
		r := image.Rect(x0*v.cellW, yy, x1*v.cellW, yy+thick)
		paint.FillShape(gtx.Ops, c.Fg, clip.Rect(r).Op())
	}
	top := y * v.cellH
	if c.Attrs&term.AttrUnderline != 0 {
		v.underline(gtx, c.Fg, x0, x1, y)
	}
	if c.Attrs&term.AttrStrikethrough != 0 {
		line(top + v.ascent*2/3)
	}
	if c.Attrs&term.AttrOverline != 0 {
		line(top)
	}
}

// paintHover underlines the link under the pointer while Ctrl is held.
func (v *View) paintHover(gtx layout.Context, f *term.Frame) {
	h := v.hover
	if h.uri == "" {
		return
	}
	y := h.row - (f.History - f.Offset)
	if y < 0 || y >= f.Rows {
		return
	}
	v.underline(gtx, f.Fg, h.start, h.end+1, y)
}

// cursorShape is the program's cursor shape, or the configured one while
// the program keeps the default block.
func (v *View) cursorShape(f *term.Frame) term.CursorShape {
	if f.Cursor.Shape == term.CursorBlock {
		return v.CursorShape
	}
	return f.Cursor.Shape
}

// paintCursor draws the cursor: before the text (block cursor background) or
// after it (the character under a block cursor in the background color, and
// the bar, underline and unfocused outline shapes).
func (v *View) paintCursor(gtx layout.Context, f *term.Frame, after, on bool) {
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
	shape := v.cursorShape(f)
	if !v.focused {
		shape = term.CursorBlockHollow
	} else if !on {
		return
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
			v.drawRun(gtx, v.sh, fixed.I(gtx.Sp(v.textSize())), cell.Text, inv, c.X, c.Y)
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
