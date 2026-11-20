package leanedit

import (
	"reflect"
	"testing"
)

func run(text string, ch []Change) string {
	out, _ := apply(text, ch)
	return out
}

// The cases of commands.test.ts.
func TestChangeCase(t *testing.T) {
	ch, _ := changeCase("abc", Sel{0, 3}, "upper")
	if got := run("abc", ch); got != "ABC" {
		t.Error(got)
	}
	ch, _ = changeCase("ABC", Sel{0, 3}, "lower")
	if got := run("ABC", ch); got != "abc" {
		t.Error(got)
	}
	ch, s := changeCase("abc", Sel{1, 1}, "upper")
	if got := run("abc", ch); got != "aBc" || s.Head != 2 {
		t.Error(got, s)
	}
	ch, _ = changeCase("hello big world", Sel{0, 15}, "capitalize")
	if got := run("hello big world", ch); got != "Hello Big World" {
		t.Error(got)
	}
	ch, _ = changeCase("hello world", Sel{8, 8}, "capitalize")
	if got := run("hello world", ch); got != "hello World" {
		t.Error(got)
	}
}

func TestJoinLines(t *testing.T) {
	for _, c := range []struct {
		in   string
		s    Sel
		want string
	}{{"a\n    b\nc", Sel{0, 0}, "a b\nc"}, {"a\n  b\n  c", Sel{0, 9}, "a b c"}, {"a \nb", Sel{0, 0}, "a b"}} {
		if got := run(c.in, joinLines(c.in, c.s)); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIndentation(t *testing.T) {
	mixed := "\tone\n    two\n  \tthree"
	if got := run(mixed, convertIndentation(mixed, Sel{}, false, 4)); got != "    one\n    two\n    three" {
		t.Error(got)
	}
	if got := run(mixed, convertIndentation(mixed, Sel{}, true, 4)); got != "\tone\n\ttwo\n\tthree" {
		t.Error(got)
	}
	if got := run("\ta\n\tb", convertIndentation("\ta\n\tb", Sel{0, 2}, false, 4)); got != "    a\n\tb" {
		t.Error(got)
	}
	if got := run("a\n  b", indentChanges("a\n  b", Sel{0, 5}, 1, "    ", 4)); got != "    a\n      b" {
		t.Errorf("indent: %q", got)
	}
	if got := run("      b", indentChanges("      b", Sel{0, 0}, -1, "    ", 4)); got != "    b" {
		t.Errorf("unindent: %q", got)
	}
}

func TestAlignAndWrap(t *testing.T) {
	in := "a = 1\nlong = 2\nxy= 3"
	if got := run(in, alignOn(in, Sel{}, "=", 4)); got != "a    = 1\nlong = 2\nxy   = 3" {
		t.Error(got)
	}
	in = "a: 1\nbbb:  2"
	if got := run(in, alignOn(in, Sel{}, `:\s*(\S)`, 4)); got != "a:    1\nbbb:  2" {
		t.Error(got)
	}
	in = "f(x\nfoo(y"
	if got := run(in, alignOn(in, Sel{}, "(", 4)); got != "f  (x\nfoo(y" {
		t.Error(got)
	}
	if got := wrapLine("  aaa bbb ccc ddd", 10, 4); !reflect.DeepEqual(got, []string{"  aaa bbb", "  ccc ddd"}) {
		t.Error(got)
	}
	if got := wrapLine("short", 10, 4); !reflect.DeepEqual(got, []string{"short"}) {
		t.Error(got)
	}
	if got := wrapLine("averyveryverylongword x", 10, 4); !reflect.DeepEqual(got, []string{"averyveryverylongword", "x"}) {
		t.Error(got)
	}
	if got := run("aaa bbb ccc ddd", applyWordWrap("aaa bbb ccc ddd", Sel{}, 10, 4)); got != "aaa bbb\nccc ddd" {
		t.Error(got)
	}
}

func TestCommentsBracketsOccurrences(t *testing.T) {
	in := "a\n  b\n"
	out := run(in, toggleComment(in, Sel{0, 4}, "go"))
	if out != "// a\n//   b\n" && out != "// a\n  // b\n" {
		t.Errorf("comment: %q", out)
	}
	in = "  x\n  y"
	out = run(in, toggleComment(in, Sel{0, 7}, "python"))
	if out != "  # x\n  # y" {
		t.Errorf("comment: %q", out)
	}
	if back := run(out, toggleComment(out, Sel{0, len([]rune(out))}, "python")); back != in {
		t.Errorf("uncomment: %q", back)
	}
	if out := run("<p>", toggleComment("<p>", Sel{}, "html")); out != "<!-- <p> -->" {
		t.Errorf("block: %q", out)
	}
	if m, at, ok := matchingBracket("f(a[1])", 1); !ok || m != 6 || at != 1 {
		t.Error("bracket", m, at, ok)
	}
	if m, _, ok := matchingBracket("f(a[1])", 7); !ok || m != 1 {
		t.Error("bracket before cursor", m, ok)
	}
	if s, ok := nextOccurrence("ab x ab", Sel{0, 2}); !ok || s != (Sel{5, 7}) {
		t.Error("occurrence", s)
	}
	if s, _ := nextOccurrence("hello world", Sel{2, 2}); s != (Sel{0, 5}) {
		t.Error("word", s)
	}
}

func TestApplyMapsPositions(t *testing.T) {
	out, mp := apply("abcdef", []Change{{1, 1, "XY"}, {3, 5, "z"}})
	if out != "aXYbczf" || mp(0, 1) != 0 || mp(2, 1) != 4 || mp(6, 1) != 7 {
		t.Error(out, mp(0, 1), mp(2, 1), mp(6, 1))
	}
}

// The location history cases of commands.test.ts.
func TestLocationHistory(t *testing.T) {
	same := func(a, b location) bool { return a.tab == b.tab && a.pos == b.pos }
	l := func(tab, pos int) location { return location{tab: tab, pos: pos} }
	h := newHistory(same)
	h.jump(l(1, 0), l(1, 100))
	h.jump(l(1, 100), l(2, 5))
	cur := l(2, 5)
	if b := h.back(&cur); b == nil || *b != l(1, 100) {
		t.Fatal("back 1", b)
	}
	cur = l(1, 100)
	if b := h.back(&cur); b == nil || *b != l(1, 0) {
		t.Fatal("back 2", b)
	}
	cur = l(1, 0)
	if b := h.back(&cur); b != nil {
		t.Fatal("back 3", b)
	}
	if f := h.forward(); f == nil || *f != l(1, 100) {
		t.Fatal("forward 1", f)
	}
	if f := h.forward(); f == nil || *f != l(2, 5) {
		t.Fatal("forward 2", f)
	}
	if f := h.forward(); f != nil {
		t.Fatal("forward 3", f)
	}

	h = newHistory(same)
	h.edit(l(1, 10))
	cur = l(1, 50)
	if b := h.back(&cur); b == nil || *b != l(1, 10) {
		t.Fatal("edit back", b)
	}
	if f := h.forward(); f == nil || *f != l(1, 50) {
		t.Fatal("edit forward", f)
	}

	h = newHistory(same)
	h.jump(l(1, 5), l(2, 7))
	h.removeTab(1)
	if len(h.list) != 1 || h.list[0] != l(2, 7) || h.index != 0 {
		t.Fatal("removeTab", h.list, h.index)
	}
}

func TestModifiedLines(t *testing.T) {
	got := modifiedLines("a\nb\nc\nd", "a\nb\nxc\nd")
	if len(got) != 1 || !got[3] {
		t.Error(got)
	}
	got = modifiedLines("a\nb\nc\nd", "new\na\nb\nxc\nd")
	if !got[1] || !got[4] {
		t.Error(got)
	}
}

func TestKWriteSaveAndIndent(t *testing.T) {
	text := "a  \nb\t\nc"
	if got := run(text, saveCleanup(text, "modified", map[int]bool{1: true}, true)); got != "a\nb\t\nc\n" {
		t.Errorf("modified: %q", got)
	}
	if got := run(text, saveCleanup(text, "all", nil, false)); got != "a\nb\nc" {
		t.Errorf("all: %q", got)
	}
	if ch := saveCleanup("x\n", "never", nil, true); len(ch) != 0 {
		t.Errorf("never: %v", ch)
	}
	if f, sp, w := detectIndent("func x() {\n\treturn\n}\n"); !f || sp {
		t.Error("tabs", f, sp, w)
	}
	if f, sp, w := detectIndent("a:\n  b:\n    c: 1\n  d: 2\n"); !f || !sp || w != 2 {
		t.Error("spaces", f, sp, w)
	}
	if f, _, _ := detectIndent("x\ny\n"); f {
		t.Error("none")
	}
}
