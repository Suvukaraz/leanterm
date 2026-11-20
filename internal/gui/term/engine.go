// Package term is LeanTerm's engine-neutral terminal core. A terminal engine
// parses the output of a program (VT sequences) into a grid of cells and
// encodes keyboard and paste input for it; the GUI only talks to [Engine], so
// the engines can be swapped per terminal:
//
//   - "ghostty": Ghostty's terminal core (libghostty-vt via cgo), the default.
//     It is compiled in with the build tag "ghostty".
//   - "vt": charmbracelet/x/vt, pure Go, always available.
package term

import (
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strings"
	"sync"
)

// Engine names.
const (
	Ghostty = "ghostty"
	VT      = "vt"
)

// ErrUnavailable is returned by [New] for an engine that is not compiled in.
var ErrUnavailable = errors.New("terminal engine not available in this build")

// Attr is a set of text attributes of a cell.
type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrInvisible
	AttrStrikethrough
	AttrOverline
)

// Cell is one character cell of the viewport. Colors are resolved (palette,
// defaults and reverse video applied), so the renderer just paints them.
type Cell struct {
	// Text is the grapheme cluster of the cell; empty for blank cells and for
	// the second half of a wide character.
	Text string
	// Width is the number of columns the cell's text covers (0 for the second
	// half of a wide character, 2 for wide characters).
	Width uint8
	Fg    color.NRGBA
	Bg    color.NRGBA
	Attrs Attr
}

// CursorShape is the visual style of the cursor.
type CursorShape uint8

const (
	CursorBlock CursorShape = iota
	CursorBar
	CursorUnderline
	CursorBlockHollow
)

// Cursor is the cursor position in the viewport.
type Cursor struct {
	X, Y    int
	Visible bool
	Shape   CursorShape
	Blink   bool
}

// Frame is a snapshot of the visible part of the terminal.
type Frame struct {
	Cols, Rows int
	// Cells holds Rows*Cols cells, row by row.
	Cells  []Cell
	Cursor Cursor
	// Fg and Bg are the default colors (the background of the whole view).
	Fg, Bg color.NRGBA
	// History is the number of scrollback lines above the screen and Offset
	// how many of them the viewport is scrolled up (0 = at the bottom).
	History, Offset int
}

// Row returns the cells of row y.
func (f *Frame) Row(y int) []Cell { return f.Cells[y*f.Cols : (y+1)*f.Cols] }

func (f *Frame) reset(cols, rows int) {
	f.Cols, f.Rows = cols, rows
	if cap(f.Cells) < cols*rows {
		f.Cells = make([]Cell, cols*rows)
	}
	f.Cells = f.Cells[:cols*rows]
	for i := range f.Cells {
		f.Cells[i] = Cell{Fg: f.Fg, Bg: f.Bg, Width: 1}
	}
}

// Key is a named key that has no text of its own.
type Key uint8

const (
	KeyNone Key = iota
	KeyEnter
	KeyTab
	KeyBackspace
	KeyEscape
	KeySpace
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Mods is a set of modifier keys.
type Mods uint8

const (
	ModShift Mods = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

// KeyEvent is a key press. Either Key is set (a named key), or Rune is the
// unshifted character of the key, used with Ctrl or Alt (Ctrl+C: Rune 'c').
// Plain typed text goes through [Engine.SendText] instead.
type KeyEvent struct {
	Key  Key
	Rune rune
	Mods Mods
}

// Callbacks are called by an engine; they may run on any goroutine.
type Callbacks struct {
	// Output receives bytes for the program: encoded keys, pastes and the
	// terminal's answers to queries. Required.
	Output func([]byte)
	Title  func(string)
	// Dir is the working directory reported by the shell (OSC 7).
	Dir  func(string)
	Bell func()
}

// Options configure a new engine.
type Options struct {
	Cols, Rows int
	// Scrollback is the maximum number of history lines (0 = 10000).
	Scrollback int
	Colors     Colors
	Callbacks
}

// Engine is a terminal emulator. All methods are safe for concurrent use.
type Engine interface {
	Name() string
	// Feed parses output of the program.
	Feed(p []byte)
	// Resize changes the grid size; cellW and cellH are the cell size in
	// pixels (for size reports and image protocols).
	Resize(cols, rows, cellW, cellH int)
	SendKey(ev KeyEvent)
	// SendText sends typed text (including text committed by an input method).
	SendText(text string)
	// Paste sends text as a paste (bracketed when the program asked for it).
	Paste(text string)
	// Scroll moves the viewport by delta lines (negative = into the history).
	Scroll(delta int)
	ScrollToBottom()
	// Snapshot fills f with the current viewport.
	Snapshot(f *Frame)
	// Modes are the mouse and screen modes the program set.
	Modes() Modes
	// Lines is the number of lines of the shown screen: the history and the
	// screen rows (the alternate screen has no history).
	Lines() int
	// Line appends the cells of line row (0 = the oldest history line, as
	// [Frame.History]-[Frame.Offset]+y for viewport row y) to dst.
	Line(row int, dst []Cell) []Cell
	// LinkAt is the URI of the OSC 8 hyperlink at a viewport cell ("" for
	// none).
	LinkAt(x, y int) string
	// SendRaw sends bytes to the program as they are (mouse reports).
	SendRaw(p []byte)
	// Reset resets the terminal (RIS); Clear erases the screen and the
	// history.
	Reset()
	Clear()
	Close()
}

// Text is the text of cells (wide characters once, blank cells as spaces,
// trailing blanks removed).
func Text(cells []Cell) string {
	var sb strings.Builder
	for _, c := range cells {
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

type factory func(Options) (Engine, error)

var (
	regMu    sync.Mutex
	registry = map[string]factory{}
)

func register(name string, f factory) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[name] = f
}

// Available lists the engines compiled into this build.
func Available() []string {
	regMu.Lock()
	defer regMu.Unlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Default is the engine used when none is configured: Ghostty when it is
// compiled in, else vt.
func Default() string {
	regMu.Lock()
	defer regMu.Unlock()
	if _, ok := registry[Ghostty]; ok {
		return Ghostty
	}
	return VT
}

// Normalize maps an engine setting to an engine of this build: "vt" and the
// web GUI's alternative "rio" are vt; "ghostty", the former "xterm" and
// anything else the default.
func Normalize(name string) string {
	switch name {
	case VT, "rio":
		return VT
	}
	return Default()
}

// New creates a terminal engine by name ("" = [Default]).
func New(name string, opts Options) (Engine, error) {
	if name == "" {
		name = Default()
	}
	regMu.Lock()
	f, ok := registry[name]
	regMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, ErrUnavailable)
	}
	if opts.Output == nil {
		return nil, errors.New("term: Options.Output is required")
	}
	if opts.Cols < 1 {
		opts.Cols = 80
	}
	if opts.Rows < 1 {
		opts.Rows = 24
	}
	if opts.Scrollback <= 0 {
		opts.Scrollback = 10000
	}
	if opts.Colors.Palette == nil {
		opts.Colors = DefaultColors()
	}
	return f(opts)
}
