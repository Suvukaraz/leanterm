package leanedit

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/oligo/gvcode/gutter"

	"github.com/Suvukaraz/leanterm/internal/gui/folding"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- code folding (KTextEditor's folding, with gotreesitter) --------------------------------------
//
// The folding package parses the document in the background and finds the
// foldable ranges; folding.Manager keeps which are folded and moves them
// with edits. Here they meet the editor: the hidden lines go to gvcode
// (SetHiddenLines), the gutter shows the fold markers, a folded header ends
// in a "⋯" box, and the caret never rests on a hidden line (vertical moves
// skip them, horizontal moves jump over the block, jumps unfold).

// maxFoldBytes: larger documents are not parsed (as with highlighting,
// the parse would cost more than it helps).
const maxFoldBytes = 8 << 20

type foldState struct {
	m       folding.Manager
	parser  *folding.Parser
	gen     uint64 // the text's generation (parse results of older ones are stale)
	text    string // the text the manager is in sync with
	caret   int    // the caret of the last frame
	applied int    // the manager version given to gvcode
	gutter  *foldGutter
	boxes   []foldBox
}

// foldBox is the "⋯" after a folded header (clicking it unfolds).
type foldBox struct {
	line  int
	click widget.Clickable
}

// closeFolds stops the tab's parser (the tab closes).
func (t *Tab) closeFolds() {
	if t.fold.parser != nil {
		t.fold.parser.Close()
		t.fold.parser = nil
	}
}

// foldLanguage is the language the tab folds ("" for none).
func (t *Tab) foldLanguage() string {
	if !t.ready || !folding.Supported(t.LanguageID) || len(t.fold.text) > maxFoldBytes {
		return ""
	}
	return t.LanguageID
}

// syncFolds runs before the editor is laid out: it follows the text and
// the language, takes new parse results, keeps the caret off hidden lines
// and gives the hidden lines to gvcode.
func (t *Tab) syncFolds(invalidate func()) {
	f := &t.fold
	text := t.ed.Text()
	if text != f.text {
		f.m.TextChanged(f.text, text)
		f.text = text
		f.gen++
		if f.parser != nil {
			f.parser.Update(f.gen, text)
		}
	}
	lang := t.foldLanguage()
	if f.parser == nil || f.parser.Language() != lang {
		t.closeFolds()
		f.m.UnfoldAll()
		f.m.SetRanges(nil)
		if lang != "" {
			f.parser = folding.NewParser(lang, invalidate)
			f.parser.Update(f.gen, text)
		}
	}
	if f.parser != nil {
		if r, ok := f.parser.Result(); ok && r.Gen == f.gen {
			f.m.SetRanges(r.Ranges)
		}
	}
	t.keepCaretVisible()
	t.applyFolds()
}

// applyFolds gives the hidden lines to gvcode.
func (t *Tab) applyFolds() {
	f := &t.fold
	if f.applied == f.m.Version() {
		return
	}
	f.applied = f.m.Version()
	spans := f.m.HiddenSpans()
	out := make([][2]int, len(spans))
	for i, s := range spans {
		out[i] = [2]int{s.StartLine, s.EndLine}
	}
	t.ed.SetHiddenLines(out)
}

// keepCaretVisible: the caret on a hidden line moves over the fold when
// it came from the fold's edges (Right at the end of the header, Left at
// the start of the line below); any other way there (search, go to line,
// undo, a click on a bookmark) unfolds the folds around it.
func (t *Tab) keepCaretVisible() {
	f := &t.fold
	caret, anchor := t.ed.Selection()
	defer func() { f.caret, _ = t.ed.Selection() }()
	if !f.m.HasFolds() {
		return
	}
	d := t.lineIndex()
	ln := d.lineAt(caret).n - 1
	if !f.m.IsLineHidden(ln) {
		return
	}
	fold, _ := f.m.HidingFold(ln)
	prev := d.lineAt(min(f.caret, len(d.r))).n - 1
	to := -1
	switch {
	case caret > f.caret && prev == fold.StartLine:
		if fold.EndLine+1 < len(d.lines) {
			to = d.line(fold.EndLine + 2).from
		} else {
			to = d.line(fold.StartLine + 1).to // nothing below the fold
		}
	case caret < f.caret && prev == fold.EndLine+1:
		to = d.line(fold.StartLine + 1).to
	}
	if to < 0 {
		f.m.Reveal(ln)
		t.ed.SetCaret(caret, anchor) // scrolls to the caret once laid out
		return
	}
	if anchor == caret {
		anchor = to
	}
	t.ed.SetCaret(to, anchor)
}

// caretToHeader moves a caret that a fold just hid to the end of the
// fold's header (KTextEditor does the same).
func (t *Tab) caretToHeader() {
	f := &t.fold
	caret, anchor := t.ed.Selection()
	d := t.lineIndex()
	ln := d.lineAt(caret).n - 1
	if !f.m.IsLineHidden(ln) {
		return
	}
	fold, _ := f.m.HidingFold(ln)
	to := d.line(fold.StartLine + 1).to
	if anchor == caret || f.m.IsLineHidden(d.lineAt(anchor).n-1) {
		anchor = to
	}
	t.ed.SetCaret(to, anchor)
	f.caret = to
}

// The fold actions (View menu).

func (t *Tab) toggleFoldAtCaret() {
	caret, _ := t.ed.Selection()
	t.fold.m.ToggleAt(t.lineIndex().lineAt(caret).n - 1)
	t.caretToHeader()
	t.applyFolds()
}

func (t *Tab) foldAll() {
	t.fold.m.FoldAll()
	t.caretToHeader()
	t.applyFolds()
}

func (t *Tab) unfoldAll() {
	t.fold.m.UnfoldAll()
	t.applyFolds()
}

// toggleFoldLine is a click on the fold marker of a line.
func (t *Tab) toggleFoldLine(line int) {
	if t.fold.m.Toggle(line) {
		t.caretToHeader()
		t.applyFolds()
	}
}

// canFold reports whether the tab has something to fold.
func (t *Tab) canFold() bool { return t != nil && len(t.fold.m.Ranges()) > 0 }

// visualLines is the number of lines on screen (without the hidden ones).
func (t *Tab) visualLines() int { return t.fold.m.TotalVisualLines(max(1, t.ed.Lines())) }

// ---- the fold markers in the gutter ------------------------------------------------------------

// foldGutter is the column right of the line numbers with KTextEditor's
// fold markers: a down arrow where a block starts, a right arrow where one
// is folded.
type foldGutter struct {
	t          *Tab
	pal        *gtk.Palette
	invalidate func()
}

func (g *foldGutter) ID() string    { return "leanedit-folding" }
func (g *foldGutter) Priority() int { return 50 } // between the line numbers and the text

func (g *foldGutter) Width(gtx layout.Context, sh *text.Shaper, p text.Parameters, lines int) unit.Dp {
	if g.t.foldLanguage() == "" {
		return 0
	}
	return 14
}

func (g *foldGutter) Layout(gtx layout.Context, ctx gutter.GutterContext) layout.Dimensions {
	w := gtx.Dp(g.Width(gtx, nil, text.Parameters{}, 0))
	if w == 0 {
		return layout.Dimensions{}
	}
	m := &g.t.fold.m
	col := g.pal.MutedFg
	for _, p := range ctx.Paragraphs {
		if _, ok := m.RangeAt(p.Index); !ok {
			continue
		}
		top, _ := lineBox(ctx, p)
		bot := top + ctx.LineHeight.Ceil() // the first screen line of a wrapped line
		cy := float32(top+bot) / 2
		cx := float32(w) / 2
		s := float32(gtx.Dp(3.5))
		var path clip.Path
		path.Begin(gtx.Ops)
		if m.IsFolded(p.Index) {
			path.MoveTo(f32.Pt(cx-s*0.6, cy-s))
			path.LineTo(f32.Pt(cx+s*0.8, cy))
			path.LineTo(f32.Pt(cx-s*0.6, cy+s))
		} else {
			path.MoveTo(f32.Pt(cx-s, cy-s*0.6))
			path.LineTo(f32.Pt(cx+s, cy-s*0.6))
			path.LineTo(f32.Pt(cx, cy+s*0.8))
		}
		path.Close()
		c := col
		if m.IsFolded(p.Index) {
			c = g.pal.FgColor
		}
		paint.FillShape(gtx.Ops, c, clip.Outline{Path: path.End()}.Op())
	}
	return layout.Dimensions{Size: image.Pt(w, gtx.Constraints.Max.Y)}
}

// HandleClick toggles the fold of the clicked line.
func (g *foldGutter) HandleClick(line int, _ pointer.Source, _ int, _ key.Modifiers) bool {
	if _, ok := g.t.fold.m.RangeAt(line); !ok {
		return false
	}
	g.t.toggleFoldLine(line)
	if g.invalidate != nil {
		g.invalidate()
	}
	return true
}

func (g *foldGutter) HandleHover(int) *gutter.HoverInfo { return nil }

var _ gutter.InteractiveGutter = (*foldGutter)(nil)

func (t *Tab) foldProvider(p *gtk.Palette, invalidate func()) *foldGutter {
	if t.fold.gutter == nil {
		t.fold.gutter = &foldGutter{t: t}
	}
	t.fold.gutter.pal = p
	t.fold.gutter.invalidate = invalidate
	return t.fold.gutter
}

// ---- the "⋯" boxes after folded headers ----------------------------------------------------------

// layoutFoldBoxes draws the boxes over the editor (after it is laid out).
// A click unfolds.
func (t *Tab) layoutFoldBoxes(gtx layout.Context, th *gtk.Theme) {
	f := &t.fold
	folded := f.m.FoldedRanges()
	if len(folded) == 0 {
		f.boxes = f.boxes[:0]
		return
	}
	d := t.lineIndex()
	vp := t.ed.TextView().Viewport()
	gw := t.ed.GutterWidth()
	lh := t.ed.TextView().GetLineHeight().Ceil()
	n := 0
	for _, r := range folded {
		if r.StartLine+1 > len(d.lines) || f.m.IsLineHidden(r.StartLine) {
			continue
		}
		pos := t.ed.TextView().RuneCoords(d.line(r.StartLine + 1).to)
		if pos.Y+float32(lh) < 0 || pos.Y-float32(lh) > float32(vp.Dy()) {
			continue
		}
		if n == len(f.boxes) {
			f.boxes = append(f.boxes, foldBox{})
		}
		b := &f.boxes[n]
		n++
		b.line = r.StartLine
		if b.click.Clicked(gtx) {
			f.m.Unfold(b.line)
			t.applyFolds()
			gtx.Execute(op.InvalidateCmd{})
		}
		x := gw + int(pos.X) + gtx.Dp(6)
		asc := int(float32(lh) * 0.75)
		top := int(pos.Y) - asc
		off := op.Offset(image.Pt(x, top)).Push(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(40), lh)}
		b.click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
			return foldBoxLayout(gtx, th, lh, b.click.Hovered())
		})
		off.Pop()
	}
	f.boxes = f.boxes[:n]
}

// foldBoxLayout is KTextEditor's folded-block marker: a small rounded box
// with an ellipsis.
func foldBoxLayout(gtx layout.Context, th *gtk.Theme, lh int, hovered bool) layout.Dimensions {
	pal := th.Pal
	w, h := gtx.Dp(22), max(gtx.Dp(10), lh*3/4)
	rr := gtx.Dp(3)
	bg := pal.HeaderBg
	if hovered {
		bg = pal.EditorSelection
	}
	r := image.Rect(0, 0, w, h)
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(r, rr).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, pal.BorderColor, clip.Stroke{Path: clip.UniformRRect(r, rr).Path(gtx.Ops), Width: float32(max(1, gtx.Dp(1)))}.Op())
	dot := max(2, gtx.Dp(2))
	cy := h / 2
	for i := -1; i <= 1; i++ {
		cx := w/2 + i*gtx.Dp(5)
		box := image.Rect(cx-dot/2, cy-dot/2, cx-dot/2+dot, cy-dot/2+dot)
		paint.FillShape(gtx.Ops, pal.MutedFg, clip.Ellipse(box).Op(gtx.Ops))
	}
	pointer.CursorPointer.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
}
