// Package form builds the settings dialogs (edit connection, preferences)
// from the gtk widgets bound to Go fields: a widget keeps its state by the
// field's address and writes the field when the user changes it, so a
// dialog edits a config struct directly. The layout helpers follow the web
// GUI's .row-flex (gap 8px), .col-flex (gap 6px), .section-group (gap 8px)
// and .field-label (min-width 90px, 0.9em).
package form

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// Form holds the widgets of one dialog.
type Form struct {
	Th *gtk.Theme
	// Disabled greys out every field (protected nodes, read-only config).
	Disabled bool
	// ChooseFile asks for a file or folder (file fields); it may block and
	// is called in a goroutine.
	ChooseFile func(title, dir string, folder bool) (string, error)
	// Changed is called after a file dialog wrote a field (a redraw).
	Changed func()

	state   map[any]any
	clk     map[string]*widget.Clickable
	pending chan func()
}

// Later runs fn on the UI goroutine with the next widget the form lays out
// (results of file dialogs, which run in goroutines), and asks for a frame.
func (f *Form) Later(fn func()) {
	f.pendingCh() <- fn
	if f.Changed != nil {
		f.Changed()
	}
}

func (f *Form) pendingCh() chan func() {
	if f.pending == nil {
		f.pending = make(chan func(), 16)
	}
	return f.pending
}

// apply runs what Later queued. It creates the channel too: every widget
// calls it on the UI goroutine before a click can start a goroutine that
// calls Later.
func (f *Form) apply() {
	ch := f.pendingCh()
	for {
		select {
		case fn := <-ch:
			fn()
		default:
			return
		}
	}
}

func get[T any](f *Form, key any, mk func() *T) *T {
	f.apply()
	if f.state == nil {
		f.state = map[any]any{}
	}
	if v, ok := f.state[key].(*T); ok {
		return v
	}
	v := mk()
	f.state[key] = v
	return v
}

// Reset forgets the widgets (a dialog shows another object).
func (f *Form) Reset() { f.state, f.clk = nil, nil }

// Clicked returns the button of a name, for Button.
func (f *Form) Clicked(name string) *widget.Clickable {
	if f.clk == nil {
		f.clk = map[string]*widget.Clickable{}
	}
	c := f.clk[name]
	if c == nil {
		c = new(widget.Clickable)
		f.clk[name] = c
	}
	return c
}

// ---- layout -------------------------------------------------------------------

// Row lays children side by side, centered, 8px apart (.row-flex).
func Row(children ...layout.Widget) layout.Widget { return gapped(layout.Horizontal, 8, children) }

// RowGap is Row with another gap.
func RowGap(gap unit.Dp, children ...layout.Widget) layout.Widget {
	return gapped(layout.Horizontal, gap, children)
}

// Col stacks children 6px apart (.col-flex); Section 8px (.section-group).
func Col(children ...layout.Widget) layout.Widget { return gapped(layout.Vertical, 6, children) }

func Section(children ...layout.Widget) layout.Widget { return gapped(layout.Vertical, 8, children) }

func gapped(axis layout.Axis, gap unit.Dp, children []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for _, c := range children {
			if c == nil {
				continue
			}
			if len(kids) > 0 {
				if axis == layout.Horizontal {
					kids = append(kids, layout.Rigid(layout.Spacer{Width: gap}.Layout))
				} else {
					kids = append(kids, layout.Rigid(layout.Spacer{Height: gap}.Layout))
				}
			}
			kids = append(kids, layout.Rigid(c))
		}
		// Rows center their children vertically; columns put them at the
		// start (CSS align-items: stretch / flex-start).
		align := layout.Middle
		if axis == layout.Vertical {
			align = layout.Start
		}
		return layout.Flex{Axis: axis, Alignment: align}.Layout(gtx, kids...)
	}
}

// If returns w when cond holds (else nothing).
func If(cond bool, w layout.Widget) layout.Widget {
	if cond {
		return w
	}
	return nil
}

// Label is a .field-label: 0.9em, at least 90px wide.
func (f *Form) Label(s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		d := gtk.Text{S: s, Size: gtk.Em(0.9), Color: f.Th.Pal.FgColor}.Layout(gtx, f.Th)
		d.Size.X = max(d.Size.X, gtx.Dp(90))
		return d
	}
}

// Text is plain text (0.9em; muted for hints).
func (f *Form) Text(s string, muted bool) layout.Widget {
	c := f.Th.Pal.FgColor
	if muted {
		c = f.Th.Pal.MutedFg
	}
	return gtk.Text{S: s, Size: gtk.Em(0.9), Color: c}.Widget(f.Th)
}

// Frame is a labelled frame around w.
func (f *Form) Frame(label string, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return f.Th.Frame(gtx, label, w) }
}

// Hint is a box on the row-hover color (e.g. manual authentication).
func (f *Form) Hint(s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		m := layout.UniformInset(12)
		call, d := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return m.Layout(gtx, gtk.Text{S: s, Size: gtk.Em(0.9), Color: f.Th.Pal.FgColor}.Widget(f.Th))
		})
		d.Size.X = gtx.Constraints.Max.X
		gtk.FillRect(gtx, image.Rectangle{Max: d.Size}, f.Th.Pal.RowHover)
		call.Add(gtx.Ops)
		return d
	}
}

