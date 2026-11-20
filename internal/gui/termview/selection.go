package termview

import (
	"strings"
	"unicode"

	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// Pos is a cell in buffer coordinates: Row counts from the oldest history
// line ([term.Engine.Line]), Col from the left.
type Pos struct{ Row, Col int }

func (p Pos) before(q Pos) bool { return p.Row < q.Row || p.Row == q.Row && p.Col < q.Col }

// SelectMode is how a selection grows: by characters, words (double click)
// or lines (triple click).
type SelectMode uint8

const (
	SelectChars SelectMode = iota
	SelectWords
	SelectLines
)

// Selection is a selected range of the buffer. Rect selects the same
// columns on every row (Alt+drag).
type Selection struct {
	Anchor, Head Pos
	Mode         SelectMode
	Rect         bool
}

// DefaultWordChars are the characters besides letters and digits that a
// double click selects as part of a word (paths, URLs, options).
const DefaultWordChars = "-,./?%&#:_=+@~"

func isWordRune(r rune, extra string) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(extra, r)
}

// span is the selected part of a line: columns [Start, End] (End inclusive).
type span struct{ Start, End int }

// bounds orders the selection and widens it to words or lines; line reads
// a buffer line for word boundaries.
func (s Selection) bounds(line func(int) []term.Cell, wordChars string, cols int) (from, to Pos) {
	from, to = s.Anchor, s.Head
	if to.before(from) {
		from, to = to, from
	}
	switch s.Mode {
	case SelectWords:
		from.Col, _ = wordAt(line(from.Row), from.Col, wordChars)
		_, to.Col = wordAt(line(to.Row), to.Col, wordChars)
	case SelectLines:
		from.Col, to.Col = 0, cols-1
	}
	return from, to
}

// wordAt returns the columns of the word at col (just col when it is not
// part of a word).
func wordAt(cells []term.Cell, col int, extra string) (start, end int) {
	if col < 0 || col >= len(cells) {
		return col, col
	}
	isWord := func(i int) bool {
		t := cells[i].Text
		if cells[i].Width == 0 && i > 0 {
			t = cells[i-1].Text // the second half of a wide character
		}
		r := []rune(t)
		return len(r) > 0 && isWordRune(r[0], extra)
	}
	if !isWord(col) {
		return col, col
	}
	start, end = col, col
	for start > 0 && isWord(start-1) {
		start--
	}
	for end < len(cells)-1 && isWord(end+1) {
		end++
	}
	return start, end
}

// Contains reports whether a cell is selected.
func (s Selection) contains(p Pos, from, to Pos) bool {
	if p.Row < from.Row || p.Row > to.Row {
		return false
	}
	if s.Rect {
		lo, hi := min(s.Anchor.Col, s.Head.Col), max(s.Anchor.Col, s.Head.Col)
		return p.Col >= lo && p.Col <= hi
	}
	if p.Row == from.Row && p.Col < from.Col {
		return false
	}
	if p.Row == to.Row && p.Col > to.Col {
		return false
	}
	return true
}

// rowSpan is the selected part of a row, ok false when none.
func (s Selection) rowSpan(row, cols int, from, to Pos) (sp span, ok bool) {
	if row < from.Row || row > to.Row {
		return span{}, false
	}
	if s.Rect {
		return span{min(s.Anchor.Col, s.Head.Col), max(s.Anchor.Col, s.Head.Col)}, true
	}
	sp = span{0, cols - 1}
	if row == from.Row {
		sp.Start = from.Col
	}
	if row == to.Row {
		sp.End = to.Col
	}
	return sp, sp.Start <= sp.End
}

// Text is the selected text: lines joined with "\n", trailing blanks of
// each line removed.
func (s Selection) Text(line func(int) []term.Cell, wordChars string, cols int) string {
	from, to := s.bounds(line, wordChars, cols)
	var lines []string
	for r := from.Row; r <= to.Row; r++ {
		cells := line(r)
		sp, ok := s.rowSpan(r, max(cols, len(cells)), from, to)
		if !ok {
			continue
		}
		lo, hi := max(0, sp.Start), min(len(cells)-1, sp.End)
		if lo > hi {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, term.Text(cells[lo:hi+1]))
	}
	return strings.Join(lines, "\n")
}
