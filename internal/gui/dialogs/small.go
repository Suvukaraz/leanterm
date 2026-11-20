package dialogs

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// ---- Wake On LAN ---------------------------------------------------------------

// WakeOnLan is WakeOnLanDialog.svelte.
type WakeOnLan struct {
	Send       func(mac, host string, port int, broadcast bool) error
	LookupArp  func(host string) (string, error)
	Invalidate func()

	open       bool
	mac, host  string
	port       int
	broadcast  bool
	status     string
	sending    bool
	closeAt    time.Time
	f          form.Form
	dlg        gtk.Dialog
	arp, close widget.Clickable
	wake       widget.Clickable
	mb         mailbox
}

// Open shows the dialog for a host (and MAC address, if known).
func (w *WakeOnLan) Open(host, mac string) {
	*w = WakeOnLan{Send: w.Send, LookupArp: w.LookupArp, Invalidate: w.Invalidate, mb: w.mb,
		open: true, host: host, mac: mac, port: 9, broadcast: true}
}

func (w *WakeOnLan) Opened() bool { return w.open }

func (w *WakeOnLan) Layout(gtx layout.Context, th *gtk.Theme) {
	w.mb.drain()
	if !w.open {
		return
	}
	if !w.closeAt.IsZero() && !gtx.Now.Before(w.closeAt) {
		w.open = false
		return
	}
	w.f.Th = th
	w.dlg.Title, w.dlg.Width, w.dlg.Closable = "Wake On LAN", 480, true
	if w.close.Clicked(gtx) || w.dlg.CloseRequested() {
		w.open = false
		return
	}
	if w.arp.Clicked(gtx) {
		if w.host == "" {
			w.status = "Please enter a host or IP first"
		} else if w.LookupArp != nil {
			host := w.host
			w.status = fmt.Sprintf("Checking MAC for '%s' ...", host)
			lookup := w.LookupArp
			w.mb.run(w.Invalidate, func() func() {
				found, err := lookup(host)
				return func() {
					switch {
					case err != nil:
						w.status = "Failed: " + err.Error()
					case found != "":
						w.mac, w.status = found, "Found MAC: "+found
					default:
						w.status = "MAC not found in ARP table"
					}
				}
			})
		}
	}
	if w.wake.Clicked(gtx) && w.mac != "" && !w.sending && w.Send != nil {
		w.sending, w.status = true, "Sending Wake-on-LAN magic packet..."
		mac, host, port, bc := w.mac, w.host, w.port, w.broadcast
		send := w.Send
		w.mb.run(w.Invalidate, func() func() {
			err := send(mac, host, port, bc)
			return func() {
				w.sending = false
				if err != nil {
					w.status = "Error sending packet: " + err.Error()
					return
				}
				// Closes a second later (the frame is asked for below).
				w.status = "Magic packet sent successfully!"
				w.closeAt = time.Now().Add(time.Second)
			}
		})
	}
	pal := th.Pal
	f := &w.f
	lbl := func(s string) layout.Widget { return text(th, s, px12, pal.FgColor, font.Normal) }
	w.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(16).Layout(gtx, gapCol(12,
			grid(120, 8, 10,
				form.GridRow{Label: lbl("MAC Address:"), Field: form.FlexRow(6,
					form.Grow{W: f.Entry(&w.mac, form.EntryOpts{Placeholder: "00:11:22:33:44:55", Width: -1})},
					button(th, &w.arp, "Check ARP", fonts.IconSearch, false, false))},
				form.GridRow{Label: lbl("Host / IP:"), Field: f.Entry(&w.host, form.EntryOpts{Placeholder: "192.168.1.100", Width: -1})},
				form.GridRow{Label: lbl("Port Number:"), Field: f.Spin(&w.port, 1, 65535, 1, w.broadcast)},
				form.GridRow{Label: func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} },
					Field: f.Check(&w.broadcast, "Send to broadcast (255.255.255.255:9)", false)},
			),
			form.If(w.status != "", statusBox(th, w.status, pal.Accent, true)),
		))
	}, button(th, &w.close, "Close", 0, false, false),
		button(th, &w.wake, "Wake up", fonts.IconSitemap, false, w.mac == "" || w.sending))
	if !w.closeAt.IsZero() {
		gtx.Execute(invalidateAt(w.closeAt))
	}
}