// ---- fields ---------------------------------------------------------------------

// EntryOpts configure an entry.
type EntryOpts struct {
	Placeholder string
	Width       unit.Dp // 0 = 228px, -1 = all the room
	Password    bool
	ReadOnly    bool
	Disabled    bool
	Mono        bool
}

type entryState struct {
	ed   widget.Editor
	last string
}

// Entry edits *p.
func (f *Form) Entry(p *string, o EntryOpts) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st := get(f, p, func() *entryState { return &entryState{last: "\x00"} })
		st.ed.SingleLine = true
		if *p != st.last && !gtx.Focused(&st.ed) {
			st.ed.SetText(*p)
			st.last = *p
		}
		for {
			ev, ok := st.ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.ChangeEvent); ok && !o.ReadOnly && !f.Disabled && !o.Disabled {
				*p = st.ed.Text()
				st.last = *p
			}
		}
		st.ed.ReadOnly = o.ReadOnly
		st.ed.Mask = 0
		if o.Password {
			st.ed.Mask = '•'
		}
		return gtk.Entry{Placeholder: o.Placeholder, Width: o.Width, Password: o.Password, ReadOnly: o.ReadOnly, Disabled: f.Disabled || o.Disabled, Mono: o.Mono}.Layout(gtx, f.Th, &st.ed)
	}
}

// Check edits *p with a check button.
func (f *Form) Check(p *bool, label string, disabled bool) layout.Widget {
	return f.CheckSize(p, label, 0, disabled)
}

// CheckSize is Check with the label in size (0 = the GUI font).
func (f *Form) CheckSize(p *bool, label string, size unit.Sp, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := get(f, p, func() *widget.Bool { return &widget.Bool{} })
		if b.Update(gtx) && !f.Disabled && !disabled {
			*p = b.Value
		}
		b.Value = *p
		return f.Th.CheckButtonSize(gtx, b, label, size, f.Disabled || disabled)
	}
}

// Spin edits *p between min and max.
func (f *Form) Spin(p *int, min, max, step int, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		s := get(f, p, func() *gtk.SpinButton { return &gtk.SpinButton{} })
		s.Min, s.Max, s.Step = float64(min), float64(max), float64(max1(step))
		if !s.Changed() {
			s.Value = float64(*p)
		}
		d := s.Layout(gtx, f.Th, f.Disabled || disabled)
		if !f.Disabled && !disabled {
			*p = int(s.Value)
		}
		return d
	}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// Option is a choice of a combo or radio group.
type Option struct{ Value, Label string }

// Opts builds options from value/label pairs.
func Opts(pairs ...string) []Option {
	var out []Option
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Option{Value: pairs[i], Label: pairs[i+1]})
	}
	return out
}

// Combo edits *p with a drop-down list.
func (f *Form) Combo(p *string, opts []Option, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		c := get(f, p, func() *gtk.Combo { return &gtk.Combo{} })
		var o []gtk.ComboOption
		for _, op := range opts {
			o = append(o, gtk.ComboOption{Value: op.Value, Label: op.Label})
		}
		if c.Changed() && !f.Disabled && !disabled {
			*p = c.Value
		}
		c.Value = *p
		return c.Layout(gtx, f.Th, o, f.Disabled || disabled)
	}
}

// Radio edits *p with radio buttons (vertical or in a row).
func (f *Form) Radio(p *string, opts []Option, vertical, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		e := get(f, p, func() *widget.Enum { return &widget.Enum{} })
		if e.Update(gtx) && !f.Disabled && !disabled {
			*p = e.Value
		}
		e.Value = *p
		var o []gtk.RadioOption
		for _, op := range opts {
			o = append(o, gtk.RadioOption{Value: op.Value, Label: op.Label})
		}
		return f.Th.RadioGroup(gtx, e, o, vertical, f.Disabled || disabled)
	}
}

// Button is a push button; onClick runs when it is clicked.
func (f *Form) Button(label string, icon rune, disabled bool, onClick func()) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		c := f.Clicked("btn:" + label)
		if c.Clicked(gtx) && !disabled && onClick != nil {
			onClick()
		}
		return gtk.Button{Label: label, Icon: icon, Disabled: disabled}.Layout(gtx, f.Th, c)
	}
}

// File edits a path with a file chooser button (and Clear).
func (f *Form) File(p *string, title string, folder, disabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := get(f, p, func() *gtk.FileChooserButton { return &gtk.FileChooserButton{Folder: folder} })
		b.Path = *p
		if b.Clicked(gtx) && !f.Disabled && !disabled && f.ChooseFile != nil {
			dir := ""
			if *p != "" {
				dir = parentDir(*p)
			}
			go func() {
				path, err := f.ChooseFile(title, dir, folder)
				if err == nil && path != "" {
					f.Later(func() { *p = path })
				}
			}()
		}
		d := b.Layout(gtx, f.Th, f.Disabled || disabled)
		if b.Cleared() {
			*p = ""
		}
		return d
	}
}

func parentDir(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i <= 0 {
		return ""
	}
	return p[:i]
}

func record(gtx layout.Context, w layout.Widget) (callOp, layout.Dimensions) {
	return recordOp(gtx, w)
}
