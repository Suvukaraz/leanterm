package codeedit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/alecthomas/chroma/v2"
	"github.com/oligo/gvcode"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

// Editor is one open document: a gvcode editor with highlighting.
type Editor struct {
	*gvcode.Editor
	Path     string
	Modified bool

	lexer     chroma.Lexer
	dirty     bool
	refreshAt time.Time
	drawn     bool
}

// New creates an editor for text; path picks the language.
func New(path, content string) *Editor {
	e := &Editor{Editor: &gvcode.Editor{}, Path: path, lexer: Lexer(filepath.Base(path))}
	style, _, width := gvcode.GuessIndentation(content)
	if width <= 0 {
		width = 4
	}
	e.WithOptions(
		gvcode.WithFont(font.Font{Typeface: fonts.Monospace}),
		gvcode.WithTextSize(unit.Sp(10*96/72)),
		gvcode.WithLineHeight(0, 1.35),
		gvcode.WithTabWidth(width),
		gvcode.WithSoftTab(style == gvcode.Spaces),
		gvcode.WithColorScheme(DarkScheme()),
		gvcode.WithDefaultGutters(),
		gvcode.WithGutterGap(unit.Dp(8)),
		gvcode.WrapLine(false),
	)
	e.SetText(content)
	e.SetSyntaxTokens(Tokens(e.lexer, content)...)
	return e
}

// Open reads a file into a new editor.
func Open(path string) (*Editor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return New(path, string(data)), nil
}

// Layout handles edits (re-highlighting shortly after typing stops) and draws
// the editor.
func (e *Editor) Layout(gtx layout.Context, sh *text.Shaper) layout.Dimensions {
	for {
		ev, ok := e.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(gvcode.ChangeEvent); ok {
			e.Modified = true
			e.dirty = true
			e.refreshAt = gtx.Now.Add(150 * time.Millisecond)
		}
	}
	if e.dirty {
		if gtx.Now.Before(e.refreshAt) {
			gtx.Execute(op.InvalidateCmd{At: e.refreshAt})
		} else {
			e.SetSyntaxTokens(Tokens(e.lexer, e.Text())...)
			e.dirty = false
		}
	}
	// gvcode measures the gutter (line numbers) from the text layout of the
	// previous frame, so the first frame is drawn without it: draw again.
	if !e.drawn {
		e.drawn = true
		gtx.Execute(op.InvalidateCmd{})
	}
	return e.Editor.Layout(gtx, sh)
}

// Language is the name of the highlighting language.
func (e *Editor) Language() string {
	if e.lexer == nil {
		return "Normal"
	}
	return e.lexer.Config().Name
}

// StatusFields are the fields of LeanEdit's status bar: position on the left,
// mode, indentation, encoding, line endings and language on the right.
func (e *Editor) StatusFields() (left string, right []string) {
	line, col := e.CaretPos()
	left = fmt.Sprintf("Line %d, Column %d", line+1, col+1)
	style, width := e.TabStyle()
	tabs := fmt.Sprintf("Soft Tabs: %d", width)
	if style == gvcode.Tabs {
		tabs = fmt.Sprintf("Tab Size: %d", width)
	}
	eol := "Unix (LF)"
	if strings.Contains(string(e.DetectedLineEnding()), "\r") {
		eol = "Windows (CRLF)"
	}
	return left, []string{"INS", "LINE", tabs, "UTF-8", eol, e.Language()}
}
