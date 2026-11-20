package form

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// GridRow is a label and its field in a Grid.
type GridRow struct{ Label, Field layout.Widget }

// Grid is a two column CSS grid (.form-grid: grid-template-columns
// max-content <field>): the labels in a column as wide as the widest, the
// fields beside them, rows centered. FieldMax limits the field column
// (0 = the rest of the row; the field gets at least FieldMin).
func Grid(colGap, rowGap unit.Dp, fieldMin, fieldMax unit.Dp, rows ...GridRow) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lw := 0
		for _, r := range rows {
			if r.Label == nil {
				continue
			}
			g := gtx
			g.Constraints.Min = image.Point{}
			m := op.Record(gtx.Ops)
			d := r.Label(g)
			m.Stop()
			lw = max(lw, d.Size.X)
		}
		gap := gtx.Dp(colGap)
		fw := gtx.Constraints.Max.X - lw - gap
		if fieldMax > 0 {
			fw = min(fw, gtx.Dp(fieldMax))
		}
		fw = max(fw, gtx.Dp(fieldMin))
		y := 0
		for i, r := range rows {
			if i > 0 {
				y += gtx.Dp(rowGap)
			}
			// Measure both, then center them on the row.
			lg := gtx
			lg.Constraints.Min = image.Point{}
			lm := op.Record(gtx.Ops)
			var ld layout.Dimensions
			if r.Label != nil {
				ld = r.Label(lg)
			}
			lc := lm.Stop()
			fg := gtx
			fg.Constraints = layout.Exact(image.Pt(fw, gtx.Constraints.Max.Y))
			fg.Constraints.Min.Y = 0
			fg.Constraints.Min.X = fw
			fm := op.Record(gtx.Ops)
			fd := r.Field(fg)
			fc := fm.Stop()
			h := max(ld.Size.Y, fd.Size.Y)
			o := op.Offset(image.Pt(0, y+(h-ld.Size.Y)/2)).Push(gtx.Ops)
			lc.Add(gtx.Ops)
			o.Pop()
			o = op.Offset(image.Pt(lw+gap, y+(h-fd.Size.Y)/2)).Push(gtx.Ops)
			fc.Add(gtx.Ops)
			o.Pop()
			y += h
		}
		return layout.Dimensions{Size: image.Pt(lw+gap+fw, y)}
	}
}

// Columns2 lays widgets out in two equal columns (.grid-2col: gap 6px 16px).
func Columns2(children ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gap := gtx.Dp(16)
		cw := (gtx.Constraints.Max.X - gap) / 2
		y := 0
		for i := 0; i < len(children); i += 2 {
			if i > 0 {
				y += gtx.Dp(6)
			}
			h := 0
			for j := 0; j < 2 && i+j < len(children); j++ {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(cw, gtx.Constraints.Max.Y)}
				o := op.Offset(image.Pt(j*(cw+gap), y)).Push(gtx.Ops)
				d := children[i+j](g)
				o.Pop()
				h = max(h, d.Size.Y)
			}
			y += h
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y)}
	}
}

// Wrap lays children side by side like Row and starts a new line when the
// next one does not fit (.wrap-row: flex-wrap with gap in both directions).
func Wrap(gap unit.Dp, children ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		g := gtx.Dp(gap)
		maxW := gtx.Constraints.Max.X
		type item struct {
			call op.CallOp
			size image.Point
		}
		var lines [][]item
		var line []item
		x := 0
		for _, c := range children {
			if c == nil {
				continue
			}
			cg := gtx
			cg.Constraints.Min = image.Point{}
			m := op.Record(gtx.Ops)
			d := c(cg)
			it := item{m.Stop(), d.Size}
			if len(line) > 0 && x+g+d.Size.X > maxW {
				lines = append(lines, line)
				line, x = nil, 0
			}
			if len(line) > 0 {
				x += g
			}
			line = append(line, it)
			x += d.Size.X
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
		y, w := 0, 0
		for i, l := range lines {
			if i > 0 {
				y += g
			}
			h := 0
			for _, it := range l {
				h = max(h, it.size.Y)
			}
			x := 0
			for j, it := range l {
				if j > 0 {
					x += g
				}
				o := op.Offset(image.Pt(x, y+(h-it.size.Y)/2)).Push(gtx.Ops)
				it.call.Add(gtx.Ops)
				o.Pop()
				x += it.size.X
			}
			w = max(w, x)
			y += h
		}
		return layout.Dimensions{Size: image.Pt(w, y)}
	}
}
