package dialogs

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/cluster"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Session is an open terminal as the cluster windows list it.
type Session struct{ TmpID, Title, Kind string }

// Clusters is what the cluster windows need of the services.
type Clusters struct {
	List      func() []cluster.ClusterInfo
	Sessions  func() []Session
	Create    func(name string) error
	Delete    func(name string)
	Add       func(tmpID, name string)
	Remove    func(tmpID string)
	Matches   func(rule config.AutoClusterConfig) []string
	SaveRule  func(rule config.AutoClusterConfig) error
	Saved     func() [][2]string // connection title, its configured clusters
	Send      func(name, text string, o services.PCCOptions) error
	Restart   func(name string) error
	CloseAll  func(name string) error
	PreventSB func(bool)
}

func (c *Clusters) options() []form.Option {
	var out []form.Option
	if c.List == nil {
		return nil
	}
	for _, cl := range c.List() {
		out = append(out, form.Option{Value: cl.Name, Label: fmt.Sprintf("%s (%d terminals)", cl.Name, len(cl.Sessions))})
	}
	return out
}

// ---- Cluster Administration ---------------------------------------------------------

// ClusterAdmin is ClusterAdminWindow.svelte.
type ClusterAdmin struct {
	C       *Clusters
	OpenPCC func()

	open                   bool
	tabs                   gtk.Notebook
	selected               string
	newName                string
	unclustered, clustered string
	rule                   config.AutoClusterConfig
	lastRule               config.AutoClusterConfig
	matches                []string
	f                      form.Form
	dlg                    gtk.Dialog
	left, right            clickableRows
	saved                  widget.List
	addTo, delFrom, delCl  widget.Clickable
	addCl, preview, saveR  widget.Clickable
	pcc, close             widget.Clickable
}

func (a *ClusterAdmin) Open() {
	a.open = true
	if len(a.tabs.Tabs) == 0 {
		for _, t := range []string{"Running clusters", "Saved clusters", "Auto Cluster"} {
			a.tabs.Tabs = append(a.tabs.Tabs, &gtk.Tab{Title: t})
		}
	}
	if a.selected == "" && a.C.List != nil {
		if l := a.C.List(); len(l) > 0 {
			a.selected = l[0].Name
		}
	}
}

func (a *ClusterAdmin) Opened() bool { return a.open }

func (a *ClusterAdmin) Layout(gtx layout.Context, th *gtk.Theme) {
	if !a.open {
		return
	}
	a.f.Th = th
	a.dlg.Title, a.dlg.Width, a.dlg.Closable = "Cluster Administration : LeanTerm", 750, true
	if a.close.Clicked(gtx) || a.dlg.CloseRequested() {
		a.open = false
		return
	}
	if a.pcc.Clicked(gtx) {
		a.open = false
		if a.OpenPCC != nil {
			a.OpenPCC()
		}
		return
	}
	c := a.C
	if a.addCl.Clicked(gtx) && strings.TrimSpace(a.newName) != "" {
		name := strings.TrimSpace(a.newName)
		if c.Create(name) == nil {
			a.selected, a.newName = name, ""
		}
	}
	if a.delCl.Clicked(gtx) && a.selected != "" {
		c.Delete(a.selected)
		a.selected = ""
	}
	if a.addTo.Clicked(gtx) && a.unclustered != "" && a.selected != "" {
		c.Add(a.unclustered, a.selected)
		a.unclustered = ""
	}
	if a.delFrom.Clicked(gtx) && a.clustered != "" {
		c.Remove(a.clustered)
		a.clustered = ""
	}
	if a.preview.Clicked(gtx) || (a.rule != a.lastRule && a.tabs.Selected == 2) {
		a.lastRule = a.rule
		if c.Matches != nil {
			a.matches = c.Matches(a.rule)
		}
	}
	if a.saveR.Clicked(gtx) && a.rule.Name != "" && c.SaveRule != nil {
		if c.SaveRule(a.rule) == nil {
			a.rule, a.lastRule, a.matches = config.AutoClusterConfig{}, config.AutoClusterConfig{}, nil
		}
	}
	a.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.tabs.Layout(gtx, th) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// .tab-content: min-height 340px, padding 10px 0.
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					d := layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						switch a.tabs.Selected {
						case 0:
							return a.running(gtx, th)
						case 1:
							return a.savedTab(gtx, th)
						}
						return a.auto(gtx, th)
					})
					d.Size.Y = max(d.Size.Y, gtx.Dp(340))
					return d
				}),
			)
		})
	}, button(th, &a.pcc, "Power Cluster Controller", fonts.IconList, false, false),
		button(th, &a.close, "Close", 0, false, false))
}

