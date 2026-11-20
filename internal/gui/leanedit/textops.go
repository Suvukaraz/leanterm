package leanedit

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// The text commands of commands.ts as pure functions: they work on the
// document as runes with one selection (gvcode has a single cursor) and
// return changes plus the new selection. Offsets are in runes.

// Sel is a selection: Anchor stays, Head is the cursor.
type Sel struct{ Anchor, Head int }

func (s Sel) from() int   { return min(s.Anchor, s.Head) }
func (s Sel) to() int     { return max(s.Anchor, s.Head) }
func (s Sel) empty() bool { return s.Anchor == s.Head }

// Change replaces [From, To) of the old text with Insert.
type Change struct {
	From, To int
	Insert   string
}

// line is one line of a document: [from, to) without the line break.
type line struct {
	n        int // 1-based
	from, to int
	text     string
}

type doc struct {
	r     []rune
	lines []line
}

func newDoc(text string) *doc {
	d := &doc{r: []rune(text)}
	start, n := 0, 1
	for i, c := range d.r {
		if c == '\n' {
			d.lines = append(d.lines, line{n, start, i, string(d.r[start:i])})
			start, n = i+1, n+1
		}
	}
	d.lines = append(d.lines, line{n, start, len(d.r), string(d.r[start:])})
	return d
}

func (d *doc) lineAt(pos int) line {
	i := sort.Search(len(d.lines), func(i int) bool { return d.lines[i].to >= pos })
	if i >= len(d.lines) {
		i = len(d.lines) - 1
	}
	return d.lines[i]
}

func (d *doc) line(n int) line { return d.lines[n-1] }

func (d *doc) slice(from, to int) string { return string(d.r[from:to]) }

// selectedLines: the lines the selection touches (a selection ending at a
// line start leaves that line out); all lines when empty and whole is set.
func (d *doc) selectedLines(s Sel, whole bool) []line {
	if whole && s.empty() {
		return d.lines
	}
	first := d.lineAt(s.from()).n
	last := d.lineAt(s.to()).n
	if last > first && d.line(last).from == s.to() {
		last--
	}
	return d.lines[first-1 : last]
}

// apply applies sorted, non-overlapping changes; mapPos maps old offsets
// (assoc -1 before an insertion at the position, 1 after).
func apply(text string, changes []Change) (string, func(pos, assoc int) int) {
	r := []rune(text)
	sort.Slice(changes, func(i, j int) bool { return changes[i].From < changes[j].From })
	var b strings.Builder
	last := 0
	for _, c := range changes {
		b.WriteString(string(r[last:c.From]))
		b.WriteString(c.Insert)
		last = c.To
	}
	b.WriteString(string(r[last:]))
	mapPos := func(pos, assoc int) int {
		delta := 0
		for _, c := range changes {
			n := len([]rune(c.Insert))
			switch {
			case pos < c.From || (pos == c.From && assoc < 0 && c.From == c.To):
				return pos + delta
			case pos > c.To || (pos == c.To && c.From != c.To) || (pos == c.From && assoc >= 0):
				delta += n - (c.To - c.From)
			default: // inside a replaced range
				return c.From + delta + n
			}
		}
		return pos + delta
	}
	return b.String(), mapPos
}

// ---- capitalisation ----------------------------------------------------------------------

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func transformCase(s, mode string) string {
	switch mode {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	}
	r := []rune(s)
	for i := range r {
		if unicode.IsLetter(r[i]) && (i == 0 || !isWordRune(r[i-1])) {
			r[i] = unicode.ToUpper(r[i])
		}
	}
	return string(r)
}

// wordAt is the word around pos (CodeMirror's wordAt), ok=false outside one.
func (d *doc) wordAt(pos int) (from, to int, ok bool) {
	from, to = pos, pos
	for from > 0 && isWordRune(d.r[from-1]) {
		from--
	}
	for to < len(d.r) && isWordRune(d.r[to]) {
		to++
	}
	return from, to, to > from
}

// changeCase changes the selection's case; without one KWrite changes the
// character right of the cursor (and moves on), or the word for capitalize.
func changeCase(text string, s Sel, mode string) ([]Change, Sel) {
	d := newDoc(text)
	from, to := s.from(), s.to()
	moveAfter := false
	if s.empty() {
		if mode == "capitalize" {
			f, t, ok := d.wordAt(s.Head)
			if !ok {
				return nil, s
			}
			from, to = f, t
		} else {
			if s.Head >= d.lineAt(s.Head).to {
				return nil, s
			}
			to = s.Head + 1
			moveAfter = true
		}
	}
	old := d.slice(from, to)
	ins := transformCase(old, mode)
	n := len([]rune(ins))
	var ns Sel
	switch {
	case s.empty() && moveAfter:
		ns = Sel{from + n, from + n}
	case s.empty():
		ns = s
	case s.Anchor == from:
		ns = Sel{from, from + n}
	default:
		ns = Sel{from + n, from}
	}
	if ins == old {
		return nil, ns
	}
	return []Change{{from, to, ins}}, ns
}

