package leanedit

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/oligo/gvcode/textstyle/syntax"

	"github.com/Suvukaraz/leanterm/internal/gui/codeedit"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// minimap.ts: the right edge shows the document as small blocks (one per
// character, in the syntax colors) with the visible part marked. It
// replaces the vertical scrollbar: click to jump, drag the marked area to
// scroll, the wheel scrolls the editor.
const (
	miniWidth unit.Dp = 80
	miniPad   unit.Dp = 4
	miniLine  unit.Dp = 3
	miniChar  unit.Dp = 1
)

type minimap struct {
	tag     int
	drag    bool
	dragY   float32
	dragTop float32
	hover   gtk.Hover
}

// miniGeo is the minimap's geometry in lines (fractions) and pixels.
type miniGeo struct {
	lines          int
	top, visible   float32
	offset, height int
	line           int // px per line
}

func (t *Tab) miniGeo(gtx layout.Context, height int) miniGeo {
	lines := max(1, t.ed.Lines())
	_, _, minY, maxY := t.ed.ScrollRatio()
	if maxY <= 0 || maxY != maxY { // NaN before the first layout
		minY, maxY = 0, 1
	}
	g := miniGeo{lines: lines, height: height, line: max(1, gtx.Dp(miniLine))}
	g.top = minY * float32(lines)
	g.visible = max(1, (min(1, maxY)-minY)*float32(lines))
	total := lines * g.line
	if total > height {
		g.offset = int(g.top / max(1, float32(lines)-g.visible) * float32(total-height))
		g.offset = min(total-height, max(0, g.offset))
	}
	return g
}

// scrollToLine makes line (0-based, fractional) the first visible one.
func (t *Tab) scrollToLine(gtx layout.Context, g miniGeo, line float32) {
	line = max(0, min(line, float32(g.lines)-g.visible))
	t.ed.Scroll(gtx, 0, (line-g.top)/float32(g.lines))
}

func scopeColor(p *gtk.Palette, s syntax.StyleScope) color.NRGBA {
	switch s {
	case codeedit.ScopeKeyword, codeedit.ScopeTag:
		return p.SyntaxKeyword
	case codeedit.ScopeString:
		return p.SyntaxString
	case codeedit.ScopeComment:
		return p.SyntaxComment
	case codeedit.ScopeNumber:
		return p.SyntaxNumber
	case codeedit.ScopeType:
		return p.SyntaxType
	case codeedit.ScopeFunction:
		return p.SyntaxFunction
	case codeedit.ScopeConstant:
		return p.SyntaxConstant
	case codeedit.ScopeOperator:
		return p.SyntaxOperator
	case codeedit.ScopeAttribute:
		return p.SyntaxAttribute
	case codeedit.ScopeProperty:
		return p.SyntaxProperty
	case codeedit.ScopeMeta:
		return p.SyntaxMeta
	case codeedit.ScopeInvalid:
		return p.SyntaxInvalid
	}
	return p.TextColor
}