func colTitle(th *gtk.Theme, s string) layout.Widget {
	return text(th, strings.ToUpper(s), px11, th.Pal.MutedFg, font.SemiBold)
}

func (a *ClusterAdmin) running(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	c := a.C
	var sessions []Session
	if c.Sessions != nil {
		sessions = c.Sessions()
	}
	in := map[string]bool{}
	var members map[string]bool
	for _, cl := range c.List() {
		for _, s := range cl.Sessions {
			in[s] = true
		}
		if cl.Name == a.selected {
			members = map[string]bool{}
			for _, s := range cl.Sessions {
				members[s] = true
			}
		}
	}
	var free, inCl []Session
	for _, s := range sessions {
		if !in[s.TmpID] {
			free = append(free, s)
		}
		if members[s.TmpID] {
			inCl = append(inCl, s)
		}
	}
	label := func(l []Session) []string {
		var out []string
		for _, s := range l {
			t := s.Title
			if t == "" {
				t = "Terminal"
			}
			out = append(out, fmt.Sprintf("%s (%s)", t, s.Kind))
		}
		return out
	}
	index := func(l []Session, id string) int {
		for i, s := range l {
			if s.TmpID == id {
				return i
			}
		}
		return -1
	}
	list := func(rows *clickableRows, l []Session, sel *string, empty string, minH unit.Dp) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return rows.layout(gtx, th, label(l), index(l, *sel), empty, minH, 220, func(i int) { *sel = l[i].TmpID })
		}
	}
	f := &a.f
	right := gapCol(6,
		colTitle(th, "Active Clusters"),
		form.FlexRow(6, f.Combo(&a.selected, c.options(), false), button(th, &a.delCl, "Delete", 0, false, a.selected == ""), form.Grow{W: layout.Spacer{}.Layout}),
		form.FlexRow(6, form.Grow{W: f.Entry(&a.newName, form.EntryOpts{Placeholder: "New cluster name...", Width: -1})},
			button(th, &a.addCl, "Add", 0, false, strings.TrimSpace(a.newName) == "")),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8}.Layout(gtx, colTitle(th, "Terminals in Cluster"))
		},
		list(&a.right, inCl, &a.clustered, "No terminals in this cluster", 180),
	)
	mid := gapCol(12,
		button(th, &a.addTo, "Add to Cluster ›", 0, false, a.unclustered == "" || a.selected == ""),
		button(th, &a.delFrom, "‹ Del from Cluster", 0, false, a.clustered == ""))
	// .running-tab: three columns (1fr, 140px, 1fr), gap 12px, stretched
	// to the tallest: the left list grows (up to 220px), the buttons are
	// centered.
	gap, midW := gtx.Dp(12), gtx.Dp(140)
	cw := (gtx.Constraints.Max.X - 2*gap - midW) / 2
	col := gtx
	col.Constraints = layout.Constraints{Max: image.Pt(cw, gtx.Constraints.Max.Y)}
	col.Constraints.Min.X = cw
	m := op.Record(gtx.Ops)
	rd := right(col)
	rc := m.Stop()
	m = op.Record(gtx.Ops)
	td := colTitle(th, "Unclustered Terminals")(col)
	tc := m.Stop()
	h := rd.Size.Y
	listH := min(unit.Dp(float32(h-td.Size.Y-gtx.Dp(6))/gtx.Metric.PxPerDp), 220)
	m = op.Record(gtx.Ops)
	ld := list(&a.left, free, &a.unclustered, "No unclustered terminals", listH)(col)
	lc := m.Stop()
	h = max(h, td.Size.Y+gtx.Dp(6)+ld.Size.Y)
	tc.Add(gtx.Ops)
	o := op.Offset(image.Pt(0, td.Size.Y+gtx.Dp(6))).Push(gtx.Ops)
	lc.Add(gtx.Ops)
	o.Pop()
	mg := gtx
	mg.Constraints = layout.Constraints{Max: image.Pt(midW, h)}
	m = op.Record(gtx.Ops)
	md := mid(mg)
	mc := m.Stop()
	o = op.Offset(image.Pt(cw+gap, (h-md.Size.Y)/2)).Push(gtx.Ops)
	mc.Add(gtx.Ops)
	o.Pop()
	o = op.Offset(image.Pt(cw+2*gap+midW, 0)).Push(gtx.Ops)
	rc.Add(gtx.Ops)
	o.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

