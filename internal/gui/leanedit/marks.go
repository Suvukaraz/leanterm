package leanedit

import (
	"image"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/gutter"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- bookmarks and modified lines (commands.ts) --------------------------------------------------

// marks is a gutter column left of the line numbers: a bar on modified
// lines (cm-modified-line), and the bookmarked lines' background across the
// editor (cm-bookmark-line).
type marks struct {
	t   *Tab
	pal *gtk.Palette
}

func (m *marks) ID() string    { return "leanedit-marks" }
func (m *marks) Priority() int { return 200 } // left of the line numbers

func (m *marks) Width(gtx layout.Context, sh *text.Shaper, p text.Parameters, lines int) unit.Dp {
	return 3
}

func (m *marks) Layout(gtx layout.Context, ctx gutter.GutterContext) layout.Dimensions {
	modified := m.t.modifiedSet()
	w := gtx.Dp(3)
	for _, p := range ctx.Paragraphs {
		if !modified[p.Index+1] {
			continue
		}
		top, bot := lineBox(ctx, p)
		r := clip.Rect(image.Rect(0, top, w, bot)).Push(gtx.Ops)
		paint.ColorOp{Color: m.pal.EditorModified}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		r.Pop()
	}
	return layout.Dimensions{Size: image.Pt(w, gtx.Constraints.Max.Y)}
}

// lineBox is a paragraph's line box in gutter coordinates.
func lineBox(ctx gutter.GutterContext, p gutter.Paragraph) (top, bottom int) {
	lh := ctx.LineHeight.Ceil()
	glyph := (p.Ascent + p.Descent).Ceil()
	lead := (lh - glyph) / 2
	top = p.StartY - p.Ascent.Ceil() - lead - ctx.Viewport.Min.Y
	bottom = p.EndY + p.Descent.Ceil() + (lh - glyph - lead) - ctx.Viewport.Min.Y
	return top, bottom
}

// HighlightedLines: the bookmarked lines (gvcode paints them across the editor).
func (m *marks) HighlightedLines() []gutter.LineHighlight {
	var out []gutter.LineHighlight
	for _, n := range m.t.bookmarkLines() {
		out = append(out, gutter.LineHighlight{Line: n - 1, Color: gvcolor.MakeColor(m.pal.EditorBookmark)})
	}
	return out
}

// bookmarkLines are the bookmarked line numbers (1-based, sorted, unique).
func (t *Tab) bookmarkLines() []int {
	if len(t.bookmarks) == 0 {
		return nil
	}
	d := t.lineIndex()
	seen := map[int]bool{}
	var out []int
	for _, mk := range t.bookmarks {
		n := d.lineAt(mk.Offset()).n
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// toggleBookmark sets or removes the bookmark of the cursor line.
func (t *Tab) toggleBookmark() {
	d := t.lineIndex()
	caret, _ := t.ed.Selection()
	l := d.lineAt(caret)
	kept := t.bookmarks[:0]
	removed := false
	for _, mk := range t.bookmarks {
		if d.lineAt(mk.Offset()).n == l.n {
			t.ed.RemoveMarker(mk)
			removed = true
			continue
		}
		kept = append(kept, mk)
	}
	t.bookmarks = kept
	if !removed {
		if mk, err := t.ed.CreateMarker(l.from, gvcode.BiasForward); err == nil {
			t.bookmarks = append(t.bookmarks, mk)
		}
	}
}

func (t *Tab) clearBookmarks() {
	for _, mk := range t.bookmarks {
		t.ed.RemoveMarker(mk)
	}
	t.bookmarks = nil
}

// lineIndex is the document's lines (cached until the text changes).
func (t *Tab) lineIndex() *doc {
	s := t.ed.Text()
	if t.lines == nil || t.linesOf != s {
		t.lines, t.linesOf = newDoc(s), s
	}
	return t.lines
}

// modifiedSet: the lines changed since the last load or save. The web GUI
// tracks edits; here the lines between the common start and end of the
// saved and the current text count (the same for single edits).
func (t *Tab) modifiedSet() map[int]bool {
	cur := t.ed.Text()
	if t.modOf == cur && t.modSaved == t.savedText && t.modified != nil {
		return t.modified
	}
	t.modOf, t.modSaved = cur, t.savedText
	t.modified = modifiedLines(t.savedText, cur)
	return t.modified
}

func modifiedLines(saved, cur string) map[int]bool {
	out := map[int]bool{}
	if saved == cur {
		return out
	}
	a, b := strings.Split(saved, "\n"), strings.Split(cur, "\n")
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	for n := pre; n < len(b)-suf; n++ {
		out[n+1] = true
	}
	return out
}

func (t *Tab) modifiedList() []int {
	var out []int
	for n := range t.modifiedSet() {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// nextLineIn is the next (dir 1) or previous (-1) line of lines after
// current, wrapping around if wrap is set (0 = none).
func nextLineIn(lines []int, current, dir int, wrap bool) int {
	if len(lines) == 0 {
		return 0
	}
	if dir > 0 {
		for _, n := range lines {
			if n > current {
				return n
			}
		}
		if wrap {
			return lines[0]
		}
		return 0
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] < current {
			return lines[i]
		}
	}
	if wrap {
		return lines[len(lines)-1]
	}
	return 0
}

// ---- location history (navigation.ts) -------------------------------------------------------------

// location is a place in a tab, kept by a marker so it follows edits.
type location struct {
	tab int
	mk  *gvcode.Marker
	pos int // when there is no marker (tests)
}

func (l location) offset() int {
	if l.mk != nil {
		return l.mk.Offset()
	}
	return l.pos
}

const maxLocations = 100

// history is LocationHistory: jumps ("Previous/Next Location") or edits.
type history struct {
	list  []location
	index int
	same  func(a, b location) bool
	// drop releases an entry's marker when the history forgets it.
	drop func(location)
}

func (h *history) forget(ls []location) {
	if h.drop == nil {
		return
	}
	for _, l := range ls {
		h.drop(l)
	}
}

func newHistory(same func(a, b location) bool) *history { return &history{index: -1, same: same} }

func (h *history) trim() {
	if len(h.list) > maxLocations {
		h.forget(h.list[:len(h.list)-maxLocations])
		h.list = h.list[len(h.list)-maxLocations:]
	}
	h.index = len(h.list) - 1
}

func (h *history) push(l location) {
	h.forget(h.list[h.index+1:])
	h.list = h.list[:h.index+1]
	if n := len(h.list); n > 0 && h.same(h.list[n-1], l) {
		h.forget([]location{h.list[n-1]})
		h.list[n-1] = l
	} else {
		h.list = append(h.list, l)
	}
	h.trim()
}

func (h *history) jump(from, to location) {
	h.push(from)
	h.push(to)
}

func (h *history) edit(l location) {
	if n := len(h.list); n > 0 && h.same(h.list[n-1], l) {
		h.forget([]location{h.list[n-1]})
		h.list[n-1] = l
	} else {
		h.list = append(h.list, l)
	}
	h.trim()
}

// back returns where to go from current (nil: nowhere). If the cursor left
// the current entry, that entry comes first and current is kept after it.
func (h *history) back(current *location) *location {
	if h.index < 0 {
		return nil
	}
	entry := h.list[h.index]
	if current != nil && !h.same(entry, *current) {
		h.list = append(h.list[:h.index+1:h.index+1], *current)
		return &entry
	}
	if h.index == 0 {
		return nil
	}
	h.index--
	e := h.list[h.index]
	return &e
}

func (h *history) forward() *location {
	if h.index >= len(h.list)-1 {
		return nil
	}
	h.index++
	e := h.list[h.index]
	return &e
}

func (h *history) removeTab(tab int) {
	before := 0
	for i := 0; i <= h.index && i < len(h.list); i++ {
		if h.list[i].tab == tab {
			before++
		}
	}
	kept := h.list[:0]
	for _, l := range h.list {
		if l.tab != tab {
			kept = append(kept, l)
		}
		// The tab's markers go with its buffer.
	}
	h.list = kept
	low := -1
	if len(h.list) > 0 {
		low = 0
	}
	h.index = min(len(h.list)-1, max(low, h.index-before))
}
