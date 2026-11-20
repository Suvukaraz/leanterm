// Package codeedit builds LeanEdit's editor on gvcode: syntax highlighting
// with chroma lexers and LeanTerm's editor colors.
package codeedit

import (
	"image/color"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/textstyle/syntax"
)

// Scopes used for highlighting; the color scheme defines a style for each.
const (
	ScopeKeyword   syntax.StyleScope = "keyword"
	ScopeString    syntax.StyleScope = "string"
	ScopeComment   syntax.StyleScope = "comment"
	ScopeNumber    syntax.StyleScope = "number"
	ScopeType      syntax.StyleScope = "type"
	ScopeFunction  syntax.StyleScope = "function"
	ScopeConstant  syntax.StyleScope = "constant"
	ScopeOperator  syntax.StyleScope = "operator"
	ScopeTag       syntax.StyleScope = "tag"
	ScopeAttribute syntax.StyleScope = "attribute"
	ScopeProperty  syntax.StyleScope = "property"
	ScopeMeta      syntax.StyleScope = "meta"
	ScopeInvalid   syntax.StyleScope = "invalid"
)

// scopeOf maps a chroma token type to a scope ("" = not highlighted).
func scopeOf(t chroma.TokenType) syntax.StyleScope {
	switch {
	case t == chroma.KeywordType || t == chroma.NameClass:
		return ScopeType
	case t == chroma.KeywordConstant || t == chroma.NameConstant:
		return ScopeConstant
	case t.InCategory(chroma.Keyword):
		return ScopeKeyword
	case t.InCategory(chroma.Comment):
		if t == chroma.CommentPreproc || t == chroma.CommentPreprocFile {
			return ScopeMeta
		}
		return ScopeComment
	case t.InSubCategory(chroma.LiteralString):
		return ScopeString
	case t.InSubCategory(chroma.LiteralNumber):
		return ScopeNumber
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return ScopeFunction
	case t == chroma.NameBuiltin:
		return ScopeType
	case t == chroma.NameTag:
		return ScopeTag
	case t == chroma.NameAttribute:
		return ScopeAttribute
	case t == chroma.NameProperty || t == chroma.NameVariableInstance:
		return ScopeProperty
	case t == chroma.NameDecorator || t == chroma.NameLabel:
		return ScopeMeta
	case t.InCategory(chroma.Operator):
		return ScopeOperator
	case t == chroma.Error:
		return ScopeInvalid
	}
	return ""
}

// Lexer returns the chroma lexer for a file name, or nil for plain text.
func Lexer(filename string) chroma.Lexer {
	l := lexers.Match(filename)
	if l == nil {
		return nil
	}
	return chroma.Coalesce(l)
}

// Tokens highlights text with lexer. Offsets are in runes, as gvcode expects.
func Tokens(lexer chroma.Lexer, text string) []syntax.Token {
	if lexer == nil {
		return nil
	}
	it, err := lexer.Tokenise(nil, text)
	if err != nil {
		return nil
	}
	var out []syntax.Token
	pos := 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		n := utf8.RuneCountInString(tok.Value)
		if scope := scopeOf(tok.Type); scope != "" && n > 0 {
			// Merge with the previous token of the same scope.
			if k := len(out) - 1; k >= 0 && out[k].Scope == scope && out[k].End == pos {
				out[k].End = pos + n
			} else {
				out = append(out, syntax.Token{Start: pos, End: pos + n, Scope: scope})
			}
		}
		pos += n
	}
	return out
}

func hex(s string) gvcolor.Color {
	c, _ := gvcolor.Hex2Color(s)
	return c
}

func nrgba(r, g, b, a uint8) gvcolor.Color {
	return gvcolor.MakeColor(color.NRGBA{R: r, G: g, B: b, A: a})
}

// DarkScheme is LeanEdit's dark color scheme (the --gtk-syntax-* and
// --gtk-editor-* colors of LeanTerm's theme).
func DarkScheme() syntax.ColorScheme {
	cs := syntax.ColorScheme{Name: "LeanTerm dark"}
	cs.Foreground = hex("#f2f2f2")
	cs.Background = hex("#1f1f1f")
	cs.SelectColor = nrgba(74, 144, 217, 97)
	cs.LineColor = nrgba(255, 255, 255, 12)
	cs.LineNumberColor = hex("#8c8c8c")
	none := gvcolor.Color{}
	cs.AddStyle(ScopeKeyword, syntax.Bold, hex("#78aef0"), none)
	cs.AddStyle(ScopeString, 0, hex("#9ccc65"), none)
	cs.AddStyle(ScopeComment, syntax.Italic, hex("#8c8c8c"), none)
	cs.AddStyle(ScopeNumber, 0, hex("#e5b567"), none)
	cs.AddStyle(ScopeType, 0, hex("#6fc3c3"), none)
	cs.AddStyle(ScopeFunction, 0, hex("#f3df8a"), none)
	cs.AddStyle(ScopeConstant, 0, hex("#e59a6f"), none)
	cs.AddStyle(ScopeOperator, 0, hex("#c8c8c8"), none)
	cs.AddStyle(ScopeTag, 0, hex("#78aef0"), none)
	cs.AddStyle(ScopeAttribute, 0, hex("#c39ac9"), none)
	cs.AddStyle(ScopeProperty, 0, hex("#9fc7f0"), none)
	cs.AddStyle(ScopeMeta, 0, hex("#b48ead"), none)
	cs.AddStyle(ScopeInvalid, 0, hex("#ffb4b4"), none)
	return cs
}

// LexerByName returns the chroma lexer of a name or alias (nil if unknown).
func LexerByName(name string) chroma.Lexer {
	l := lexers.Get(name)
	if l == nil {
		return nil
	}
	return chroma.Coalesce(l)
}
