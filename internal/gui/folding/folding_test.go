package folding

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
)

func parseRanges(t *testing.T, langID, src string) []Range {
	t.Helper()
	lang := grammars[langID]()
	p := gotreesitter.NewParser(lang)
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Release()
	return Ranges(tree.RootNode(), lang)
}

func has(rs []Range, s, e int) bool {
	for _, r := range rs {
		if r.StartLine == s && r.EndLine == e {
			return true
		}
	}
	return false
}

func TestRangesGo(t *testing.T) {
	src := `package main

import (
	"fmt"
	"os"
)

/* a
   block comment */
func main() {
	if len(os.Args) > 1 {
		fmt.Println("x")
	}
	x := []int{
		1,
	}
	_ = x
}
`
	rs := parseRanges(t, "go", src)
	for _, w := range []Range{{2, 5}, {7, 8}, {9, 17}, {10, 12}, {13, 15}} {
		if !has(rs, w.StartLine, w.EndLine) {
			t.Errorf("missing %v in %v", w, rs)
		}
	}
	for _, r := range rs {
		if r.EndLine <= r.StartLine {
			t.Errorf("single-line range %v", r)
		}
		if r.StartLine == 0 {
			t.Errorf("the whole file folds: %v", r)
		}
	}
}

func TestRangesPython(t *testing.T) {
	src := "class A:\n    def f(self):\n        return 1\n\n    def g(self):\n        pass\n"
	rs := parseRanges(t, "python", src)
	if !has(rs, 0, 5) || !has(rs, 1, 2) || !has(rs, 4, 5) {
		t.Errorf("python: %v", rs)
	}
	// The body block of "def f" starts on line 2: no second fold there.
	for _, r := range rs {
		if r.StartLine == 2 {
			t.Errorf("body wrapper folds: %v", rs)
		}
	}
}

func TestRangesDataFormats(t *testing.T) {
	if rs := parseRanges(t, "json", "{\n  \"a\": [\n    1,\n    2\n  ]\n}\n"); !has(rs, 1, 4) {
		t.Errorf("json: %v", rs)
	}
	if rs := parseRanges(t, "yaml", "a:\n  b:\n    c: 1\n  d: 2\ne: 3\n"); !has(rs, 0, 3) || !has(rs, 1, 2) {
		t.Errorf("yaml: %v", rs)
	}
	if rs := parseRanges(t, "html", "<html>\n<body>\n<p>x</p>\n</body>\n</html>\n"); !has(rs, 1, 3) {
		t.Errorf("html: %v", rs)
	}
}

// Every mapped grammar loads and parses.
func TestAllGrammarsLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("loads every grammar")
	}
	for id, load := range grammars {
		lang := loadLanguage(load)
		if lang == nil {
			t.Errorf("%s: grammar did not load", id)
			continue
		}
		p := gotreesitter.NewParser(lang)
		tree, err := p.Parse([]byte("x\n"))
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		tree.Release()
	}
}

func TestUnsupported(t *testing.T) {
	for _, id := range []string{"batch", "sass", "systemd", "apache", "crontab", "conf", "reg", "rpmspec", "", "text"} {
		if Supported(id) || NewParser(id, nil) != nil {
			t.Errorf("%q folds", id)
		}
	}
}

func TestManagerMapping(t *testing.T) {
	var m Manager
	m.SetRanges([]Range{{2, 5}, {3, 4}, {8, 10}})
	if !m.Fold(3) || !m.Fold(2) || !m.Fold(8) {
		t.Fatal("fold")
	}
	// Lines 3..5 and 9..10 hidden.
	hidden := []int{}
	for l := 0; l < 12; l++ {
		if m.IsLineHidden(l) {
			hidden = append(hidden, l)
		}
	}
	if !reflect.DeepEqual(hidden, []int{3, 4, 5, 9, 10}) {
		t.Fatal("hidden", hidden)
	}
	if m.TotalVisualLines(12) != 7 {
		t.Error("total", m.TotalVisualLines(12))
	}
	vis := 0
	for l := 0; l < 12; l++ {
		if m.IsLineHidden(l) {
			continue
		}
		if v := m.DocLineToVisualLine(l); v != vis {
			t.Errorf("doc %d -> %d, want %d", l, v, vis)
		}
		if d := m.VisualLineToDocLine(vis); d != l {
			t.Errorf("visual %d -> %d, want %d", vis, d, l)
		}
		vis++
	}
	if m.DocLineToVisualLine(4) != 2 {
		t.Error("hidden line maps to its header", m.DocLineToVisualLine(4))
	}
	// Nested state survives the outer fold.
	m.Unfold(2)
	if !m.IsFolded(3) || m.IsLineHidden(3) || !m.IsLineHidden(4) {
		t.Error("inner fold lost", m.FoldedRanges())
	}
	m.Fold(2)
	m.Reveal(4) // a jump into the inner block opens both
	if m.IsLineHidden(4) || m.IsFolded(2) || m.IsFolded(3) || !m.IsFolded(8) {
		t.Error("reveal", m.FoldedRanges())
	}
	m.UnfoldAll()
	if m.HasFolds() || m.TotalVisualLines(12) != 12 {
		t.Error("unfold all")
	}
	m.FoldAll()
	if len(m.FoldedRanges()) != 3 {
		t.Error("fold all", m.FoldedRanges())
	}
}

