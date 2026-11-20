package form

import (
	"gioui.org/layout"
	"gioui.org/op"
)

type callOp = op.CallOp

func recordOp(gtx layout.Context, w layout.Widget) (op.CallOp, layout.Dimensions) {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	return m.Stop(), d
}