// ---- join lines --------------------------------------------------------------------------

var leadingWS = regexp.MustCompile(`^[ \t]*`)

// joinLines joins the selected lines, or the cursor line with the next.
func joinLines(text string, s Sel) []Change {
	d := newDoc(text)
	first := d.lineAt(s.from()).n
	last := d.lineAt(s.to()).n
	if last == first {
		last = first + 1
	}
	var out []Change
	for n := first; n < last && n < len(d.lines); n++ {
		l, next := d.line(n), d.line(n+1)
		indent := len([]rune(leadingWS.FindString(next.text)))
		space := l.text != "" && !strings.HasSuffix(l.text, " ") && !strings.HasSuffix(l.text, "\t") && len([]rune(next.text)) > indent
		ins := ""
		if space {
			ins = " "
		}
		out = append(out, Change{l.to, next.from + indent, ins})
	}
	return out
}

// ---- indentation -----------------------------------------------------------------------------

// countColumn is the column of rune offset upto in s (tabs to tabSize).
func countColumn(s string, tabSize, upto int) int {
	col := 0
	for i, r := range []rune(s) {
		if upto >= 0 && i >= upto {
			break
		}
		if r == '\t' {
			col += tabSize - col%tabSize
		} else {
			col++
		}
	}
	return col
}

// findColumn is the rune offset of column col in s.
func findColumn(s string, col, tabSize int) int {
	c := 0
	for i, r := range []rune(s) {
		if c >= col {
			return i
		}
		if r == '\t' {
			c += tabSize - c%tabSize
		} else {
			c++
		}
	}
	return len([]rune(s))
}

func buildIndent(width int, tabs bool, tabSize int) string {
	if tabs {
		return strings.Repeat("\t", width/tabSize) + strings.Repeat(" ", width%tabSize)
	}
	return strings.Repeat(" ", width)
}

// convertIndentation rewrites the leading whitespace of the selected lines
// (all lines without a selection) with tabs or spaces.
func convertIndentation(text string, s Sel, tabs bool, tabSize int) []Change {
	d := newDoc(text)
	var out []Change
	for _, l := range d.selectedLines(s, true) {
		cur := leadingWS.FindString(l.text)
		if cur == "" {
			continue
		}
		next := buildIndent(countColumn(cur, tabSize, -1), tabs, tabSize)
		if next != cur {
			out = append(out, Change{l.from, l.from + len([]rune(cur)), next})
		}
	}
	return out
}

// indentLines adds (dir 1) or removes (dir -1) one indent unit on the
// selected lines (CodeMirror's indentMore/indentLess).
func indentChanges(text string, s Sel, dir int, unit string, tabSize int) []Change {
	d := newDoc(text)
	unitW := countColumn(unit, tabSize, -1)
	var out []Change
	for _, l := range d.selectedLines(s, false) {
		cur := leadingWS.FindString(l.text)
		if dir > 0 && l.text == "" && !s.empty() {
			continue
		}
		w := countColumn(cur, tabSize, -1)
		nw := w + unitW
		if dir < 0 {
			if w == 0 {
				continue
			}
			nw = max(0, (w-1)/unitW*unitW)
		}
		next := buildIndent(nw, strings.Contains(unit, "\t"), tabSize)
		if next != cur {
			out = append(out, Change{l.from, l.from + len([]rune(cur)), next})
		}
	}
	return out
}

// ---- align on --------------------------------------------------------------------------------

// alignPattern is the matcher of Align On: a regular expression, or the
// literal text when it is not a valid one.
func alignPattern(p string) *regexp.Regexp {
	if p == "" {
		return nil
	}
	if re, err := regexp.Compile(p); err == nil {
		return re
	}
	return regexp.MustCompile(regexp.QuoteMeta(p))
}

