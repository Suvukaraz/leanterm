package folding

import (
	"sort"
	"strings"
)

// Manager holds a document's foldable ranges and which of them are folded,
// and maps between document lines (the lines of the text) and visual lines
// (the lines on screen, without the hidden ones). It is not safe for
// concurrent use: the editor owns it on the UI goroutine.
//
// Folds are independent of each other: folding an outer block hides the
// inner ones with it, and unfolding it again shows them as they were (still
// folded or not).
type Manager struct {
	ranges []Range // the foldable ranges, sorted by start line
	folded []Range // the folded ranges, sorted by start line

	spans   []Range // the merged hidden lines (StartLine..EndLine hidden), sorted
	prefix  []int   // prefix[i]: the hidden lines in spans[:i]
	hidden  int     // all hidden lines
	version int     // changes with every change of the folded set
}

// Version changes whenever the hidden lines change.
func (m *Manager) Version() int { return m.version }

// Ranges are the foldable ranges, sorted by start line.
func (m *Manager) Ranges() []Range { return m.ranges }

// FoldedRanges are the folded ranges, sorted by start line.
func (m *Manager) FoldedRanges() []Range { return m.folded }

// HasFolds reports whether any range is folded.
func (m *Manager) HasFolds() bool { return len(m.folded) > 0 }

// SetRanges replaces the foldable ranges (a new parse). Folded ranges whose
// start line is still a foldable range keep their state (with the new end
// line); folds that the parse no longer finds (a syntax error while typing
// elsewhere) stay folded until they are unfolded or edited.
func (m *Manager) SetRanges(rs []Range) {
	m.ranges = normalize(rs)
	if len(m.folded) == 0 {
		return
	}
	changed := false
	for i, f := range m.folded {
		if r, ok := find(m.ranges, f.StartLine); ok && r.EndLine != f.EndLine {
			m.folded[i].EndLine = r.EndLine
			changed = true
		}
	}
	if changed {
		m.rebuild()
	}
}

// normalize sorts the ranges and keeps the multi-line ones, one per start
// line (the longest).
func normalize(rs []Range) []Range {
	out := make([]Range, 0, len(rs))
	for _, r := range rs {
		if r.EndLine > r.StartLine && r.StartLine >= 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].EndLine > out[j].EndLine
	})
	kept := out[:0]
	for _, r := range out {
		if len(kept) > 0 && kept[len(kept)-1].StartLine == r.StartLine {
			continue
		}
		kept = append(kept, r)
	}
	return kept
}

func find(rs []Range, start int) (Range, bool) {
	i := sort.Search(len(rs), func(i int) bool { return rs[i].StartLine >= start })
	if i < len(rs) && rs[i].StartLine == start {
		return rs[i], true
	}
	return Range{}, false
}

// RangeAt is the foldable (or folded) range that starts at line.
func (m *Manager) RangeAt(line int) (Range, bool) {
	if r, ok := find(m.folded, line); ok {
		return r, true
	}
	return find(m.ranges, line)
}

// IsFolded reports whether the range starting at line is folded.
func (m *Manager) IsFolded(line int) bool {
	_, ok := find(m.folded, line)
	return ok
}

// Fold folds the range starting at line.
func (m *Manager) Fold(line int) bool {
	if m.IsFolded(line) {
		return false
	}
	r, ok := find(m.ranges, line)
	if !ok {
		return false
	}
	m.folded = append(m.folded, r)
	sort.Slice(m.folded, func(i, j int) bool { return m.folded[i].StartLine < m.folded[j].StartLine })
	m.rebuild()
	return true
}

// Unfold unfolds the range starting at line.
func (m *Manager) Unfold(line int) bool {
	for i, f := range m.folded {
		if f.StartLine == line {
			m.folded = append(m.folded[:i], m.folded[i+1:]...)
			m.rebuild()
			return true
		}
	}
	return false
}

