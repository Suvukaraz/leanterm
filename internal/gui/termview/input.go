package termview

import (
	"image"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

const allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand

// events turns pending input into terminal input.
func (v *View) events(gtx layout.Context) {
	filters := []event.Filter{
		key.FocusFilter{Target: v},
		pointer.Filter{
			Target:  v,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll | pointer.Cancel | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		},
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
			v.blinkStart = gtx.Now
		case pointer.Event:
			v.pointer(gtx, e)
		case transfer.DataEvent:
			r := e.Open()
			data, _ := io.ReadAll(io.LimitReader(r, 16<<20))
			r.Close()
			v.Paste(string(data))
		case key.EditEvent:
			if e.Text != "" {
				v.input()
				v.Engine.SendText(e.Text)
			}
		case key.Event:
			if e.State == key.Press {
				v.key(gtx, e)
			}
		}
	}
}

// input notes typing: the cursor shows at once and the selection ends.
func (v *View) input() {
	v.blinkStart = time.Now()
	v.sel = nil
}

// Paste sends text to the program as a paste.
func (v *View) Paste(s string) {
	if s != "" {
		v.input()
		v.Engine.Paste(s)
	}
}

// RequestPaste reads the clipboard and pastes it.
func (v *View) RequestPaste(gtx layout.Context) { gtx.Execute(clipboard.ReadCmd{Tag: v}) }

// SelectionText is the selected text ("" without a selection).
func (v *View) SelectionText() string {
	if v.sel == nil {
		return ""
	}
	return v.sel.Text(v.line, v.WordChars, v.cols)
}

// Copy puts the selected text on the clipboard.
func (v *View) Copy(gtx layout.Context) {
	if s := v.SelectionText(); s != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s))})
	}
}

// Select replaces the selection and scrolls it into view (the find bar).
func (v *View) Select(s *Selection) {
	v.sel = s
	if s == nil {
		return
	}
	var f term.Frame
	v.Engine.Snapshot(&f)
	top := f.History - f.Offset
	if r := s.Head.Row; r < top || r >= top+f.Rows {
		// Center the row like the web GUI's find bar.
		v.Engine.Scroll(r - top - f.Rows/2)
	}
}

// cellAt is the buffer cell under a pointer position; inside is false
// outside the grid.
func (v *View) cellAt(p f32.Point) (Pos, image.Point, bool) {
	x := int(p.X) - v.origin.X
	y := int(p.Y) - v.origin.Y
	col, row := floorDiv(x, v.cellW), floorDiv(y, v.cellH)
	inside := col >= 0 && col < v.cols && row >= 0 && row < v.rows
	vp := image.Pt(max(0, min(v.cols-1, col)), max(0, min(v.rows-1, row)))
	top := v.frame.History - v.frame.Offset
	return Pos{Row: top + vp.Y, Col: vp.X}, vp, inside
}

func floorDiv(a, b int) int {
	if a < 0 {
		return (a - b + 1) / b
	}
	return a / b
}

func buttonOf(b pointer.Buttons) int {
	switch {
	case b.Contain(pointer.ButtonPrimary):
		return 0
	case b.Contain(pointer.ButtonTertiary):
		return 1
	case b.Contain(pointer.ButtonSecondary):
		return 2
	}
	return -1
}

func modsOf(m key.Modifiers) term.Mods {
	var mods term.Mods
	if m.Contain(key.ModShift) {
		mods |= term.ModShift
	}
	if m.Contain(key.ModCtrl) {
		mods |= term.ModCtrl
	}
	if m.Contain(key.ModAlt) {
		mods |= term.ModAlt
	}
	if m.Contain(key.ModSuper) {
		mods |= term.ModSuper
	}
	return mods
}

