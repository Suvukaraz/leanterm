package leanedit

import (
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/Suvukaraz/leanterm/internal/gui/completion"
	"github.com/Suvukaraz/leanterm/internal/services"
)

func (h *harness) key(name key.Name, mods key.Modifiers) {
	h.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	h.frame()
	h.frame()
}

// typeText types s one rune per frame (as the keyboard does).
func (h *harness) typeText(t *Tab, s string) {
	for _, r := range s {
		t.ed.Insert(string(r))
		h.frame()
	}
}

func completionHarness(t *testing.T, text string) (*harness, *Tab) {
	h := newHarness(t, newFake())
	h.frame()
	h.a.NewDocument()
	tab := h.a.Active()
	tab.LanguageID = "go"
	tab.ed.SetText(text)
	h.r.Queue(key.FocusEvent{Focus: true})
	h.a.ui_.focusEditor = true
	h.frame()
	h.frame()
	n := tab.ed.Len()
	tab.ed.SetCaret(n, n)
	h.frame()
	h.until("the word index", func() bool { return tab.comp.idx != nil && tab.comp.idx.Snapshot() != nil })
	return h, tab
}

func words(items []completion.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Text)
	}
	return out
}

func TestWordCompletionPopup(t *testing.T) {
	h, tab := completionHarness(t, "func handler() {}\nhandleRequest(x)\n")
	base := tab.ed.Text()

	// Two characters: nothing; the third opens it.
	h.typeText(tab, "ha")
	if tab.CompletionOpen() {
		t.Fatal("open after two characters")
	}
	h.typeText(tab, "n")
	items, sel := tab.CompletionItems()
	if !tab.CompletionOpen() || sel != 0 || len(items) < 2 || strings.Join(words(items[:2]), ",") != "handler,handleRequest" {
		t.Fatalf("after han: open=%v %v", tab.CompletionOpen(), words(items))
	}
	if items[0].Kind != completion.KindDocumentWord || len(items[0].Matched) != 3 {
		t.Errorf("item: %+v", items[0])
	}
	// The popup was drawn.
	if len(tab.comp.clicks) < 2 {
		t.Error("popup rows not laid out")
	}

	// Enter without Up/Down: a line break, the popup closes.
	h.key(key.NameReturn, 0)
	if tab.CompletionOpen() || tab.ed.Text() != base+"han\n" {
		t.Fatalf("enter: open=%v %q", tab.CompletionOpen(), tab.ed.Text())
	}
	tab.ed.Undo()
	h.frame()
	if tab.ed.Text() != base+"han" {
		t.Fatalf("undo enter: %q", tab.ed.Text())
	}

	// Ctrl+Space opens it; Down (wraps), Up, Enter accepts.
	h.key(key.NameSpace, key.ModShortcut)
	if !tab.CompletionOpen() {
		t.Fatal("ctrl+space")
	}
	h.key(key.NameUpArrow, 0) // wraps to the last
	if items, sel := tab.CompletionItems(); sel != len(items)-1 {
		t.Fatalf("selection %d of %d", sel, len(items))
	}
	h.key(key.NameDownArrow, 0) // wraps to the first
	h.key(key.NameDownArrow, 0)
	if _, sel := tab.CompletionItems(); sel != 1 {
		t.Fatalf("selection %d", sel)
	}
	h.key(key.NameReturn, 0)
	if tab.CompletionOpen() || tab.ed.Text() != base+"handleRequest" {
		t.Fatalf("accept: %q", tab.ed.Text())
	}
	// One undo step takes the completion back.
	tab.ed.Undo()
	h.frame()
	if tab.ed.Text() != base+"han" {
		t.Fatalf("undo completion: %q", tab.ed.Text())
	}
	// gvcode's undo selects the restored text; continue after it.
	n := tab.ed.Len()
	tab.ed.SetCaret(n, n)
	h.frame()

	// Typing on opens it again, Tab accepts the first.
	h.typeText(tab, "d")
	if !tab.CompletionOpen() {
		c, a := tab.ed.Selection()
		t.Fatalf("not open after hand: %q sel %d %d", tab.ed.Text()[len(base):], c, a)
	}
	h.key(key.NameTab, 0)
	if tab.ed.Text() != base+"handler" || tab.CompletionOpen() {
		t.Fatalf("tab: %q open=%v", tab.ed.Text(), tab.CompletionOpen())
	}

	// Escape closes; a space closes; backspace below three characters closes.
	h.typeText(tab, " han")
	if !tab.CompletionOpen() {
		t.Fatal("not open")
	}
	h.key(key.NameEscape, 0)
	if tab.CompletionOpen() || !strings.HasSuffix(tab.ed.Text(), " han") {
		t.Fatalf("escape: %q", tab.ed.Text())
	}
	h.typeText(tab, "d")
	if !tab.CompletionOpen() {
		t.Fatal("not open after hand")
	}
	h.typeText(tab, " ")
	if tab.CompletionOpen() {
		t.Fatal("open after a space")
	}
	h.typeText(tab, "han")
	tab.ed.Delete(-1)
	h.frame()
	if tab.CompletionOpen() {
		t.Fatal("open with two characters")
	}

	// A new word never suggests itself.
	h.typeText(tab, "\nzebra")
	if tab.CompletionOpen() {
		t.Fatalf("self match: %v", words(tab.comp.items))
	}
}