// Toggle folds or unfolds the range starting at line (a click on the fold
// marker).
func (m *Manager) Toggle(line int) bool {
	if m.Unfold(line) {
		return true
	}
	return m.Fold(line)
}

// ToggleAt is "Toggle Current Block" for the cursor line: a folded range
// starting at line unfolds; otherwise the innermost range around line
// folds. It returns the range and whether it is folded now.
func (m *Manager) ToggleAt(line int) (Range, bool, bool) {
	if r, ok := find(m.folded, line); ok {
		m.Unfold(line)
		return r, false, true
	}
	var best Range
	found := false
	for _, r := range m.ranges {
		if r.StartLine > line {
			break
		}
		if r.EndLine >= line && !m.IsFolded(r.StartLine) && (!found || r.StartLine >= best.StartLine) {
			best, found = r, true
		}
	}
	if !found {
		return Range{}, false, false
	}
	m.Fold(best.StartLine)
	return best, true, true
}

// FoldAll folds every foldable range.
func (m *Manager) FoldAll() {
	m.folded = append([]Range(nil), m.ranges...)
	m.rebuild()
}

// UnfoldAll unfolds everything.
func (m *Manager) UnfoldAll() {
	if len(m.folded) == 0 {
		return
	}
	m.folded = nil
	m.rebuild()
}

// Reveal unfolds every fold that hides line (a jump to it). It reports
// whether something unfolded.
func (m *Manager) Reveal(line int) bool {
	kept := m.folded[:0]
	changed := false
	for _, f := range m.folded {
		if f.Contains(line) {
			changed = true
			continue
		}
		kept = append(kept, f)
	}
	m.folded = kept
	if changed {
		m.rebuild()
	}
	return changed
}

// HidingFold is the outermost fold that hides line.
func (m *Manager) HidingFold(line int) (Range, bool) {
	for _, f := range m.folded {
		if f.StartLine >= line {
			break
		}
		if f.Contains(line) && !m.IsLineHidden(f.StartLine) {
			return f, true
		}
	}
	return Range{}, false
}

// rebuild recomputes the hidden spans.
func (m *Manager) rebuild() {
	m.version++
	m.spans = m.spans[:0]
	for _, f := range m.folded {
		from, to := f.StartLine+1, f.EndLine
		if to < from {
			continue
		}
		if n := len(m.spans); n > 0 && from <= m.spans[n-1].EndLine+1 {
			if to > m.spans[n-1].EndLine {
				m.spans[n-1].EndLine = to
			}
			continue
		}
		m.spans = append(m.spans, Range{StartLine: from, EndLine: to})
	}
	m.prefix = m.prefix[:0]
	m.hidden = 0
	for _, s := range m.spans {
		m.prefix = append(m.prefix, m.hidden)
		m.hidden += s.EndLine - s.StartLine + 1
	}
}

// HiddenSpans are the hidden lines as merged, sorted ranges (StartLine to
// EndLine hidden, inclusive).
func (m *Manager) HiddenSpans() []Range { return m.spans }

// span is the index of the span holding line, or -1.
func (m *Manager) span(line int) int {
	i := sort.Search(len(m.spans), func(i int) bool { return m.spans[i].EndLine >= line })
	if i < len(m.spans) && m.spans[i].StartLine <= line {
		return i
	}
	return -1
}

// IsLineHidden reports whether a document line is inside a folded range.
func (m *Manager) IsLineHidden(line int) bool { return m.span(line) >= 0 }

// hiddenBefore is the number of hidden lines before line.
func (m *Manager) hiddenBefore(line int) int {
	i := sort.Search(len(m.spans), func(i int) bool { return m.spans[i].StartLine >= line })
	if i == 0 {
		return 0
	}
	last := m.spans[i-1]
	n := m.prefix[i-1]
	if line > last.EndLine {
		return n + last.EndLine - last.StartLine + 1
	}
	return n + line - last.StartLine
}

