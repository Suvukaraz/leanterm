package leanedit

import (
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/alecthomas/chroma/v2"
	"github.com/oligo/gvcode"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/textstyle/syntax"

	"github.com/Suvukaraz/leanterm/internal/gui/codeedit"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Tab is one document (EditorApp.svelte's Tab).
type Tab struct {
	ID    int
	File  *services.EditorFile
	Title string

	ed        *gvcode.Editor
	ready     bool // the text is loaded (initialState in the web GUI)
	savedText string
	Dirty     bool

	Encoding, savedEncoding     string
	LineEnding, savedLineEnding string // "lf" or "crlf"
	Mixed                       bool
	LanguageID                  string
	Highlight                   bool
	modTime, size               int64
	Loading                     string

	lexer     chroma.Lexer
	retoken   bool
	refreshAt time.Time
	dirtyAt   time.Time
	drawn     bool
	styled    string // the look last applied (font, size, colors)

	bookmarks       []*gvcode.Marker
	lines           *doc
	linesOf         string
	modified        map[int]bool
	modOf, modSaved string
	marks           *marks
	tokens          []syntax.Token // the highlighting (also the minimap's colors)
	mini            minimap
}

func newTab(id int, file *services.EditorFile, title string) *Tab {
	return &Tab{ID: id, File: file, Title: title, ed: &gvcode.Editor{}, Encoding: "utf-8", savedEncoding: "utf-8",
		LineEnding: "lf", savedLineEnding: "lf", Highlight: true}
}

// Text is the document's text.
func (t *Tab) Text() string { return t.ed.Text() }

// Editor is the gvcode editor of the tab.
func (t *Tab) Editor() *gvcode.Editor { return t.ed }

// setText replaces the document (a load) and makes it the saved state.
func (t *Tab) setText(s string) {
	t.ed.SetText(s)
	t.savedText = s
	t.ready = true
	t.Dirty = false
	t.retoken = true
}

func (t *Tab) computeDirty() bool {
	return t.Encoding != t.savedEncoding || t.LineEnding != t.savedLineEnding || t.Mixed || t.ed.Text() != t.savedText
}

// look is what the editor's appearance depends on.
type look struct {
	family   string
	size     float32 // pt
	tabWidth int
	softTab  bool
	wrap     bool
	spaces   bool
	pal      *gtk.Palette
}

// scheme builds the editor colors from the theme (theme.ts: the --gtk-syntax-*
// and --gtk-editor-* colors).
func scheme(p *gtk.Palette) syntax.ColorScheme {
	c := func(n color.NRGBA) gvcolor.Color { return gvcolor.MakeColor(n) }
	none := gvcolor.Color{}
	cs := syntax.ColorScheme{Name: "LeanTerm " + p.Name}
	cs.Foreground = c(p.TextColor)
	cs.Background = c(p.BaseColor)
	cs.SelectColor = c(p.EditorSelection)
	cs.LineColor = c(p.EditorActiveLine)
	cs.LineNumberColor = c(p.FgColor) // gvcode dims the other lines
	cs.AddStyle(codeedit.ScopeKeyword, syntax.Bold, c(p.SyntaxKeyword), none)
	cs.AddStyle(codeedit.ScopeString, 0, c(p.SyntaxString), none)
	cs.AddStyle(codeedit.ScopeComment, syntax.Italic, c(p.SyntaxComment), none)
	cs.AddStyle(codeedit.ScopeNumber, 0, c(p.SyntaxNumber), none)
	cs.AddStyle(codeedit.ScopeType, 0, c(p.SyntaxType), none)
	cs.AddStyle(codeedit.ScopeFunction, 0, c(p.SyntaxFunction), none)
	cs.AddStyle(codeedit.ScopeConstant, 0, c(p.SyntaxConstant), none)
	cs.AddStyle(codeedit.ScopeOperator, 0, c(p.SyntaxOperator), none)
	cs.AddStyle(codeedit.ScopeTag, 0, c(p.SyntaxTag), none)
	cs.AddStyle(codeedit.ScopeAttribute, 0, c(p.SyntaxAttribute), none)
	cs.AddStyle(codeedit.ScopeProperty, 0, c(p.SyntaxProperty), none)
	cs.AddStyle(codeedit.ScopeMeta, 0, c(p.SyntaxMeta), none)
	cs.AddStyle(codeedit.ScopeInvalid, 0, c(p.SyntaxInvalid), none)
	return cs
}

// apply sets the look on the editor when it changed.
func (t *Tab) apply(l look, typeface font.Typeface) {
	key := l.family + "|" + l.pal.Name + "|" + ftoa(l.size) + "|" + itoa(l.tabWidth) + "|" + btoa(l.softTab) + "|" + btoa(l.wrap) + "|" + btoa(l.spaces)
	if key == t.styled {
		return
	}
	t.styled = key
	t.ed.WithOptions(
		gvcode.WithFont(font.Font{Typeface: typeface}),
		// pt to px: 1pt = 4/3 px (sp at the window's scale).
		gvcode.WithTextSize(unit.Sp(l.size*4/3)),
		gvcode.WithLineHeight(0, 1.45),
		gvcode.WithTabWidth(l.tabWidth),
		gvcode.WithSoftTab(l.softTab),
		gvcode.WithColorScheme(scheme(l.pal)),
		gvcode.WithDefaultGutters(),
		gvcode.WithGutter(t.marksProvider(l.pal)),
		gvcode.WithGutterGap(unit.Dp(6)),
		// .cm-gutters: the header color with a border on the right.
		gvcode.WithGutterStyle(gvcolor.MakeColor(l.pal.HeaderBg), gvcolor.MakeColor(l.pal.BorderColor)),
		gvcode.WrapLine(l.wrap),
		// .cm-highlightSpace / .cm-highlightTab: the muted color.
		gvcode.WithWhitespace(l.spaces, l.pal.MutedFg),
	)
	t.retoken = true
}

func (t *Tab) marksProvider(p *gtk.Palette) *marks {
	if t.marks == nil {
		t.marks = &marks{t: t}
	}
	t.marks.pal = p
	return t.marks
}

// layout handles the editor's events and draws it. It reports whether the
// text changed.
func (t *Tab) layout(gtx layout.Context, sh *text.Shaper) (changed bool) {
	for {
		ev, ok := t.ed.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(gvcode.ChangeEvent); ok {
			changed = true
			if t.Highlight && t.lexer != nil {
				t.retoken = true
				t.refreshAt = gtx.Now.Add(150 * time.Millisecond)
			}
		}
	}
	if t.retoken {
		if gtx.Now.Before(t.refreshAt) {
			gtx.Execute(op.InvalidateCmd{At: t.refreshAt})
		} else {
			t.retoken = false
			if t.Highlight && t.lexer != nil {
				t.tokens = codeedit.Tokens(t.lexer, t.ed.Text())
			} else {
				t.tokens = nil
			}
			t.ed.SetSyntaxTokens(t.tokens...)
		}
	}
	// gvcode measures the gutter from the previous frame's layout: the first
	// frame needs a second one.
	if !t.drawn {
		t.drawn = true
		gtx.Execute(op.InvalidateCmd{})
	}
	t.ed.Layout(gtx, sh)
	return changed
}
