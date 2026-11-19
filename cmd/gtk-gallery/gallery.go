package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// gallery holds the widgets' state, in the states of the reference page.
type gallery struct {
	section string
	th      *gtk.Theme

	buttons  [6]widget.Clickable
	checks   [3]widget.Bool
	radio    widget.Enum
	entries  [3]widget.Editor
	spin     gtk.SpinButton
	combo    gtk.Combo
	color    widget.Clickable
	frameChk widget.Bool
	frameEd  widget.Editor
	scale    widget.Float
	nb       gtk.Notebook
	tree     gtk.Tree
	menu     gtk.Menu
	msg      gtk.MessageDialog
	colors   gtk.ColorChooser
	fonts    gtk.FontChooser
	file     gtk.FileChooserButton
	fontBtn  gtk.FontButton
}

func newGallery(section string, th *gtk.Theme) *gallery {
	g := &gallery{section: section, th: th}
	g.checks[0].Value = true
	g.radio.Value = "b"
	g.entries[0].SetText("Entry text")
	g.entries[2].SetText("secret")
	g.spin = gtk.SpinButton{Value: 22, Min: 1, Max: 65535, Step: 1}
	g.combo.Value = "ssh"
	g.frameChk.Value = true
	g.frameEd.SetText("Second row")
	g.scale.Value = 0.3
	g.nb.Tabs = []*gtk.Tab{
		{Title: "Info", Icon: fonts.IconInfo},
		{Title: "web-prod-01", Icon: fonts.IconLock, IconColor: color.NRGBA{R: 0xf3, G: 0xc6, B: 0x3f, A: 0xff}, Color: color.NRGBA{R: 0x73, G: 0xd2, B: 0x16, A: 0xff}, Closable: true},
		{Title: "db-prod-01", Closable: true},
	}
	g.nb.Selected = 1
	g.tree = gtk.Tree{
		Root: &gtk.TreeNode{ID: "__root__", Name: "My Connections", Group: true, Children: []*gtk.TreeNode{
			{ID: "g1", Name: "Customers", Group: true, Children: []*gtk.TreeNode{
				{ID: "c1", Name: "acme-backup", Method: "ssh"},
				{ID: "c2", Name: "acme-erp", Method: "ssh", Protected: true},
			}},
			{ID: "g2", Name: "Home Lab", Group: true},
			{ID: "c3", Name: "web-prod-01", Method: "ssh", Favourite: true},
			{ID: "c4", Name: "switch-01", Method: "telnet"},
		}},
		Expanded:       map[string]bool{"__root__": true, "g1": true},
		Selected:       map[string]bool{"c3": true},
		ProtectedColor: color.NRGBA{R: 0xf5, G: 0xa6, B: 0x23, A: 0xff},
	}
	g.menu.Items = []gtk.MenuItem{
		{ID: "a", Label: "Connect", Icon: '\uf0ec'},
		{ID: "b", Label: "Edit", Shortcut: "F2"},
		{Separator: true},
		{ID: "c", Label: "Favourite", Checked: true},
		{ID: "d", Label: "Disabled", Disabled: true},
		{ID: "e", Label: "More", Submenu: []gtk.MenuItem{{ID: "f", Label: "Sub item"}}},
	}
	g.menu.Open(image.Pt(20, 20))
	g.colors.Open(color.NRGBA{R: 0x35, G: 0x84, B: 0xe4, A: 0xff})
	if fams, err := services.NewSystemService(nil).ListFonts(); err == nil {
		for _, f := range fams {
			g.fonts.Families = append(g.fonts.Families, gtk.FontFamily{Name: f.Name, Styles: f.Styles, Monospace: f.Monospace})
		}
	}
	g.fonts.Open(gtk.FontSetting{Family: "DejaVu Sans Mono", Style: "Book", Size: 11}, true)
	g.file = gtk.FileChooserButton{Path: "/home/user/.ssh/id_ed25519", ShowClear: true}
	g.msg = gtk.MessageDialog{Type: gtk.MessageQuestion, Title: "Quit", Primary: "Close all connections?", Secondary: "3 connections are still open.", Buttons: gtk.ButtonsYesNo}
	return g
}