func TestToggleAt(t *testing.T) {
	var m Manager
	m.SetRanges([]Range{{0, 10}, {2, 5}})
	if r, folded, ok := m.ToggleAt(3); !ok || !folded || r != (Range{2, 5}) {
		t.Error("innermost", r, folded, ok)
	}
	if r, folded, ok := m.ToggleAt(2); !ok || folded || r != (Range{2, 5}) {
		t.Error("unfold header", r, folded, ok)
	}
	if _, _, ok := m.ToggleAt(20); ok {
		t.Error("outside")
	}
}

func lines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line%d\n", i)
	}
	return b.String()
}

func TestEditsMoveFolds(t *testing.T) {
	text := lines(10)
	setup := func() *Manager {
		m := &Manager{}
		m.SetRanges([]Range{{3, 6}, {1, 8}})
		m.Fold(3)
		return m
	}
	ls := strings.SplitAfter(text, "\n")
	join := func(parts ...string) string { return strings.Join(parts, "") }

	// Two lines inserted above the fold: it moves down.
	m := setup()
	m.TextChanged(text, join(join(ls[:1]...), "a\nb\n", join(ls[1:]...)))
	if !m.IsFolded(5) || m.FoldedRanges()[0] != (Range{5, 8}) {
		t.Error("insert above", m.FoldedRanges())
	}
	if !has(m.Ranges(), 1+2, 8+2) && !has(m.Ranges(), 1, 10) {
		t.Error("enclosing range", m.Ranges())
	}
	// A line deleted above.
	m = setup()
	m.TextChanged(text, join(join(ls[:1]...), join(ls[2:]...)))
	if !m.IsFolded(2) {
		t.Error("delete above", m.FoldedRanges())
	}
	// A new line right before the header.
	m = setup()
	m.TextChanged(text, join(join(ls[:3]...), "new\n", join(ls[3:]...)))
	if !m.IsFolded(4) {
		t.Error("insert before header", m.FoldedRanges())
	}
	// Typing in the header keeps the fold.
	m = setup()
	m.TextChanged(text, strings.Replace(text, "line3", "line3 // x", 1))
	if !m.IsFolded(3) {
		t.Error("header edit", m.FoldedRanges())
	}
	// An edit inside the hidden lines unfolds.
	m = setup()
	m.TextChanged(text, strings.Replace(text, "line5", "changed", 1))
	if m.HasFolds() {
		t.Error("edit inside", m.FoldedRanges())
	}
	// The header deleted: unfolds.
	m = setup()
	m.TextChanged(text, join(join(ls[:3]...), join(ls[4:]...)))
	if m.HasFolds() {
		t.Error("header deleted", m.FoldedRanges())
	}
	// A selection over the fold deleted (lines 2..7): unfolds, nothing hidden.
	m = setup()
	m.TextChanged(text, join(join(ls[:2]...), join(ls[8:]...)))
	if m.HasFolds() || m.IsLineHidden(4) {
		t.Error("deleted over", m.FoldedRanges())
	}
	// An edit below the fold changes nothing.
	m = setup()
	m.TextChanged(text, join(text, "more\n"))
	if !m.IsFolded(3) || m.FoldedRanges()[0] != (Range{3, 6}) {
		t.Error("edit below", m.FoldedRanges())
	}
}

func TestInputEdit(t *testing.T) {
	e := inputEdit([]byte("ab\ncd\n"), []byte("ab\nXYcd\n"))
	if e.StartByte != 3 || e.OldEndByte != 3 || e.NewEndByte != 5 ||
		e.StartPoint != (gotreesitter.Point{Row: 1}) || e.NewEndPoint != (gotreesitter.Point{Row: 1, Column: 2}) {
		t.Errorf("%+v", e)
	}
	e = inputEdit([]byte("a\nb\nc"), []byte("a\nc"))
	if e.StartByte != 2 || e.OldEndByte != 4 || e.NewEndByte != 2 || e.OldEndPoint != (gotreesitter.Point{Row: 2}) {
		t.Errorf("%+v", e)
	}
}

func waitResult(t *testing.T, p *Parser, gen uint64) Result {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := p.Result(); ok && r.Gen == gen {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no result for", gen)
	return Result{}
}

// The incremental parses give the same ranges as a full parse; updates in
// quick succession from another goroutine are safe and only the latest one
// counts.
func TestParserIncremental(t *testing.T) {
	p := NewParser("go", func() {})
	defer p.Close()
	p.delay = 10 * time.Millisecond
	src := "package x\n\nfunc a() {\n\treturn\n}\n"
	p.Update(1, src)
	if r := waitResult(t, p, 1); !has(r.Ranges, 2, 4) {
		t.Fatal("first", r.Ranges)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := src
		for i := 0; i < 50; i++ {
			s += fmt.Sprintf("func f%d() {\n\tx := %d\n\t_ = x\n}\n", i, i)
			p.Update(uint64(i+2), s)
		}
	}()
	wg.Wait()
	final := src
	for i := 0; i < 50; i++ {
		final += fmt.Sprintf("func f%d() {\n\tx := %d\n\t_ = x\n}\n", i, i)
	}
	r := waitResult(t, p, 51)
	if want := parseRanges(t, "go", final); !reflect.DeepEqual(r.Ranges, want) {
		t.Errorf("incremental %v\nfull %v", r.Ranges, want)
	}
	// An edit in the middle.
	edited := strings.Replace(final, "x := 20\n", "x := 20\n\tif x > 1 {\n\t\tx++\n\t}\n", 1)
	p.Update(52, edited)
	r = waitResult(t, p, 52)
	if want := parseRanges(t, "go", edited); !reflect.DeepEqual(r.Ranges, want) {
		t.Errorf("edit: incremental %v\nfull %v", r.Ranges, want)
	}
}
