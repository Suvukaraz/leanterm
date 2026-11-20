package form

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

func TestPasswordEntryIsMasked(t *testing.T) {
	f := &Form{Th: gtk.New("dark")}
	pw, plain := "secret", "visible"
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(400, 100)}}
	f.Entry(&pw, EntryOpts{Password: true})(gtx)
	f.Entry(&plain, EntryOpts{})(gtx)
	if m := f.state[&pw].(*entryState).ed.Mask; m != '•' {
		t.Errorf("password entry mask %q", m)
	}
	if m := f.state[&plain].(*entryState).ed.Mask; m != 0 {
		t.Errorf("plain entry mask %q", m)
	}
}
