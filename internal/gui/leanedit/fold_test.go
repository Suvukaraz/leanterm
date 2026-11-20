package leanedit

import (
	"strings"
	"testing"

	"github.com/oligo/gvcode/textview"

	"github.com/Suvukaraz/leanterm/internal/services"
)

const foldSrc = `package main

import "fmt"

func a() {
	fmt.Println("a")
	if true {
		fmt.Println("b")
	}
}

func b() {
	fmt.Println("c")
}
`

// lineStart is the rune offset of a 0-based line.
func lineStart(t *Tab, line int) int { return t.lineIndex().line(line + 1).from }
func lineEnd(t *Tab, line int) int   { return t.lineIndex().line(line + 1).to }
func caretLine(t *Tab) int {
	c, _ := t.ed.Selection()
	return t.lineIndex().lineAt(c).n - 1
}

func openFolding(t *testing.T) (*harness, *Tab) {
	svc := newFake()
	svc.files["/w/main.go"] = foldSrc
	svc.pending = []services.EditorFile{{Kind: "local", Path: "/w/main.go"}}
	h := newHarness(t, svc)
	h.until("the folds", func() bool { t := h.a.Active(); return t != nil && t.ready && t.canFold() })
	return h, h.a.Active()
}

func TestFoldingInTheEditor(t *testing.T) {
	h, tab := openFolding(t)
	m := &tab.fold.m
	if _, ok := m.RangeAt(4); !ok { // func a() { .. }
		t.Fatal("no fold at func a", m.Ranges())
	}
	if _, ok := m.RangeAt(6); !ok { // if true {
		t.Fatal("no fold at if", m.Ranges())
	}

	// Fold "func a" by its marker: lines 5..9 hidden, on screen too.
	tab.setSel(Sel{lineStart(tab, 4), lineStart(tab, 4)})
	tab.toggleFoldLine(4)
	h.frame()
	tl := tab.ed.TextView().TextLayout()
	for l := 5; l <= 9; l++ {
		if !m.IsLineHidden(l) || !tl.Paragraphs[l].Hidden {
			t.Fatalf("line %d not hidden", l)
		}
		if tl.Paragraphs[l].StartY != tl.Paragraphs[4].StartY {
			t.Fatalf("hidden line %d takes space", l)
		}
	}
	if tl.Paragraphs[11].StartY-tl.Paragraphs[4].StartY != 2*(tl.Paragraphs[1].StartY-tl.Paragraphs[0].StartY) {
		t.Fatal("the lines below the fold do not move up")
	}
	if got, want := tab.visualLines(), tab.ed.Lines()-5; got != want {
		t.Fatalf("visual lines %d, want %d", got, want)
	}
	if len(tab.fold.boxes) != 1 || tab.fold.boxes[0].line != 4 {
		t.Fatal("no ⋯ box", tab.fold.boxes)
	}

	// Down from the header skips the hidden lines; up comes back.
	tab.ed.TextView().MoveLines(1, textview.SelectionClear)
	h.frame()
	if l := caretLine(tab); l != 10 {
		t.Fatalf("down: line %d", l)
	}
	tab.ed.TextView().MoveLines(-1, textview.SelectionClear)
	h.frame()
	if l := caretLine(tab); l != 4 {
		t.Fatalf("up: line %d", l)
	}

	// Right at the end of the header jumps over the block, Left back.
	tab.setSel(Sel{lineEnd(tab, 4), lineEnd(tab, 4)})
	h.frame()
	tab.ed.MoveCaret(1, 1)
	h.frame()
	if c, _ := tab.ed.Selection(); c != lineStart(tab, 10) || !m.IsFolded(4) {
		t.Fatalf("right: caret %d (line %d), folded %v", c, caretLine(tab), m.IsFolded(4))
	}
	tab.ed.MoveCaret(-1, -1)
	h.frame()
	if c, _ := tab.ed.Selection(); c != lineEnd(tab, 4) || !m.IsFolded(4) {
		t.Fatalf("left: caret %d (line %d)", c, caretLine(tab))
	}

	// A selection over the fold copies the hidden lines too.
	tab.setSel(Sel{Anchor: lineStart(tab, 3), Head: lineStart(tab, 10)})
	h.frame()
	if got := tab.ed.SelectedText(); !strings.Contains(got, `fmt.Println("b")`) || !strings.HasPrefix(got, "\nfunc a() {") {
		t.Fatalf("selected %q", got)
	}

	// Inserting lines above moves the fold; typing in the header keeps it.
	h.a.applyEdit(tab, []Change{{0, 0, "// one\n// two\n"}}, nil)
	h.frame()
	if !m.IsFolded(6) || !m.IsLineHidden(7) || m.IsLineHidden(12) {
		t.Fatalf("after insert: %v", m.FoldedRanges())
	}
	h.a.applyEdit(tab, []Change{{lineEnd(tab, 6), lineEnd(tab, 6), " // x"}}, nil)
	h.frame()
	if !m.IsFolded(6) {
		t.Fatal("header edit unfolded")
	}
	// Undo twice (a frame each, as with the keys): the fold stays with its
	// block.
	tab.ed.Undo()
	h.frame()
	tab.ed.Undo()
	h.frame()
	if tab.ed.Text() != foldSrc || !m.IsFolded(4) {
		t.Fatalf("undo: folded %v", m.FoldedRanges())
	}

	// A jump onto a hidden line (search, go to line) unfolds.
	tab.setSel(Sel{lineStart(tab, 7), lineStart(tab, 7)})
	h.frame()
	if m.IsFolded(4) || m.IsLineHidden(7) || caretLine(tab) != 7 {
		t.Fatalf("jump: folded %v, caret line %d", m.FoldedRanges(), caretLine(tab))
	}

	// Nested: fold the inner block, then the outer one, then unfold the outer.
	tab.toggleFoldLine(6)
	if caretLine(tab) != 6 {
		t.Fatalf("the caret stays hidden: line %d", caretLine(tab))
	}
	tab.toggleFoldLine(4)
	h.frame()
	tab.toggleFoldLine(4)
	h.frame()
	if !m.IsFolded(6) || m.IsLineHidden(6) || !m.IsLineHidden(7) {
		t.Fatal("inner fold lost", m.FoldedRanges())
	}

	// Deleting a selection over a fold removes the hidden lines with it.
	tab.toggleFoldLine(4)
	h.frame()
	tab.setSel(Sel{Anchor: lineStart(tab, 4), Head: lineStart(tab, 11)})
	tab.ed.Delete(1)
	h.frame()
	want := strings.Replace(foldSrc, "func a() {\n\tfmt.Println(\"a\")\n\tif true {\n\t\tfmt.Println(\"b\")\n\t}\n}\n\n", "", 1)
	if tab.ed.Text() != want || m.HasFolds() {
		t.Fatalf("delete over fold: %q folded %v", tab.ed.Text(), m.FoldedRanges())
	}
	h.until("the new parse", func() bool { _, ok := m.RangeAt(4); return ok && len(m.Ranges()) < 4 })

	// Fold all / unfold all through the menu actions.
	tab.foldAll()
	h.frame()
	if !m.IsFolded(4) || tab.visualLines() != tab.ed.Lines()-2 {
		t.Fatal("fold all", m.FoldedRanges(), tab.visualLines())
	}
	tab.unfoldAll()
	h.frame()
	if m.HasFolds() || tab.visualLines() != tab.ed.Lines() {
		t.Fatal("unfold all")
	}
	h.a.closeTabNow(tab.ID)
	if tab.fold.parser != nil {
		t.Fatal("the parser keeps running")
	}
}