// DocLineToVisualLine maps a document line to its visual line. A hidden
// line maps to the visual line of its fold's header.
func (m *Manager) DocLineToVisualLine(line int) int {
	if i := m.span(line); i >= 0 {
		line = m.spans[i].StartLine - 1
	}
	return line - m.hiddenBefore(line)
}

// VisualLineToDocLine maps a visual line to its document line.
func (m *Manager) VisualLineToDocLine(v int) int {
	// The spans before v are those whose first hidden line is at a visual
	// position <= v.
	j := sort.Search(len(m.spans), func(i int) bool { return m.spans[i].StartLine-m.prefix[i] > v })
	if j == 0 {
		return v
	}
	return v + m.prefix[j-1] + m.spans[j-1].EndLine - m.spans[j-1].StartLine + 1
}

// TotalVisualLines is the number of visual lines of a document with
// docLines lines.
func (m *Manager) TotalVisualLines(docLines int) int {
	return max(0, docLines-m.hiddenBefore(docLines))
}

// TextChanged moves the ranges with an edit from oldText to newText: ranges
// below the edit move with the inserted or deleted lines, ranges around it
// grow or shrink, and a fold whose hidden lines or line break the edit
// touches (or whose header it deletes) unfolds. An edit within the header
// line of a fold keeps it folded.
func (m *Manager) TextChanged(oldText, newText string) {
	if oldText == newText || (len(m.ranges) == 0 && len(m.folded) == 0) {
		return
	}
	p, q, ins := diff(oldText, newText)
	delta := strings.Count(ins, "\n") - strings.Count(oldText[p:q], "\n")
	ls := lineStarts(oldText)
	lineStart := func(l int) int {
		if l < len(ls) {
			return ls[l]
		}
		return len(oldText) + 1
	}
	// The offsets of a range: its header line [hs, hn] (hn is the newline)
	// and the end of its last line be (the newline or the end of the text).
	offsets := func(r Range) (hs, hn, be int) {
		return lineStart(r.StartLine), lineStart(r.StartLine+1) - 1, lineStart(r.EndLine+1) - 1
	}
	before := func(r Range) bool { // the edit is above the range: it moves
		hs, _, _ := offsets(r)
		return q < hs || (q == hs && p == q)
	}
	after := func(r Range) bool { // the edit is below the range
		_, _, be := offsets(r)
		return p > be
	}
	inHeader := func(r Range) bool {
		hs, hn, _ := offsets(r)
		return delta == 0 && hs <= p && q <= hn && !strings.Contains(ins, "\n") && !strings.Contains(oldText[p:q], "\n")
	}
	inside := func(r Range) bool { // the edit is within the range below its header
		_, hn, be := offsets(r)
		return p > hn && q <= be
	}
	shift := func(r Range) Range { return Range{StartLine: r.StartLine + delta, EndLine: r.EndLine + delta} }

	folded := m.folded[:0]
	foldChanged := false
	for _, f := range m.folded {
		switch {
		case after(f) || inHeader(f):
			folded = append(folded, f)
		case before(f):
			folded = append(folded, shift(f))
			foldChanged = foldChanged || delta != 0
		default:
			foldChanged = true // the edit touches the fold: it unfolds
		}
	}
	m.folded = folded

	ranges := m.ranges[:0]
	for _, r := range m.ranges {
		switch {
		case after(r) || inHeader(r):
			ranges = append(ranges, r)
		case before(r):
			ranges = append(ranges, shift(r))
		case inside(r):
			r.EndLine += delta
			if r.EndLine > r.StartLine {
				ranges = append(ranges, r)
			}
		}
	}
	m.ranges = normalize(ranges)
	if foldChanged {
		m.rebuild()
	}
}

// diff is the changed part of an edit: oldText[p:q] became ins.
func diff(a, b string) (p, q int, ins string) {
	n := min(len(a), len(b))
	for p < n && a[p] == b[p] {
		p++
	}
	s := 0
	for s < n-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	return p, len(a) - s, b[p : len(b)-s]
}

func lineStarts(s string) []int {
	out := []int{0}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, i+1)
		}
	}
	return out
}