func (v *View) pointer(gtx layout.Context, e pointer.Event) {
	pos, vp, inside := v.cellAt(e.Position)
	modes := v.Engine.Modes()
	// Programs that track the mouse get the clicks; Shift selects anyway.
	report := modes.Mouse != term.MouseNone && !e.Modifiers.Contain(key.ModShift)
	mods := modsOf(e.Modifiers)

	switch e.Kind {
	case pointer.Press:
		gtx.Execute(key.FocusCmd{Tag: v})
		b := buttonOf(e.Buttons)
		if b == 0 && e.Modifiers.Contain(key.ModCtrl) && inside {
			if uri := v.linkAt(vp, pos); uri != "" && v.OnOpenLink != nil {
				v.OnOpenLink(uri)
				return
			}
		}
		switch {
		case b == 2:
			if v.OnContextMenu != nil {
				v.OnContextMenu(e.Position.Round())
			}
		case report:
			v.sendMouse(modes, term.MouseReport{Kind: term.MousePress, Button: b, Col: vp.X, Row: vp.Y, Mods: mods})
			if v.reported == nil {
				v.reported = map[int]bool{}
			}
			v.reported[b] = true
		case b == 1:
			v.RequestPaste(gtx) // middle click pastes
		case b == 0:
			v.startSelection(gtx, pos, e.Modifiers.Contain(key.ModAlt))
		}
	case pointer.Drag:
		if v.selecting {
			// Beyond the top or bottom the view scrolls along.
			if int(e.Position.Y) < v.origin.Y {
				v.Engine.Scroll(-1)
			} else if int(e.Position.Y) >= v.origin.Y+v.rows*v.cellH {
				v.Engine.Scroll(1)
			}
			v.sel.Head = pos
			return
		}
		if report {
			v.sendMouse(modes, term.MouseReport{Kind: term.MouseMotion, Button: buttonOf(e.Buttons), Col: vp.X, Row: vp.Y, Mods: mods})
		}
	case pointer.Move:
		v.updateHover(vp, pos, e.Modifiers.Contain(key.ModCtrl) && inside)
		if report && modes.Mouse == term.MouseAny {
			v.sendMouse(modes, term.MouseReport{Kind: term.MouseMotion, Button: -1, Col: vp.X, Row: vp.Y, Mods: mods})
		}
	case pointer.Release:
		if v.selecting {
			v.selecting = false
			if v.SelectionToClipboard {
				v.Copy(gtx)
			}
			return
		}
		for b := range v.reported {
			v.sendMouse(modes, term.MouseReport{Kind: term.MouseRelease, Button: b, Col: vp.X, Row: vp.Y, Mods: mods})
			delete(v.reported, b)
		}
	case pointer.Cancel:
		v.selecting = false
		clear(v.reported)
	case pointer.Leave:
		v.hover = linkSpan{}
	case pointer.Scroll:
		v.wheel(gtx, e, modes, vp, report)
	}
}

func (v *View) sendMouse(m term.Modes, r term.MouseReport) {
	if b := term.EncodeMouse(m, r); b != nil {
		v.Engine.SendRaw(b)
	}
}

// startSelection begins a selection; double and triple clicks on the same
// cell select words and lines.
func (v *View) startSelection(gtx layout.Context, pos Pos, rect bool) {
	if gtx.Now.Sub(v.lastPress) < 400*time.Millisecond && pos == v.lastCell {
		v.clicks = v.clicks%3 + 1
	} else {
		v.clicks = 1
	}
	v.lastPress, v.lastCell = gtx.Now, pos
	mode := []SelectMode{SelectChars, SelectWords, SelectLines}[v.clicks-1]
	v.sel = &Selection{Anchor: pos, Head: pos, Mode: mode, Rect: rect && mode == SelectChars}
	v.selecting = true
}