func TestCompletionMidWord(t *testing.T) {
	h, tab := completionHarness(t, "counter := 1\nx := cou")
	tab.ed.Insert("ntXYZ") // "countXYZ"
	h.frame()
	// The caret after "count", before "XYZ".
	p := strings.Index(tab.ed.Text(), "XYZ")
	tab.ed.SetCaret(p, p)
	h.frame()
	h.key(key.NameSpace, key.ModShortcut)
	if !tab.CompletionOpen() {
		t.Fatal("ctrl+space mid-word")
	}
	h.key(key.NameTab, 0)
	if got := tab.ed.Text(); got != "counter := 1\nx := counterXYZ" {
		t.Fatalf("mid-word: %q", got)
	}
}

func TestCompletionUnicodeAndPlainText(t *testing.T) {
	h, tab := completionHarness(t, "größenordnung übel\n")
	tab.LanguageID = ""
	h.typeText(tab, "grö")
	if items, _ := tab.CompletionItems(); !tab.CompletionOpen() || len(items) != 1 || items[0].Text != "größenordnung" {
		t.Fatalf("umlauts: %v", words(items))
	}
	h.key(key.NameTab, 0)
	if tab.ed.Text() != "größenordnung übel\ngrößenordnung" {
		t.Fatalf("%q", tab.ed.Text())
	}
}

func TestGoToSymbol(t *testing.T) {
	svc := newFake()
	svc.files["/w/shapes.py"] = "LIMIT = 3\n\nclass Shape:\n    def area(self):\n        return 0\n\n    def name(self):\n        return 'x'\n\ndef helper():\n    pass\n"
	svc.files["/w/notes.txt"] = "plain text\n"
	svc.pending = []services.EditorFile{{Kind: "local", Path: "/w/shapes.py"}}
	h := newHarness(t, svc)
	h.until("the symbols", func() bool { t := h.a.Active(); return t != nil && len(t.Symbols()) >= 5 })
	tab := h.a.Active()

	// Fold the class: its methods are hidden.
	tab.toggleFoldLine(2)
	h.frame()
	if !tab.fold.m.IsLineHidden(6) {
		t.Fatal("class not folded")
	}

	h.a.openGotoSymbol()
	h.frame()
	items, _ := h.a.symPick.Visible()
	if len(items) != 5 || items[0].Label != "LIMIT" || items[3].Label != "name" || items[3].Detail != "Shape" || items[3].Right != "7" {
		t.Fatalf("all symbols: %+v", items)
	}
	h.a.symPick.SetQuery("nam")
	items, _ = h.a.symPick.Visible()
	if len(items) == 0 || items[0].Label != "name" || items[0].Badge != "meth" {
		t.Fatalf("nam: %+v", items)
	}
	h.a.symPick.Choose()
	h.frame()
	h.frame()
	if h.a.symPick.Opened() {
		t.Fatal("picker still open")
	}
	if l := caretLine(tab); l != 6 || tab.fold.m.IsLineHidden(6) || tab.fold.m.IsFolded(2) {
		t.Fatalf("jump: line %d, folded %v", l, tab.fold.m.FoldedRanges())
	}
	if c, _ := tab.ed.Selection(); c != lineStart(tab, 6)+8 {
		t.Fatalf("column: %d", c-lineStart(tab, 6))
	}

	// Escape closes the picker without a jump.
	h.a.openGotoSymbol()
	h.frame()
	h.a.symPick.Close()
	h.frame()
	if h.a.symPick.Opened() || caretLine(tab) != 6 {
		t.Fatal("close")
	}

	// Plain text: no symbols, the empty state.
	h.a.OpenFile(services.EditorFile{Kind: "local", Path: "/w/notes.txt"})
	h.until("notes", func() bool { t := h.a.Active(); return t != nil && t.Title == "notes.txt" && t.ready })
	h.a.openGotoSymbol()
	h.frame()
	if items, _ := h.a.symPick.Visible(); len(items) != 0 || !h.a.symPick.Opened() {
		t.Fatalf("plain text: %+v", items)
	}
	h.a.symPick.Close()
}
