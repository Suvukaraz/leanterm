package filebrowser

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

type panelUI struct {
	r   input.Router
	ops op.Ops
	th  *gtk.Theme
	p   *Panel
	now time.Time
	ev  time.Duration // pointer event time
}

func (u *panelUI) frame() {
	u.ops.Reset()
	u.now = u.now.Add(20 * time.Millisecond)
	gtx := layout.Context{Ops: &u.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 500)), Source: u.r.Source(), Now: u.now}
	layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return u.p.Layout(gtx, u.th) }),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { u.p.LayoutOverlays(gtx, u.th); return layout.Dimensions{} }),
	)
	u.r.Frame(&u.ops)
}

// click clicks n times (n = 2: a double click); separate calls are far
// enough apart in time not to count as one.
func (u *panelUI) click(x, y float32, n int) {
	u.ev += time.Second
	for i := 0; i < n; i++ {
		u.ev += 50 * time.Millisecond
		u.r.Queue(
			pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(x, y), Time: u.ev},
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: f32.Pt(x, y), Buttons: pointer.ButtonPrimary, Time: u.ev},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x, y), Time: u.ev},
		)
		u.frame()
	}
}

// rowY finds the y of the row showing name by scanning the frame for the
// list's rows: the header and toolbars come first, rows are 22px.
func (u *panelUI) settle(b *Browser, ok func(State) bool, t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !ok(b.Snapshot()) {
		if time.Now().After(deadline) {
			t.Fatalf("state %+v", b.Snapshot())
		}
		time.Sleep(2 * time.Millisecond)
		u.frame()
	}
	u.frame()
	u.frame()
}

func TestPanelOpenFolderRenameDelete(t *testing.T) {
	b, fb, _ := newBrowser(KindLocal)
	u := &panelUI{th: gtk.New("dark"), now: time.Now()}
	u.p = &Panel{B: b}
	u.frame() // the panel sets the browser's error handler
	b.Open(false, false)
	u.settle(b, func(s State) bool { return len(s.Entries) == 2 && !s.Loading }, t)

	// Find the list's first row: below the header, toolbar, path bar and
	// list header. Rows: "..", "src/" (folders first), "b.txt".
	y0 := float32(-1)
	for y := float32(60); y < 300; y++ {
		// The ".." row navigates up on a double click; locate it by
		// probing for the row where a single click selects "src/".
		u.click(10, y, 1)
		if s := b.Snapshot(); len(s.Selected) == 1 && strings.HasSuffix(s.Selected[0], "src/") {
			y0 = y
			break
		}
	}
	if y0 < 0 {
		t.Fatal("no row selected the folder")
	}
	// F2 renames the selected entry.
	u.r.Queue(key.Event{Name: key.NameF2, State: key.Press})
	u.frame()
	if u.p.dlg != "rename" {
		t.Fatalf("dialog %q after F2", u.p.dlg)
	}
	u.r.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 100}, Text: "lib"})
	u.frame()
	u.r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	u.frame()
	u.settle(b, func(State) bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return strings.Contains(strings.Join(fb.calls, "|"), "mv /home/me/project/src/ /home/me/project/lib")
	}, t)
	if u.p.dlg != "" {
		t.Fatalf("dialog %q still open", u.p.dlg)
	}
	// Delete asks first; "Yes" removes.
	b.SetSelected([]string{"/home/me/project/b.txt"})
	u.frame()
	u.click(10, y0+22, 1) // b.txt is the row after the folder
	u.r.Queue(key.Event{Name: key.NameDeleteForward, State: key.Press})
	u.frame()
	if u.p.dlg != "delete" {
		t.Fatalf("dialog %q after Delete", u.p.dlg)
	}
	// A double click on the folder opens it.
	u.p.dlg = ""
	u.frame() // the dialog's backdrop goes away
	u.click(10, y0, 2)
	u.settle(b, func(State) bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return strings.Contains(strings.Join(fb.calls, "|"), "list /home/me/project/src")
	}, t)
}
