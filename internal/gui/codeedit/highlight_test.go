package codeedit

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/textstyle/syntax"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
)

func TestTokensGo(t *testing.T) {
	src := "package main\n\n// Größe\nfunc f() string { return \"ä\" + 42 }\n"
	toks := Tokens(Lexer("x.go"), src)
	runes := []rune(src)
	got := map[string]syntax.StyleScope{}
	for _, tk := range toks {
		got[string(runes[tk.Start:tk.End])] = tk.Scope
	}
	want := map[string]syntax.StyleScope{
		"package":  ScopeKeyword,
		"// Größe": ScopeComment,
		"f":        ScopeFunction,
		"string":   ScopeType,
		"\"ä\"":    ScopeString,
		"42":       ScopeNumber,
	}
	for text, scope := range want {
		if got[text] != scope {
			t.Errorf("%q: scope %q, want %q (all: %v)", text, got[text], scope, got)
		}
	}
}

func TestLexerUnknownFile(t *testing.T) {
	if Lexer("notes.unknownext") != nil {
		t.Fatal("lexer for unknown extension")
	}
	if Tokens(nil, "x") != nil {
		t.Fatal("tokens without lexer")
	}
	for _, name := range []string{"a.yaml", "nginx.conf", "Dockerfile", "a.sh", "a.py", "a.json", "a.toml", "a.ini", "a.sql"} {
		if Lexer(name) == nil {
			t.Errorf("no lexer for %s", name)
		}
	}
}

func TestGutterShownAfterFirstFrame(t *testing.T) {
	e := New("a.go", "package a\n")
	sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection(fonts.Collection()))
	var r input.Router
	for i := 0; i < 2; i++ {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Exact(image.Pt(800, 400)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Source: r.Source()}
		e.Layout(gtx, sh)
		r.Frame(&ops)
		if i == 0 {
			if _, ok := r.WakeupTime(); !ok {
				t.Fatal("first frame did not ask for a second one")
			}
		}
	}
	if e.GutterWidth() <= 20 {
		t.Fatalf("gutter width %d after the second frame", e.GutterWidth())
	}
}