// wheel scrolls the history, zooms with Ctrl, or reports the wheel to
// programs that track the mouse (and sends arrow keys on the alternate
// screen, as ghostty-web does).
func (v *View) wheel(gtx layout.Context, e pointer.Event, modes term.Modes, vp image.Point, report bool) {
	v.scrollAccum += e.Scroll.Y
	lines := int(v.scrollAccum / float32(v.cellH))
	if lines == 0 {
		return
	}
	v.scrollAccum -= float32(lines * v.cellH)
	switch {
	case e.Modifiers.Contain(key.ModCtrl):
		if lines < 0 {
			v.Zoom(1)
		} else {
			v.Zoom(-1)
		}
	case report:
		kind := term.MouseWheelDown
		if lines < 0 {
			kind = term.MouseWheelUp
		}
		for i := 0; i < min(abs(lines), 10); i++ {
			v.sendMouse(modes, term.MouseReport{Kind: kind, Col: vp.X, Row: vp.Y, Mods: modsOf(e.Modifiers)})
		}
	case modes.AltScreen:
		k := term.KeyDown
		if lines < 0 {
			k = term.KeyUp
		}
		for i := 0; i < abs(lines); i++ {
			v.Engine.SendKey(term.KeyEvent{Key: k})
		}
	default:
		v.Engine.Scroll(lines)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ---- links ------------------------------------------------------------------

// linkSpan is a link on a buffer row: columns start to end (inclusive).
type linkSpan struct {
	row, start, end int
	uri             string
}

// urlRE finds plain URLs in the text (the engines' link detection).
var urlRE = regexp.MustCompile(`(?:https?|ftp|file)://[^\s<>"'\x60]+`)

// linkAt is the OSC 8 link at a viewport cell or the URL written there.
func (v *View) linkAt(vp image.Point, pos Pos) string {
	if uri := v.Engine.LinkAt(vp.X, vp.Y); uri != "" {
		return uri
	}
	if l := v.urlAt(pos); l.uri != "" {
		return l.uri
	}
	return ""
}

func (v *View) urlAt(pos Pos) linkSpan {
	cells := v.line(pos.Row)
	text, cols := rowText(cells)
	for _, m := range urlRE.FindAllStringIndex(text, -1) {
		s, e := m[0], m[1]
		for e > s && strings.ContainsRune(".,;:!?)]}", rune(text[e-1])) {
			e--
		}
		if e <= s {
			continue
		}
		start, end := cols[s], cols[e-1]
		if pos.Col >= start && pos.Col <= end {
			return linkSpan{row: pos.Row, start: start, end: end, uri: text[s:e]}
		}
	}
	return linkSpan{}
}

// rowText is a row as text with the column of every byte.
func rowText(cells []term.Cell) (string, []int) {
	var sb strings.Builder
	var cols []int
	for x, c := range cells {
		if c.Width == 0 {
			continue
		}
		s := c.Text
		if s == "" {
			s = " "
		}
		sb.WriteString(s)
		for range len(s) {
			cols = append(cols, x)
		}
	}
	return sb.String(), cols
}

func (v *View) updateHover(vp image.Point, pos Pos, ctrl bool) {
	if !ctrl {
		v.hover = linkSpan{}
		return
	}
	if uri := v.Engine.LinkAt(vp.X, vp.Y); uri != "" {
		v.hover = linkSpan{row: pos.Row, start: vp.X, end: vp.X, uri: uri}
		return
	}
	v.hover = v.urlAt(pos)
}

// ---- keys -------------------------------------------------------------------

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
	if v.OnShortcut != nil && v.OnShortcut(e) {
		return
	}
	mods := modsOf(e.Modifiers)

	// TerminalView.svelte's terminal shortcuts: copy and paste with
	// Ctrl+Shift, zoom with Ctrl; Shift+PageUp/Down scroll.
	switch {
	case mods == term.ModCtrl|term.ModShift && e.Name == "C":
		v.Copy(gtx)
		return
	case mods == term.ModCtrl|term.ModShift && e.Name == "V":
		v.RequestPaste(gtx)
		return
	case mods&term.ModCtrl != 0 && mods&term.ModAlt == 0 && (e.Name == "+" || e.Name == "="):
		v.Zoom(1)
		return
	case mods&term.ModCtrl != 0 && mods&term.ModAlt == 0 && e.Name == "-":
		v.Zoom(-1)
		return
	case mods&term.ModCtrl != 0 && mods&term.ModAlt == 0 && e.Name == "0":
		v.Zoom(0)
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
			v.input()
			v.Engine.SendKey(term.KeyEvent{Key: term.KeySpace, Mods: mods})
		}
		return
	}
	if k, ok := namedKeys[e.Name]; ok {
		v.input()
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
	v.input()
	v.Engine.SendKey(term.KeyEvent{Rune: toLower(r), Mods: mods &^ term.ModShift})
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