// Plain text and languages without a grammar have no folding (and no
// errors).
func TestNoFoldingWithoutGrammar(t *testing.T) {
	svc := newFake()
	svc.files["/w/notes.txt"] = "a\n  b\n  c\n"
	svc.files["/w/run.bat"] = "@echo off\nif x (\n  echo y\n)\n"
	svc.pending = []services.EditorFile{{Kind: "local", Path: "/w/notes.txt"}, {Kind: "local", Path: "/w/run.bat"}}
	h := newHarness(t, svc)
	h.until("the files", func() bool { return len(h.a.tabs) == 2 && h.a.tabs[0].ready && h.a.tabs[1].ready })
	for i := 0; i < 5; i++ {
		h.frame()
	}
	for _, tab := range h.a.tabs {
		if tab.fold.parser != nil || tab.canFold() || tab.fold.m.HasFolds() {
			t.Errorf("%s folds", tab.Title)
		}
		tab.foldAll()
		tab.toggleFoldAtCaret()
		if tab.fold.m.HasFolds() {
			t.Errorf("%s folded", tab.Title)
		}
	}
	// Switching the language turns folding on.
	h.a.activate(h.a.tabs[0].ID)
	h.a.setLanguage("python")
	h.a.tabs[0].ed.SetText("def f():\n    return 1\n")
	h.until("python folds", func() bool { return h.a.tabs[0].canFold() })
}
