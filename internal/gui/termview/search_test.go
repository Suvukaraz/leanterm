package termview

import (
	"testing"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// Ported from frontend/src/lib/terminal/textSearch.test.ts.

func plainRow(s string) SearchRow {
	r := SearchRow{Text: s}
	for i := range len(s) {
		r.Start = append(r.Start, i)
		r.End = append(r.End, i)
	}
	return r
}

func TestMatcher(t *testing.T) {
	o := SearchOptions{}
	if NewMatcher("a.b", o).Match("aXb") {
		t.Error("plain text not escaped")
	}
	if !NewMatcher("ABC", o).Match("xabcx") {
		t.Error("case-insensitive search failed")
	}
	if NewMatcher("ABC", SearchOptions{CaseSensitive: true}).Match("xabcx") {
		t.Error("case-sensitive search matched")
	}
	word := NewMatcher("log", SearchOptions{WholeWord: true})
	if word.Match("syslog") || !word.Match("a log b") || !word.Match("syslog log") {
		t.Error("whole word")
	}
	if !NewMatcher("er+or", SearchOptions{Regex: true}).Match("errrror") {
		t.Error("regular expression")
	}
	if NewMatcher("", o) != nil || NewMatcher("(", SearchOptions{Regex: true}) != nil {
		t.Error("empty or invalid pattern accepted")
	}
}

func TestFindMatch(t *testing.T) {
	rows := []SearchRow{plainRow("foo bar"), plainRow("nothing"), plainRow("bar foo bar")}
	get := func(i int) (SearchRow, bool) { return rows[i], true }
	m := NewMatcher("bar", SearchOptions{})
	for _, tc := range []struct {
		from     Pos
		backward bool
		want     SearchMatch
	}{
		{Pos{0, -1}, false, SearchMatch{0, 4, 6}},
		{Pos{0, 4}, false, SearchMatch{2, 0, 2}},
		{Pos{2, 8}, false, SearchMatch{0, 4, 6}},
		{Pos{2, 8}, true, SearchMatch{2, 0, 2}},
		{Pos{2, 0}, true, SearchMatch{0, 4, 6}},
		{Pos{0, 4}, true, SearchMatch{2, 8, 10}},
	} {
		got, ok := FindMatch(get, 3, m, tc.from, tc.backward)
		if !ok || got != tc.want {
			t.Errorf("from %v backward %v: %v %v, want %v", tc.from, tc.backward, got, ok, tc.want)
		}
	}
	single := []SearchRow{plainRow("x"), plainRow("only bar")}
	if got, _ := FindMatch(func(i int) (SearchRow, bool) { return single[i], true }, 2, m, Pos{1, 5}, false); got != (SearchMatch{1, 5, 7}) {
		t.Errorf("only match after a full wrap: %v", got)
	}
	if _, ok := FindMatch(get, 3, NewMatcher("zzz", SearchOptions{}), Pos{0, -1}, false); ok {
		t.Error("match without the text")
	}
}

func TestSearchRowWideCharacters(t *testing.T) {
	// "日" covers columns 0-1, "x" column 2.
	r := searchRow([]term.Cell{{Text: "日", Width: 2}, {Width: 0}, {Text: "x", Width: 1}})
	got, ok := FindMatch(func(int) (SearchRow, bool) { return r, true }, 1, NewMatcher("日x", SearchOptions{}), Pos{0, -1}, false)
	if !ok || got != (SearchMatch{0, 0, 2}) {
		t.Fatalf("%v %v", got, ok)
	}
}