// ---- Quick Connect ---------------------------------------------------------------

// QuickConnect is QuickConnectDialog.svelte: a connection that is not saved.
type QuickConnect struct {
	Connect    func(title, method, host string, port int, user, password string) error
	Invalidate func()

	open                              bool
	method, title, host, user, passwd string
	port                              int
	connecting                        bool
	err                               string
	f                                 form.Form
	dlg                               gtk.Dialog
	close, connect                    widget.Clickable
	mb                                mailbox
}

var quickMethods = form.Opts("ssh", "SSH", "telnet", "Telnet", "sftp", "SFTP Browser")

func (q *QuickConnect) Open() {
	*q = QuickConnect{Connect: q.Connect, Invalidate: q.Invalidate, mb: q.mb, open: true, method: "ssh", title: "Quick Connect", port: 22}
}

func (q *QuickConnect) Opened() bool { return q.open }

func (q *QuickConnect) submit() {
	if strings.TrimSpace(q.host) == "" {
		q.err = "Please enter a target host."
		return
	}
	if q.Connect == nil || q.connecting {
		return
	}
	q.connecting, q.err = true, ""
	title := strings.TrimSpace(q.title)
	if title == "" {
		title = strings.TrimSpace(q.host)
	}
	method, host, port, user, pw := q.method, strings.TrimSpace(q.host), q.port, strings.TrimSpace(q.user), q.passwd
	connect := q.Connect
	q.mb.run(q.Invalidate, func() func() {
		err := connect(title, method, host, port, user, pw)
		return func() {
			if err != nil {
				q.err, q.connecting = err.Error(), false
				return
			}
			q.open = false
		}
	})
}

func (q *QuickConnect) Layout(gtx layout.Context, th *gtk.Theme) {
	q.mb.drain()
	if !q.open {
		return
	}
	q.f.Th = th
	q.dlg.Title, q.dlg.Banner, q.dlg.Width, q.dlg.Closable = "Quick Connect", "Connect to a remote server without saving to configuration", 420, true
	if q.close.Clicked(gtx) || q.dlg.CloseRequested() {
		q.open = false
		return
	}
	if q.connect.Clicked(gtx) {
		q.submit()
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			q.submit()
		}
	}
	pal := th.Pal
	f := &q.f
	method := func(gtx layout.Context) layout.Dimensions {
		prev := q.method
		d := f.Combo(&q.method, quickMethods, false)(gtx)
		if q.method != prev {
			if q.method == "telnet" && q.port == 22 {
				q.port = 23
			} else if q.method != "telnet" && q.port == 23 {
				q.port = 22
			}
		}
		return d
	}
	row := func(label string, bold bool, w layout.Widget) layout.Widget {
		wt := font.Normal
		if bold {
			wt = font.Bold
		}
		return form.FlexRow(12, fixed(100, text(th, label, px12, pal.FgColor, wt)), form.Grow{W: w})
	}
	entry := func(p *string, ph string, pw bool) layout.Widget {
		return f.Entry(p, form.EntryOpts{Placeholder: ph, Password: pw, Width: -1})
	}
	label := "Connect"
	if q.connecting {
		label = "Connecting..."
	}
	q.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return gapCol(10,
			form.If(q.err != "", func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return errorBanner(gtx, th, q.err)
			}),
			row("Method:", true, method),
			row("Host:", true, entry(&q.host, "e.g. 192.168.1.100 or server.example.com", false)),
			row("Port:", true, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(100))
				return f.Spin(&q.port, 1, 65535, 1, false)(gtx)
			}),
			row("User:", true, entry(&q.user, "Username", false)),
			row("Password:", true, entry(&q.passwd, "Password (kept in memory only)", true)),
			row("Title (optional):", false, entry(&q.title, "Display title for tab", false)),
		)(gtx)
	}, button(th, &q.close, "Close", 0, false, false),
		button(th, &q.connect, label, 0, true, q.connecting))
}

// errorBanner is .error-banner: the error colors, radius 3px, 11px text.
func errorBanner(gtx layout.Context, th *gtk.Theme, s string) layout.Dimensions {
	pal := th.Pal
	return boxed(gtx, pal.ErrorBg, pal.ErrorBorder, 3, layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X // a block
			return layout.W.Layout(gtx, wrapped(th, s, px11, pal.ErrorFg))
		})
}