func (g *gallery) Layout(gtx layout.Context) layout.Dimensions {
	gtk.Fill(gtx, g.th.Pal.BgColor)
	switch g.section {
	case "basics":
		return g.basics(gtx)
	case "tree":
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(300), gtx.Dp(260)))
		return g.tree.Layout(gtx, g.th)
	case "menu":
		return g.menu.Layout(gtx, g.th)
	case "message":
		return g.msg.Layout(gtx, g.th)
	case "color":
		return g.colors.Layout(gtx, g.th)
	case "fonts":
		return g.fonts.Layout(gtx, g.th)
	case "choosers":
		return layout.UniformInset(12).Layout(gtx, row(10,
			func(gtx layout.Context) layout.Dimensions { return g.file.Layout(gtx, g.th, false) },
			func(gtx layout.Context) layout.Dimensions {
				return g.fontBtn.Layout(gtx, g.th, gtk.FontSetting{Family: "DejaVu Sans Mono", Style: "Bold", Size: 11}, false)
			},
		))
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func row(gap unit.Dp, ws ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for i, w := range ws {
			if i > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Width: gap}.Layout))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	}
}

func (g *gallery) basics(gtx layout.Context) layout.Dimensions {
	th := g.th
	btn := func(i int, b gtk.Button) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return b.Layout(gtx, th, &g.buttons[i]) }
	}
	check := func(i int, label string, disabled bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return th.CheckButton(gtx, &g.checks[i], label, disabled) }
	}
	entry := func(i int, e gtk.Entry) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return e.Layout(gtx, th, &g.entries[i]) }
	}
	rows := []layout.Widget{
		row(10,
			btn(0, gtk.Button{Label: "Normal"}),
			btn(1, gtk.Button{Label: "Default", Default: true}),
			btn(2, gtk.Button{Label: "Disabled", Disabled: true}),
			btn(3, gtk.Button{Label: "With icon", Icon: '\uf0ec'}), // nf-fa-exchange
			btn(4, gtk.Button{Label: "Toggle off"}),
			btn(5, gtk.Button{Label: "Toggle on", Active: true}),
		),
		row(10,
			check(0, "Checked", false), check(1, "Unchecked", false), check(2, "Disabled", true),
			func(gtx layout.Context) layout.Dimensions {
				return th.RadioGroup(gtx, &g.radio, []gtk.RadioOption{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}, {Value: "c", Label: "Gamma"}}, false, false)
			},
		),
		row(10,
			entry(0, gtk.Entry{}), entry(1, gtk.Entry{Placeholder: "Placeholder"}), entry(2, gtk.Entry{Password: true}),
			func(gtx layout.Context) layout.Dimensions { return g.spin.Layout(gtx, th, false) },
			func(gtx layout.Context) layout.Dimensions {
				return g.combo.Layout(gtx, th, []gtk.ComboOption{{Value: "ssh", Label: "SSH"}, {Value: "telnet", Label: "Telnet"}}, false)
			},
		),
		func(gtx layout.Context) layout.Dimensions {
			return th.ColorButton(gtx, &g.color, color.NRGBA{R: 0x4a, G: 0x90, B: 0xd9, A: 0xff}, false)
		},
		row(10,
			func(gtx layout.Context) layout.Dimensions {
				return th.Frame(gtx, "Frame label", func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return th.CheckButton(gtx, &g.frameChk, "Inside a frame", false)
						}),
						layout.Rigid(layout.Spacer{Height: 6}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return gtk.Entry{}.Layout(gtx, th, &g.frameEd) }),
					)
				})
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(260)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.ProgressBar(gtx, 0.4, 100) }),
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Scale(gtx, &g.scale, false) }),
				)
			},
		),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(120)))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return g.nb.Layout(gtx, th) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return th.Label(gtx, "Tab t2", th.Pal.FgColor)
					})
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return th.Statusbar(gtx, layout.Rigid(th.StatusText("Status: CONNECTED")))
		},
	}
	return layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(956)
		var kids []layout.FlexChild
		for i, r := range rows {
			if i > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Height: 12}.Layout))
			}
			kids = append(kids, layout.Rigid(r))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}