func fillRect(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

// minimapLayout draws the minimap of t over the full height.
func (a *App) minimapLayout(gtx layout.Context, th *gtk.Theme, t *Tab) layout.Dimensions {
	pal := th.Pal
	m := &t.mini
	w, h := gtx.Dp(miniWidth), gtx.Constraints.Max.Y
	g := t.miniGeo(gtx, h)
	sliderTop := int(g.top*float32(g.line)) - g.offset
	sliderH := max(2*g.line, int(g.visible*float32(g.line)))

	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &m.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			if !e.Buttons.Contain(pointer.ButtonPrimary) {
				continue
			}
			y := e.Position.Y
			top := g.top
			if int(y) < sliderTop || int(y) > sliderTop+sliderH {
				top = (y+float32(g.offset))/float32(g.line) - g.visible/2
				t.scrollToLine(gtx, g, top)
				top = max(0, min(top, float32(g.lines)-g.visible))
			}
			m.drag, m.dragY, m.dragTop = true, y, top
		case pointer.Drag:
			if m.drag {
				free := float32(h - sliderH)
				per := 1 / float32(g.line)
				if g.lines*g.line > h && free > 0 {
					per = (float32(g.lines) - g.visible) / free
				}
				t.scrollToLine(gtx, g, m.dragTop+(e.Position.Y-m.dragY)*per)
			}
		case pointer.Release, pointer.Cancel:
			m.drag = false
		case pointer.Scroll:
			// In lines: the editor's own wheel step is a third of a page.
			lh := float32(h) / max(1, g.visible)
			t.scrollToLine(gtx, g, g.top+e.Scroll.Y/max(1, lh*float32(g.line)/float32(g.line)))
		}
	}
	hovered := m.hover.Update(gtx)

	defer clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops).Pop()
	fillRect(gtx, image.Rect(0, 0, w, h), pal.BaseColor)

	// Selected lines.
	text := t.ed.Text()
	d := t.lineIndex()
	first := g.offset / g.line
	last := min(len(d.lines), (g.offset+h+g.line-1)/g.line)
	y := func(i int) int { return i*g.line - g.offset }
	if s := t.sel(); !s.empty() {
		a1, b1 := d.lineAt(s.from()).n-1, d.lineAt(s.to()).n
		if b1 > first && a1 < last {
			fillRect(gtx, image.Rect(0, y(a1), w, y(b1)), pal.EditorSelection)
		}
	}
	// The characters as runs of colored blocks.
	pad, cw := gtx.Dp(miniPad), max(1, gtx.Dp(miniChar))
	maxCols := (w - 2*pad) / cw
	tabSize := a.tabSize()
	toks := t.tokens
	_ = text
	for i := first; i < last; i++ {
		l := d.lines[i]
		ti := sort.Search(len(toks), func(k int) bool { return toks[k].End > l.from })
		col, runStart := 0, -1
		var runColor color.NRGBA
		yy := y(i)
		flush := func(end int) {
			if runStart < 0 {
				return
			}
			fillRect(gtx, image.Rect(pad+runStart*cw, yy, pad+min(end, maxCols)*cw, yy+g.line-1), runColor)
			runStart = -1
		}
		for j, r := range []rune(l.text) {
			if col >= maxCols || j >= maxCols*4 {
				break
			}
			if r == ' ' || r == '\t' {
				flush(col)
				if r == '\t' {
					col = (col/tabSize + 1) * tabSize
				} else {
					col++
				}
				continue
			}
			pos := l.from + j
			for ti < len(toks) && toks[ti].End <= pos {
				ti++
			}
			c := pal.TextColor
			if ti < len(toks) && toks[ti].Start <= pos {
				c = scopeColor(pal, toks[ti].Scope)
			}
			if c != runColor {
				flush(col)
				runColor = c
			}
			if runStart < 0 {
				runStart = col
			}
			col++
		}
		flush(col)
	}
	// The slider: opacity 0.75, 1 while hovered or dragged.
	sc, bc := pal.EditorMinimapSlider, pal.Scrollbar
	if !hovered && !m.drag {
		sc.A, bc.A = uint8(float32(sc.A)*0.75), uint8(float32(bc.A)*0.75)
	}
	fillRect(gtx, image.Rect(0, sliderTop, w, sliderTop+sliderH), sc)
	fillRect(gtx, image.Rect(0, sliderTop, w, sliderTop+1), bc)
	fillRect(gtx, image.Rect(0, sliderTop+sliderH-1, w, sliderTop+sliderH), bc)
	// border-left.
	fillRect(gtx, image.Rect(0, 0, max(1, gtx.Dp(1)), h), pal.BorderColor)

	event.Op(gtx.Ops, &m.tag)
	m.hover.Add(gtx)
	pointer.CursorDefault.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// scrollbarLayout is a thin vertical scrollbar when the minimap is off
// (the web view's scrollbar): drag the thumb or click the track.
func (a *App) scrollbarLayout(gtx layout.Context, th *gtk.Theme, t *Tab) layout.Dimensions {
	pal := th.Pal
	m := &t.mini
	w, h := gtx.Dp(10), gtx.Constraints.Max.Y
	_, _, minY, maxY := t.ed.ScrollRatio()
	if maxY <= 0 || maxY != maxY {
		minY, maxY = 0, 1
	}
	maxY = min(1, maxY)
	thumbTop, thumbH := int(minY*float32(h)), max(gtx.Dp(20), int((maxY-minY)*float32(h)))
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &m.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			yy := e.Position.Y
			if int(yy) < thumbTop || int(yy) > thumbTop+thumbH {
				t.ed.Scroll(gtx, 0, (yy-float32(thumbTop)-float32(thumbH)/2)/float32(h))
			}
			m.drag, m.dragY = true, yy
		case pointer.Drag:
			if m.drag {
				t.ed.Scroll(gtx, 0, (e.Position.Y-m.dragY)/float32(h))
				m.dragY = e.Position.Y
			}
		case pointer.Release, pointer.Cancel:
			m.drag = false
		}
	}
	if maxY-minY >= 0.999 {
		return layout.Dimensions{}
	}
	defer clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops).Pop()
	fillRect(gtx, image.Rect(0, 0, w, h), pal.BaseColor)
	r := image.Rect(gtx.Dp(2), thumbTop, w-gtx.Dp(2), thumbTop+thumbH)
	paint.FillShape(gtx.Ops, pal.Scrollbar, clip.UniformRRect(r, gtx.Dp(3)).Op(gtx.Ops))
	event.Op(gtx.Ops, &m.tag)
	pointer.CursorDefault.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
}