func (a *ClusterAdmin) savedTab(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	pal := th.Pal
	var rows [][2]string
	if a.C.Saved != nil {
		rows = a.C.Saved()
	}
	return gapCol(10,
		wrapped(th, "Configure static clusters assigned to connections. When launched, connections are automatically grouped into these clusters.", px12, pal.MutedFg),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(280))
			return boxed(gtx, pal.BaseColor, pal.BorderColor, 4, layout.UniformInset(1), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				a.saved.Axis = layout.Vertical
				return a.saved.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					d := layout.Inset{Top: 6, Bottom: 6, Left: 12, Right: 12}.Layout(gtx, form.FlexRow(8,
						form.Grow{W: text(th, rows[i][0], px12, pal.FgColor, font.Normal)},
						text(th, rows[i][1], px12, pal.FgColor, font.Normal)))
					gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
					return d
				})
			})
		},
	)(gtx)
}

func (a *ClusterAdmin) auto(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	pal := th.Pal
	f := &a.f
	lbl := func(s string) layout.Widget { return text(th, s, px12, pal.FgColor, font.Normal) }
	e := func(p *string, ph string) layout.Widget {
		return f.Entry(p, form.EntryOpts{Placeholder: ph, Width: -1})
	}
	var preview layout.Widget
	if len(a.matches) > 0 {
		items := []layout.Widget{text(th, fmt.Sprintf("Matches (%d):", len(a.matches)), px12, pal.FgColor, font.Normal)}
		for _, m := range a.matches {
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 24}.Layout(gtx, text(th, "• "+m, px12, pal.FgColor, font.Normal))
			})
		}
		preview = func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return boxed(gtx, pal.BaseColor, pal.BorderColor, 4, layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return gapCol(4, items...)(gtx)
				})
			})
		}
	}
	return gapCol(0,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: 10}.Layout(gtx, wrapped(th, "Automatically cluster connections based on regular expressions matching Name, Title, Host or Description.", px12, pal.MutedFg))
		},
		grid(140, 8, 8,
			form.GridRow{Label: lbl("Cluster Name:"), Field: e(&a.rule.Name, "Production")},
			form.GridRow{Label: lbl("Match Title (regex):"), Field: e(&a.rule.MatchTitle, "^prod-.*")},
			form.GridRow{Label: lbl("Match Host (regex):"), Field: e(&a.rule.MatchHost, `^10\.0\..*`)},
			form.GridRow{Label: lbl("Match Desc (regex):"), Field: e(&a.rule.MatchDescription, ".*database.*")},
		),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, form.Row(
					button(th, &a.preview, "Preview Matches", fonts.IconSearch, false, false),
					button(th, &a.saveR, "Save Rule", fonts.IconSave, false, a.rule.Name == "")))
			})
		},
		preview,
	)(gtx)
}

// ---- Power Cluster Controller ---------------------------------------------------------

// PCC is PCCWindow.svelte: type into all terminals of a cluster.
type PCC struct {
	C            *Clusters
	OpenClusters func()

	open                                    bool
	selected                                string
	all, prevent, multi, onEnter, useVars   bool
	single                                  widget.Editor
	multiEd                                 widget.Editor
	status                                  string
	f                                       form.Form
	dlg                                     gtk.Dialog
	restart, closeAll, send, clear, execAll widget.Clickable
	clusters, close                         widget.Clickable
}

func (p *PCC) Open() {
	if !p.open {
		p.onEnter, p.useVars = true, true
	}
	p.open = true
	if p.selected == "" && p.C.List != nil {
		if l := p.C.List(); len(l) > 0 {
			p.selected = l[0].Name
		}
	}
}

func (p *PCC) Opened() bool { return p.open }

func (p *PCC) sendText(s string) {
	if strings.TrimSpace(s) == "" || p.C.Send == nil {
		return
	}
	err := p.C.Send(p.selected, s, services.PCCOptions{SendToAll: p.all, PreventSingleBroadcast: p.prevent, UseVariables: p.useVars, AppendReturn: true})
	if err != nil {
		p.status = "Error: " + err.Error()
		return
	}
	shown := s
	if len(shown) > 40 {
		shown = shown[:40] + "..."
	}
	p.status = "Sent: " + shown
}