// alignOn pads the selected lines (all without a selection) so the first
// match of pattern (or of its first group) starts in the same column.
func alignOn(text string, s Sel, pattern string, tabSize int) []Change {
	re := alignPattern(pattern)
	if re == nil {
		return nil
	}
	type hit struct {
		l           line
		off, column int
	}
	var hits []hit
	for _, l := range newDoc(text).selectedLines(s, true) {
		m := re.FindStringSubmatchIndex(l.text)
		if m == nil {
			continue
		}
		b := m[0]
		if len(m) >= 4 && m[2] >= 0 {
			b = m[2]
		}
		off := len([]rune(l.text[:b]))
		hits = append(hits, hit{l, off, countColumn(l.text, tabSize, off)})
	}
	if len(hits) < 2 {
		return nil
	}
	target := 0
	for _, h := range hits {
		target = max(target, h.column)
	}
	var out []Change
	for _, h := range hits {
		if h.column < target {
			p := h.l.from + h.off
			out = append(out, Change{p, p, strings.Repeat(" ", target-h.column)})
		}
	}
	return out
}

// ---- word wrap ---------------------------------------------------------------------------------

// wrapLine breaks a line at whitespace so no part is longer than width
// columns (long words stay whole).
func wrapLine(text string, width, tabSize int) []string {
	indent := leadingWS.FindString(text)
	var out []string
	rest := text
	for countColumn(rest, tabSize, -1) > width {
		r := []rune(rest)
		limit := findColumn(rest, width, tabSize)
		head := r[:min(len(r), limit+1)]
		cut := -1
		for i := len(head) - 1; i > len([]rune(indent)); i-- {
			if head[i] == ' ' || head[i] == '\t' {
				cut = i
				break
			}
		}
		if cut < 0 {
			k := strings.IndexAny(string(r[limit:]), " \t")
			if k < 0 {
				break
			}
			cut = limit + len([]rune(string(r[limit:])[:k]))
		}
		out = append(out, strings.TrimRight(string(r[:cut]), " \t"))
		rest = indent + strings.TrimLeft(string(r[cut:]), " \t")
		if strings.TrimSpace(rest) == "" {
			break
		}
	}
	return append(out, rest)
}

// applyWordWrap hard-wraps the selected lines (or the cursor line).
func applyWordWrap(text string, s Sel, width, tabSize int) []Change {
	var out []Change
	for _, l := range newDoc(text).selectedLines(s, false) {
		if parts := wrapLine(l.text, width, tabSize); len(parts) > 1 {
			out = append(out, Change{l.from, l.to, strings.Join(parts, "\n")})
		}
	}
	return out
}

// ---- comments ----------------------------------------------------------------------------------

// commentTokens are the line comments of the languages (CodeMirror's
// commentTokens); block comments for those without.
func commentTokens(lang string) (lineTok, open, close string) {
	switch lang {
	case "c", "cpp", "arduino", "csharp", "java", "kotlin", "dart", "go", "rust", "javascript", "jsx", "typescript", "tsx",
		"scss", "less", "jsonc", "protobuf", "kdl", "mojo":
		return "//", "", ""
	case "python", "bash", "powershell", "yaml", "toml", "ini", "nginx", "apache", "sshconfig", "crontab", "conf",
		"dockerfile", "makefile", "cmake", "ignore", "rpmspec", "systemd", "desktop", "nim":
		return "#", "", ""
	case "lua", "sql":
		return "--", "", ""
	case "clojure", "reg":
		return ";", "", ""
	case "batch":
		return "REM ", "", ""
	case "html", "xml", "svelte", "markdown":
		return "", "<!--", "-->"
	case "css", "sass":
		return "", "/*", "*/"
	case "jinja2":
		return "", "{#", "#}"
	}
	return "", "", ""
}

// toggleComment comments the selected lines out with the line comment, or
// removes it when all of them have it; block comments wrap the selection.
func toggleComment(text string, s Sel, lang string) []Change {
	tok, open, close := commentTokens(lang)
	d := newDoc(text)
	if tok == "" {
		if open == "" {
			return nil
		}
		from, to := s.from(), s.to()
		if s.empty() {
			l := d.lineAt(s.Head)
			from, to = l.from, l.to
		}
		inner := d.slice(from, to)
		trimmed := strings.TrimSpace(inner)
		if strings.HasPrefix(trimmed, open) && strings.HasSuffix(trimmed, close) {
			i := strings.Index(inner, open)
			j := strings.LastIndex(inner, close)
			body := strings.TrimSuffix(strings.TrimPrefix(inner[i+len(open):j], " "), " ")
			return []Change{{from, to, inner[:i] + body + inner[j+len(close):]}}
		}
		return []Change{{from, to, open + " " + inner + " " + close}}
	}
	lines := d.selectedLines(s, false)
	all, minIndent := true, -1
	for _, l := range lines {
		if strings.TrimSpace(l.text) == "" {
			continue
		}
		ind := len([]rune(leadingWS.FindString(l.text)))
		if minIndent < 0 || ind < minIndent {
			minIndent = ind
		}
		if !strings.HasPrefix(strings.TrimLeft(l.text, " \t"), tok) {
			all = false
		}
	}
	if minIndent < 0 {
		return nil
	}
	var out []Change
	for _, l := range lines {
		if strings.TrimSpace(l.text) == "" {
			continue
		}
		if all {
			ind := len([]rune(leadingWS.FindString(l.text)))
			n := len([]rune(tok))
			rest := []rune(l.text)[ind+n:]
			if len(rest) > 0 && rest[0] == ' ' {
				n++
			}
			out = append(out, Change{l.from + ind, l.from + ind + n, ""})
		} else {
			p := l.from + minIndent
			out = append(out, Change{p, p, tok + " "})
		}
	}
	return out
}

