package termview

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// Plain-text search over the buffer, line by line
// (frontend/src/lib/terminal/textSearch.ts); matches never span lines.

// SearchOptions are the find bar's options.
type SearchOptions struct {
	CaseSensitive, WholeWord, Regex bool
}

// Matcher finds a pattern in a line.
type Matcher struct {
	re        *regexp.Regexp
	wholeWord bool
}

// NewMatcher builds the matcher for the find bar; nil for an empty or
// invalid pattern.
func NewMatcher(text string, o SearchOptions) *Matcher {
	if text == "" {
		return nil
	}
	src := text
	if !o.Regex {
		src = regexp.QuoteMeta(text)
	}
	if !o.CaseSensitive {
		src = "(?i)" + src
	}
	re, err := regexp.Compile(src)
	if err != nil {
		return nil
	}
	return &Matcher{re: re, wholeWord: o.WholeWord}
}

func isWordByte(s string, i int, before bool) bool {
	var r rune
	if before {
		r, _ = utf8.DecodeLastRuneInString(s[:i])
	} else {
		r, _ = utf8.DecodeRuneInString(s[i:])
	}
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// all returns the byte ranges of the matches in s.
func (m *Matcher) all(s string) [][2]int {
	var out [][2]int
	for pos := 0; pos <= len(s); {
		loc := m.re.FindStringIndex(s[pos:])
		if loc == nil {
			break
		}
		a, b := pos+loc[0], pos+loc[1]
		if a == b { // empty match
			_, n := utf8.DecodeRuneInString(s[a:])
			pos = a + max(1, n)
			continue
		}
		if m.wholeWord && (a > 0 && isWordByte(s, a, true) || b < len(s) && isWordByte(s, b, false)) {
			_, n := utf8.DecodeRuneInString(s[a:])
			pos = a + max(1, n)
			continue
		}
		out = append(out, [2]int{a, b})
		pos = b
	}
	return out
}

// Match reports whether the pattern occurs in s.
func (m *Matcher) Match(s string) bool { return len(m.all(s)) > 0 }

// SearchRow is a buffer line as text; Start and End are the cell columns
// covered by each byte of Text.
type SearchRow struct {
	Text       string
	Start, End []int
}

// searchRow converts cells (wide characters cover two columns).
func searchRow(cells []term.Cell) SearchRow {
	var r SearchRow
	var sb strings.Builder
	for x, c := range cells {
		if c.Width == 0 {
			continue
		}
		s := c.Text
		if s == "" {
			s = " "
		}
		end := x + max(1, int(c.Width)) - 1
		sb.WriteString(s)
		for range len(s) {
			r.Start = append(r.Start, x)
			r.End = append(r.End, end)
		}
	}
	r.Text = sb.String()
	return r
}

// SearchMatch is a match: columns Col to EndCol (inclusive) of a buffer
// line.
type SearchMatch struct{ Row, Col, EndCol int }

func rowMatches(r SearchRow, row int, m *Matcher) []SearchMatch {
	var out []SearchMatch
	for _, loc := range m.all(r.Text) {
		out = append(out, SearchMatch{Row: row, Col: r.Start[loc[0]], EndCol: r.End[loc[1]-1]})
	}
	return out
}

// FindMatch finds the next (or previous) match after (before) from,
// wrapping around the buffer once. Forward search from column -1 includes
// matches at the start of from.Row.
func FindMatch(getRow func(int) (SearchRow, bool), rowCount int, m *Matcher, from Pos, backward bool) (SearchMatch, bool) {
	if rowCount <= 0 || m == nil {
		return SearchMatch{}, false
	}
	start := max(0, min(rowCount-1, from.Row))
	for step := 0; step <= rowCount; step++ {
		r := (start + step) % rowCount
		if backward {
			r = ((start-step)%rowCount + rowCount) % rowCount
		}
		row, ok := getRow(r)
		if !ok {
			continue
		}
		var ms []SearchMatch
		for _, mt := range rowMatches(row, r, m) {
			keep := true
			switch step {
			case 0:
				keep = backward && mt.Col < from.Col || !backward && mt.Col > from.Col
			case rowCount:
				// Back at the start line after wrapping: the part step 0 skipped.
				keep = backward && mt.Col >= from.Col || !backward && mt.Col <= from.Col
			}
			if keep {
				ms = append(ms, mt)
			}
		}
		if len(ms) > 0 {
			if backward {
				return ms[len(ms)-1], true
			}
			return ms[0], true
		}
	}
	return SearchMatch{}, false
}

// Searcher keeps the find bar's position between searches
// (ghosttyEngine.ts's find).
type Searcher struct {
	last    *SearchMatch
	lastKey string
}

// Find selects and reveals the next (previous) match in v; false when there
// is none.
func (s *Searcher) Find(v *View, text string, o SearchOptions, backward bool) bool {
	m := NewMatcher(text, o)
	key := text + "\x00" + string(rune('0'+b2i(o.CaseSensitive))) + string(rune('0'+b2i(o.WholeWord))) + string(rune('0'+b2i(o.Regex)))
	changed := key != s.lastKey
	s.lastKey = key
	if m == nil {
		return false
	}
	var f term.Frame
	v.Engine.Snapshot(&f)
	rows := v.Engine.Lines()
	top := f.History - f.Offset
	if s.last != nil && s.last.Row >= rows {
		s.last = nil
	}
	var from Pos
	switch {
	case s.last != nil:
		// A changed query checks the current match position again
		// (incremental typing).
		from = Pos{Row: s.last.Row, Col: s.last.Col}
		if changed {
			if backward {
				from.Col++
			} else {
				from.Col--
			}
		}
	case backward:
		from = Pos{Row: top + f.Rows - 1, Col: 1 << 30}
	default:
		from = Pos{Row: top, Col: -1}
	}
	var buf []term.Cell
	match, ok := FindMatch(func(r int) (SearchRow, bool) {
		buf = v.Engine.Line(r, buf[:0])
		return searchRow(buf), len(buf) > 0
	}, rows, m, from, backward)
	if !ok {
		s.last = nil
		return false
	}
	s.last = &match
	v.Select(&Selection{Anchor: Pos{match.Row, match.Col}, Head: Pos{match.Row, match.EndCol}})
	return true
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