func (p *PCC) Layout(gtx layout.Context, th *gtk.Theme) {
	if !p.open {
		return
	}
	p.f.Th = th
	p.dlg.Title, p.dlg.Width, p.dlg.Closable = "LeanTerm: Power Cluster Controller", 700, true
	if p.close.Clicked(gtx) || p.dlg.CloseRequested() {
		p.open = false
		return
	}
	if p.clusters.Clicked(gtx) {
		p.open = false
		if p.OpenClusters != nil {
			p.OpenClusters()
		}
		return
	}
	c := p.C
	if p.restart.Clicked(gtx) && (p.selected != "" || p.all) && c.Restart != nil {
		if err := c.Restart(p.selected); err != nil {
			p.status = "Error: " + err.Error()
		} else {
			p.status = "Restart command sent to cluster"
		}
	}
	if p.closeAll.Clicked(gtx) && (p.selected != "" || p.all) && c.CloseAll != nil {
		if err := c.CloseAll(p.selected); err != nil {
			p.status = "Error: " + err.Error()
		} else {
			p.status = "Close command sent to cluster"
		}
	}
	p.single.SingleLine, p.single.Submit = true, true
	for {
		ev, ok := p.single.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok && p.onEnter {
			p.submitSingle()
		}
	}
	if p.send.Clicked(gtx) {
		p.submitSingle()
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.multiEd, Name: key.NameReturn, Required: key.ModShortcut})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			p.sendText(p.multiEd.Text())
		}
	}
	if p.execAll.Clicked(gtx) {
		p.sendText(p.multiEd.Text())
	}
	if p.clear.Clicked(gtx) {
		p.multiEd.SetText("")
	}
	prevent := p.prevent
	pal := th.Pal
	f := &p.f
	lines := func(w layout.Widget) layout.Widget {
		// .button-row: padding 6px 0 between two lines.
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			d := layout.Inset{Top: 7, Bottom: 7}.Layout(gtx, w)
			gtk.FillRect(gtx, image.Rect(0, 0, d.Size.X, gtx.Dp(1)), pal.BorderColor)
			gtk.FillRect(gtx, image.Rect(0, d.Size.Y-gtx.Dp(1), d.Size.X, d.Size.Y), pal.BorderColor)
			return d
		}
	}
	var entry layout.Widget
	if !p.multi {
		entry = gapCol(8,
			text(th, "Type commands here:", px12, pal.FgColor, font.Normal),
			form.Row(func(gtx layout.Context) layout.Dimensions {
				return gtk.Entry{Placeholder: "Enter command to broadcast...", Width: 176, Size: gtk.Em(px12)}.Layout(gtx, th, &p.single)
			}, button(th, &p.send, "Send", fonts.IconCogs, false, p.single.Text() == "")),
			form.RowGap(16, f.CheckSize(&p.onEnter, "Send on <ENTER>", gtk.Em(px12), false), f.CheckSize(&p.useVars, "Use LeanTerm variables", gtk.Em(px12), false)),
		)
	} else {
		entry = gapCol(8,
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Max.Y = gtx.Dp(176) // rows=10
				return gtk.TextArea{Placeholder: "Type multi-line script or commands here (Ctrl+Enter to execute all)...", Mono: true}.Layout(gtx, th, &p.multiEd)
			},
			form.FlexRow(8,
				button(th, &p.clear, "Clear", fonts.IconTrash, false, false),
				f.Check(&p.useVars, "Use LeanTerm variables", false),
				form.Grow{W: func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.E.Layout(gtx, button(th, &p.execAll, "Execute All (Ctrl+Enter)", fonts.IconCogs, false, p.multiEd.Text() == ""))
				}},
			),
		)
	}
	p.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(14).Layout(gtx, gapCol(12,
			form.FlexRow(12,
				form.Grow{W: form.FlexRow(8, text(th, "Send to Cluster:", px12, pal.FgColor, font.Normal), f.Combo(&p.selected, c.options(), p.all))},
				f.CheckSize(&p.all, "Send to ALL terminals", gtk.Em(px12), false)),
			lines(form.Wrap(8,
				button(th, &p.restart, "Restart all", fonts.IconRefresh, false, false),
				button(th, &p.closeAll, "Close ALL", fonts.IconClose, false, false),
				f.CheckSize(&p.prevent, "Prevent single window keys broadcast", gtk.Em(px12), false),
				f.CheckSize(&p.multi, "Use Multi-Line Text Entry", gtk.Em(px12), false))),
			entry,
			form.If(p.status != "", statusBox(th, p.status, pal.Accent, true)),
		))
	}, button(th, &p.clusters, "Clusters", fonts.IconSitemap, false, false),
		button(th, &p.close, "Close", 0, false, false))
	if p.prevent != prevent && c.PreventSB != nil {
		c.PreventSB(p.prevent)
	}
	_ = unit.Dp(0)
}

func (p *PCC) submitSingle() {
	if t := p.single.Text(); t != "" {
		p.sendText(t)
		p.single.SetText("")
	}
}
