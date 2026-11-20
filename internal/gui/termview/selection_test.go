package termview

import (
	"testing"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

func cellsOf(s string, cols int) []term.Cell {
	out := make([]term.Cell, cols)
	i := 0
	for _, r := range s {
		if i >= cols {
			break
		}
		if r != ' ' {
			out[i].Text = string(r)
		}
		out[i].Width = 1
		i++
	}
	for ; i < cols; i++ {
		out[i].Width = 1
	}
	return out
}

func buffer(lines ...string) func(int) []term.Cell {
	return func(r int) []term.Cell {
		if r < 0 || r >= len(lines) {
			return nil
		}
		return cellsOf(lines[r], 20)
	}
}

func TestSelectionText(t *testing.T) {
	buf := buffer("first line", "  /usr/local/bin x", "third")
	for _, tc := range []struct {
		name string
		sel  Selection
		want string
	}{
		{"chars on one line", Selection{Anchor: Pos{0, 6}, Head: Pos{0, 9}}, "line"},
		{"backwards across lines", Selection{Anchor: Pos{1, 3}, Head: Pos{0, 6}}, "line\n  /u"},
		{"word with path chars", Selection{Anchor: Pos{1, 5}, Head: Pos{1, 5}, Mode: SelectWords}, "/usr/local/bin"},
		{"word on a blank", Selection{Anchor: Pos{1, 0}, Head: Pos{1, 0}, Mode: SelectWords}, ""},
		{"lines", Selection{Anchor: Pos{0, 3}, Head: Pos{2, 1}, Mode: SelectLines}, "first line\n  /usr/local/bin x\nthird"},
		{"rectangle", Selection{Anchor: Pos{0, 0}, Head: Pos{2, 3}, Rect: true}, "firs\n  /u\nthir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sel.Text(buf, DefaultWordChars, 20); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectionContains(t *testing.T) {
	s := Selection{Anchor: Pos{2, 5}, Head: Pos{0, 3}}
	from, to := s.bounds(buffer("a", "b", "c"), DefaultWordChars, 20)
	for _, tc := range []struct {
		p    Pos
		want bool
	}{
		{Pos{0, 2}, false}, {Pos{0, 3}, true}, {Pos{1, 0}, true}, {Pos{1, 19}, true},
		{Pos{2, 5}, true}, {Pos{2, 6}, false}, {Pos{3, 0}, false},
	} {
		if got := s.contains(tc.p, from, to); got != tc.want {
			t.Errorf("contains(%v) = %v", tc.p, got)
		}
	}
}