// ---- brackets and occurrences ----------------------------------------------------------------

var brackets = map[rune]rune{'(': ')', '[': ']', '{': '}', ')': '(', ']': '[', '}': '{'}

// matchingBracket finds the bracket matching the one after (or before) the
// cursor: the position of the match and of the bracket itself.
func matchingBracket(text string, pos int) (match, at int, ok bool) {
	r := []rune(text)
	try := func(p int) (int, bool) {
		if p < 0 || p >= len(r) {
			return 0, false
		}
		c := r[p]
		other, isBr := brackets[c]
		if !isBr {
			return 0, false
		}
		dir := 1
		if c == ')' || c == ']' || c == '}' {
			dir = -1
		}
		depth := 0
		for i := p; i >= 0 && i < len(r); i += dir {
			switch r[i] {
			case c:
				depth++
			case other:
				depth--
				if depth == 0 {
					return i, true
				}
			}
		}
		return 0, false
	}
	if m, ok := try(pos); ok {
		return m, pos, true
	}
	if m, ok := try(pos - 1); ok {
		return m, pos - 1, true
	}
	return 0, 0, false
}

// nextOccurrence selects the next occurrence of the selection (or the word
// at the cursor), wrapping around.
func nextOccurrence(text string, s Sel) (Sel, bool) {
	d := newDoc(text)
	from, to := s.from(), s.to()
	if s.empty() {
		f, t, ok := d.wordAt(s.Head)
		if !ok {
			return s, false
		}
		return Sel{f, t}, true
	}
	needle := d.r[from:to]
	find := func(start, end int) int {
		for i := start; i+len(needle) <= end; i++ {
			if string(d.r[i:i+len(needle)]) == string(needle) {
				return i
			}
		}
		return -1
	}
	i := find(to, len(d.r))
	if i < 0 {
		i = find(0, from)
	}
	if i < 0 {
		return s, false
	}
	return Sel{i, i + len(needle)}, true
}

// detectIndent finds whether a text indents with tabs or spaces, and the
// width of a space indent (KWrite's "Auto Detect Indent"). found is false
// when no line is indented.
func detectIndent(text string) (found, spaces bool, width int) {
	tabs, sp := 0, 0
	widths := map[int]int{}
	prev := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		switch {
		case strings.HasPrefix(l, "\t"):
			tabs++
			prev = 0
		case strings.HasPrefix(l, " "):
			sp++
			n := len(l) - len(strings.TrimLeft(l, " "))
			// The step between neighbouring indent levels.
			if d := n - prev; d > 0 && d <= 8 {
				widths[d]++
			}
			prev = n
		default:
			prev = 0
		}
	}
	if tabs == 0 && sp == 0 {
		return false, false, 0
	}
	if tabs >= sp {
		return true, false, 0
	}
	best := 0
	for w, c := range widths {
		if c > widths[best] || (c == widths[best] && w < best) {
			best = w
		}
	}
	return true, true, best
}

// saveCleanup is what KWrite changes when saving: trailing spaces removed
// ("modified": on the given lines, "all": everywhere) and a line break added
// at the end of a non-empty text.
func saveCleanup(text string, mode string, modified map[int]bool, newlineAtEOF bool) []Change {
	var out []Change
	if mode == "modified" || mode == "all" {
		for _, l := range newDoc(text).lines {
			if mode == "modified" && !modified[l.n] {
				continue
			}
			trimmed := strings.TrimRight(l.text, " \t")
			if n := len([]rune(trimmed)); n < l.to-l.from {
				out = append(out, Change{l.from + n, l.to, ""})
			}
		}
	}
	if newlineAtEOF && text != "" && !strings.HasSuffix(text, "\n") {
		n := len([]rune(text))
		out = append(out, Change{n, n, "\n"})
	}
	return out
}