// ---- Edit Local Variables ----------------------------------------------------------

// EditVariables is EditVariablesDialog.svelte: the ten <V:n> of a connection.
type EditVariables struct {
	Load       func(id string) (services.ConnectionEdit, error)
	Save       func(id string, e services.ConnectionEdit) error
	Invalidate func()

	open         bool
	id           string
	edit         services.ConnectionEdit
	rows         [10]config.UserVariable
	status       string
	saving       bool
	f            form.Form
	dlg          gtk.Dialog
	cancel, save widget.Clickable
}

func (e *EditVariables) Open(connID string) {
	*e = EditVariables{Load: e.Load, Save: e.Save, Invalidate: e.Invalidate, open: true, id: connID}
	edit, err := e.Load(connID)
	if err != nil {
		e.status = "Error: " + err.Error()
		return
	}
	e.edit = edit
	for i := range e.rows {
		e.rows[i] = config.UserVariable{ID: fmt.Sprintf("var_%d", i)}
		if i < len(edit.Variables) {
			v := edit.Variables[i]
			if v.ID != "" {
				e.rows[i].ID = v.ID
			}
			e.rows[i].Value, e.rows[i].Hidden = v.Value, v.Hidden
		}
	}
}

func (e *EditVariables) Opened() bool { return e.open }

func (e *EditVariables) Layout(gtx layout.Context, th *gtk.Theme) {
	if !e.open {
		return
	}
	e.f.Th = th
	e.dlg.Title, e.dlg.Width, e.dlg.Closable = "Edit Local Variables (<V:0>..<V:9>)", 520, true
	if e.cancel.Clicked(gtx) || e.dlg.CloseRequested() {
		e.open = false
		return
	}
	if e.save.Clicked(gtx) && !e.saving && e.Save != nil {
		e.edit.Variables = append([]config.UserVariable(nil), e.rows[:]...)
		if err := e.Save(e.id, e.edit); err != nil {
			e.status = "Error: " + err.Error()
		} else {
			e.open = false
			return
		}
	}
	pal := th.Pal
	f := &e.f
	table := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return fill(gtx, pal.HeaderBg, layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}, form.FlexRow(0,
				fixed(50, text(th, "Var", px11, pal.FgColor, font.SemiBold)),
				form.Grow{W: text(th, "Value", px11, pal.FgColor, font.SemiBold)},
				fixed(70, layout.Widget(func(gtx layout.Context) layout.Dimensions {
					return layout.N.Layout(gtx, text(th, "Hide", px11, pal.FgColor, font.SemiBold))
				}))))
		}}
		for i := range e.rows {
			r := &e.rows[i]
			rows = append(rows, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, form.FlexRow(8,
					fixed(50, gtk.Text{S: fmt.Sprintf("<V:%d>", i), Size: gtk.Em(px12), Weight: font.SemiBold, Mono: true, Color: pal.Accent}.Widget(th)),
					form.Grow{W: f.Entry(&r.Value, form.EntryOpts{Placeholder: "Value...", Password: r.Hidden, Width: -1})},
					fixed(70, func(gtx layout.Context) layout.Dimensions {
						return layout.N.Layout(gtx, f.Check(&r.Hidden, "", false))
					}),
				))
			})
		}
		return boxed(gtx, pal.BaseColor, pal.BorderColor, 4, layout.Inset{}, rowsWithLines(pal.BorderColor, rows...))
	}
	e.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(14).Layout(gtx, gapCol(12,
			wrapped(th, "Configure user variables for this connection. Reference them in commands or scripts using <V:0> through <V:9>.", px12, pal.MutedFg),
			table,
			form.If(e.status != "", func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(gtx, text(th, e.status, px12, pal.ErrorFg, font.Normal))
			}),
		))
	}, button(th, &e.cancel, "Cancel", 0, false, false),
		button(th, &e.save, "Save", fonts.IconSave, false, e.saving))
}

// ---- Bulk Edit -----------------------------------------------------------------------

// BulkEdit is BulkEditDialog.svelte: change fields of many connections.
type BulkEdit struct {
	Apply      func(ids []string, rules []config.BulkEditRule) (config.BulkEditResult, error)
	Applied    func() // the tree reloads
	Invalidate func()

	open          bool
	ids           []string
	rows          []bulkRow
	status        string
	saving        bool
	closeAt       time.Time
	f             form.Form
	dlg           gtk.Dialog
	cancel, apply widget.Clickable
}

