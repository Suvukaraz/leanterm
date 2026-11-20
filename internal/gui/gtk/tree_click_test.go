package gtk

import (
	"image"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// A right click whose press and release arrive in one frame still opens
// the context menu.
func TestTreeFastRightClick(t *testing.T) {
	u := newUI(t)
	tr := Tree{Root: &TreeNode{ID: "root", Name: "My Connections", Group: true, Children: []*TreeNode{
		{ID: "c2", Name: "web", Method: "ssh"},
	}}, Expanded: map[string]bool{"root": true}}
	u.size = image.Pt(300, 200)
	w := func(gtx layout.Context) layout.Dimensions { return tr.Layout(gtx, u.th) }
	u.frame(w)
	u.frame(w)
	u.pointer(pointer.Move, 100, 1+22+11, 0)
	u.pointer(pointer.Press, 100, 1+22+11, pointer.ButtonSecondary)
	u.pointer(pointer.Release, 100, 1+22+11, 0)
	u.frame(w)
	found := false
	for _, e := range tr.Events() {
		found = found || e.Kind == TreeContextClick
	}
	if !found {
		t.Fatal("fast right click lost")
	}
}
