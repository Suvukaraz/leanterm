package gtk

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// offset moves the drawing by (x, y) until Pop.
func offset(gtx layout.Context, x, y int) op.TransformStack {
	return op.Offset(image.Pt(x, y)).Push(gtx.Ops)
}

// clipRect pushes a clip of size sz (also the area for pointer events).
func clipRect(gtx layout.Context, sz image.Point) clip.Stack {
	return clip.Rect{Max: sz}.Push(gtx.Ops)
}

// eventArea registers tag for pointer events in the current clip.
func eventArea(gtx layout.Context, tag event.Tag) {
	event.Op(gtx.Ops, tag)
}

// Hover tracks whether the pointer is over an area. Call Update before the
// layout and Add inside the area's clip.
type Hover struct{ on bool }

// Update reads the pointer events of the last frame and reports the state.
func (h *Hover) Update(gtx layout.Context) bool {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			h.on = e.Kind == pointer.Enter
		}
	}
	return h.on
}

// Add registers the area for the hover state.
func (h *Hover) Add(gtx layout.Context) { event.Op(gtx.Ops, h) }

// widgetDrag follows a pointer drag on an area that moves with the drag
// (a splitter handle): positions are relative to the area, so each event
// moves it by its distance from the point where it was grabbed.
type widgetDrag struct {
	pressed bool
	grab    float32
}

// update returns how far the area must move along the axis and whether a
// drag is going on.
func (d *widgetDrag) update(gtx layout.Context, tag event.Tag, vertical bool) (int, bool) {
	move := float32(0)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		v := e.Position.X
		if vertical {
			v = e.Position.Y
		}
		switch e.Kind {
		case pointer.Press:
			d.pressed, d.grab = true, v
			gtx.Execute(pointer.GrabCmd{Tag: tag, ID: e.PointerID})
		case pointer.Drag:
			// All events of a frame are relative to where the area was drawn
			// last; the latest one counts.
			move = v - d.grab
		case pointer.Release, pointer.Cancel:
			d.pressed = false
		}
	}
	return int(move), d.pressed
}

// FillWidth makes w take all the width it is given (a Flexed child must,
// else the Flex puts the space after it).
func FillWidth(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		d := w(gtx)
		d.Size.X = max(d.Size.X, gtx.Constraints.Max.X)
		return d
	}
}