type bulkRow struct {
	change       bool
	field, label string
	value, match string
	regex        bool
}

func (b *BulkEdit) Open(ids []string) {
	*b = BulkEdit{Apply: b.Apply, Applied: b.Applied, Invalidate: b.Invalidate, open: true, ids: ids}
	for _, f := range [][2]string{{"user", "User"}, {"host", "Host / IP"}, {"port", "Port"}, {"title", "Title"}, {"description", "Description"}} {
		b.rows = append(b.rows, bulkRow{field: f[0], label: f[1]})
	}
}

func (b *BulkEdit) Opened() bool { return b.open }

func (b *BulkEdit) applyNow() {
	var rules []config.BulkEditRule
	for _, r := range b.rows {
		if r.change {
			rules = append(rules, config.BulkEditRule{Field: r.field, NewValue: r.value, UseRegex: r.regex, MatchRegex: r.match})
		}
	}
	if len(rules) == 0 {
		b.status = "Please select at least one field to change."
		return
	}
	if b.Apply == nil {
		return
	}
	res, err := b.Apply(b.ids, rules)
	if err != nil {
		b.status = "Error applying bulk edit: " + err.Error()
		return
	}
	b.status = fmt.Sprintf("Modified %d connection(s).", res.ModifiedCount)
	if b.Applied != nil {
		b.Applied()
	}
	b.closeAt = time.Now().Add(time.Second)
}

func (b *BulkEdit) Layout(gtx layout.Context, th *gtk.Theme) {
	if !b.open {
		return
	}
	if !b.closeAt.IsZero() && !gtx.Now.Before(b.closeAt) {
		b.open = false
		return
	}
	b.f.Th = th
	b.dlg.Title, b.dlg.Width, b.dlg.Closable = fmt.Sprintf("Bulk Edit (%d connections selected)", len(b.ids)), 680, true
	if b.cancel.Clicked(gtx) || b.dlg.CloseRequested() {
		b.open = false
		return
	}
	if b.apply.Clicked(gtx) && !b.saving {
		b.applyNow()
	}
	pal := th.Pal
	f := &b.f
	// Columns: 40px, 90px, auto, 70px, auto (a table with 1px borders).
	cols := func(cells ...layout.Widget) layout.Widget {
		return tableRow(pal.BorderColor, []float32{0, 0, 1, 0, 1}, []unit.Dp{48, 98, 0, 78, 0}, cells...)
	}
	center := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.N.Layout(gtx, w)
		}
	}
	head := func(s string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return fill(gtx, pal.BaseColor, layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}, center(text(th, s, px12, pal.FgColor, font.SemiBold)))
		}
	}
	cell := func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4, Bottom: 4, Left: 6, Right: 6}.Layout(gtx, w)
		}
	}
	rows := []layout.Widget{cols(head("Change"), head("Field"), head("New Value / Replacement"), head("RegExp"), head("Match Pattern"))}
	for i := range b.rows {
		r := &b.rows[i]
		ph := ""
		if r.regex {
			ph = "^prefix_(.*)"
		}
		rows = append(rows, cols(
			cell(center(f.Check(&r.change, "", false))),
			cell(text(th, r.label, px12, pal.FgColor, font.Medium)),
			cell(f.Entry(&r.value, form.EntryOpts{Placeholder: "Value...", Disabled: !r.change, Width: -1})),
			cell(center(f.Check(&r.regex, "", !r.change))),
			cell(f.Entry(&r.match, form.EntryOpts{Placeholder: ph, Disabled: !r.change || !r.regex, Width: -1})),
		))
	}
	b.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(14).Layout(gtx, gapCol(12,
			wrapped(th, "Select fields to update across all selected connections. Regex substitutions support capture groups ($1, $2, etc.).", px12, pal.MutedFg),
			table(pal.BorderColor, rows...),
			form.If(b.status != "", statusBox(th, b.status, pal.FgColor, true)),
		))
	}, button(th, &b.cancel, "Cancel", 0, false, false),
		button(th, &b.apply, "Apply Changes", fonts.IconCheck, false, b.saving))
	if !b.closeAt.IsZero() {
		gtx.Execute(invalidateAt(b.closeAt))
	}
}
