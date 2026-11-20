package editconn

import (
	"fmt"
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// supportedSSHOptions are the ssh_config options of OptionsTab.svelte.
var supportedSSHOptions = []string{
	"AddressFamily", "BindAddress", "Ciphers", "ConnectTimeout", "ExitOnForwardFailure", "ForwardAgent", "ForwardX11",
	"GatewayPorts", "HostKeyAlgorithms", "HostName", "IdentitiesOnly", "IdentityFile", "KexAlgorithms", "MACs", "Port",
	"PreferredAuthentications", "ProxyJump", "PubkeyAcceptedAlgorithms", "RequestTTY", "SendEnv", "ServerAliveCountMax",
	"ServerAliveInterval", "SetEnv", "StrictHostKeyChecking", "User", "UserKnownHostsFile",
}

func record(gtx layout.Context, w layout.Widget) (op.CallOp, layout.Dimensions) {
	m := op.Record(gtx.Ops)
	d := w(gtx)
	return m.Stop(), d
}

// optionsTab is OptionsTab.svelte: SSH (IP protocol, other options, the
// forwarding and ssh_config notebook) or Telnet options.
func (d *Dialog) optionsTab(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	f, c := &d.F, &d.c
	if c.Method == "telnet" {
		t := &c.Telnet
		return d.scroll(gtx, f.Frame("Telnet Options", form.Col(
			form.Row(f.Label("IP Protocol:"), f.Combo(&t.IPVersion, ipOptions, false)),
			form.Row(f.Label("Bind Address:"), f.Entry(&t.BindAddress, form.EntryOpts{Placeholder: "Outgoing IP (leave blank for any)"})),
			form.Row(f.Label("Escape Character:"), f.Entry(&t.EscapeChar, form.EntryOpts{Placeholder: "^]"})),
			f.Check(&t.NAWS, "Send window size (NAWS)", false),
			form.Row(f.Label("Terminal type:"), f.Entry(&t.TerminalType, form.EntryOpts{Placeholder: "XTERM-256COLOR"})),
			form.Row(f.Label("Line ending:"), f.Combo(&t.LineEnding, lineEndings, false)),
		)))
	}
	s := &c.SSH
	if d.advFor != c.ID {
		d.advFor, d.adv = c.ID, nil
		var keys []string
		for k := range s.Options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			d.adv = append(d.adv, kv{k, s.Options[k]})
		}
	}
	d.syncAdv()
	titles := []string{
		fmt.Sprintf("Local Port Forwarding (%d)", len(s.LocalForwards)),
		fmt.Sprintf("Remote Port Forwarding (%d)", len(s.RemoteForwards)),
		fmt.Sprintf("Dynamic Socks Proxy (%d)", len(s.DynamicForwards)),
		fmt.Sprintf("Advanced Options (%d)", len(d.adv)),
	}
	if len(d.fwd.Tabs) != 4 {
		d.fwd.Tabs = nil
		for range titles {
			d.fwd.Tabs = append(d.fwd.Tabs, &gtk.Tab{})
		}
	}
	for i, t := range titles {
		d.fwd.Tabs[i].Title = t
	}
	top := form.Section(
		form.Row(f.Label("IP Protocol:"), f.Combo(&s.IPVersion, ipOptions, false)),
		form.If(c.Method == "ssh", f.Frame("Other Options", form.RowGap(12,
			f.Check(&s.ForwardX11, "Forward X", false),
			f.Check(&s.GatewayPorts, "Allow Remote Port Connect", false),
			f.Check(&s.ForwardAgent, "Forward Agent", false),
			f.Check(&s.NoRemoteCommand, "Do NOT execute remote command", false),
			f.Check(&s.RandomSOCKSTunnel, "Create random SOCKS tunnel", false),
		))),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: 10, Left: 10, Right: 10, Bottom: 8}.Layout(gtx, top)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.fwd.Layout(gtx, th) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return d.scroll(gtx, d.forwards(th)) }),
	)
}

// grid is a header row and rows of fixed columns (flex 0 = 110px IP
// columns, the rest grow).
func headerRow(th *gtk.Theme, labels ...string) layout.Widget {
	var kids []any
	for _, l := range labels {
		w := gtk.Text{S: l, Size: gtk.Em(0.85), Weight: 600, Color: th.Pal.MutedFg}.Widget(th)
		if l == "" {
			kids = append(kids, layout.Widget(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Dp(32), 0)}
			}))
			continue
		}
		kids = append(kids, form.Grow{W: w})
	}
	return form.FlexRow(6, kids...)
}

func (d *Dialog) forwards(th *gtk.Theme) layout.Widget {
	f, s := &d.F, &d.c.SSH
	add := func(name string, fn func()) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			cl := f.Clicked("add:" + name)
			if cl.Clicked(gtx) && !f.Disabled {
				fn()
			}
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: name, Icon: fonts.IconPlus, Disabled: f.Disabled}.Layout(gtx, th, cl)
			})
		}
	}
	entry := func(p *string, ph string) form.Grow {
		return form.Grow{W: f.Entry(p, form.EntryOpts{Placeholder: ph, Width: -1})}
	}
	spin := func(p *int) form.Grow {
		return form.Grow{W: f.Spin(p, 1, 65535, 1, false)}
	}
	fwdRows := func(list *[]config.LocalForward, a, b, cc, dd string) layout.Widget {
		rows := []layout.Widget{headerRow(th, a, b, cc, dd, "")}
		for i := range *list {
			i := i
			fw := &(*list)[i]
			rows = append(rows, form.FlexRow(6, entry(&fw.BindAddress, "127.0.0.1"), spin(&fw.Port), entry(&fw.Host, "127.0.0.1"), spin(&fw.HostPort),
				f.DeleteButton(fw, func() { *list = append((*list)[:i:i], (*list)[i+1:]...) })))
		}
		return form.Col(rows...)
	}
	switch d.fwd.Selected {
	case 0:
		return form.Col(fwdRows(&s.LocalForwards, "Local IP", "Local Port", "Remote IP / Host", "Remote Port"),
			add("Add Local Forward", func() {
				s.LocalForwards = append(s.LocalForwards, config.LocalForward{BindAddress: "127.0.0.1", Port: 8080, Host: "127.0.0.1", HostPort: 80})
			}))
	case 1:
		return form.Col(fwdRows(&s.RemoteForwards, "Remote IP", "Remote Port", "Target IP / Host", "Target Port"),
			add("Add Remote Forward", func() {
				s.RemoteForwards = append(s.RemoteForwards, config.LocalForward{BindAddress: "127.0.0.1", Port: 9090, Host: "127.0.0.1", HostPort: 80})
			}))
	case 2:
		rows := []layout.Widget{headerRow(th, "Bind Address", "Port", "")}
		for i := range s.DynamicForwards {
			i := i
			fw := &s.DynamicForwards[i]
			rows = append(rows, form.FlexRow(6, entry(&fw.BindAddress, "127.0.0.1"), spin(&fw.Port),
				f.DeleteButton(fw, func() { s.DynamicForwards = append(s.DynamicForwards[:i:i], s.DynamicForwards[i+1:]...) })))
		}
		return form.Col(append(rows, add("Add Dynamic Forward", func() {
			s.DynamicForwards = append(s.DynamicForwards, config.DynamicForward{BindAddress: "127.0.0.1", Port: 1080})
		}))...)
	case 3:
		var opts []form.Option
		for _, o := range supportedSSHOptions {
			opts = append(opts, form.Option{Value: o, Label: o})
		}
		rows := []layout.Widget{headerRow(th, "ssh_config Option", "Value", "")}
		for i := range d.adv {
			i := i
			o := &d.adv[i]
			rows = append(rows, form.FlexRow(6, form.Grow{W: f.Combo(&o.Key, opts, false)}, entry(&o.Val, "value"),
				f.DeleteButton(o, func() { d.adv = append(d.adv[:i:i], d.adv[i+1:]...) })))
		}
		return form.Col(append(rows, add("Add Option", func() { d.adv = append(d.adv, kv{Key: supportedSSHOptions[0]}) }))...)
	}
	return nil
}

var _ = unit.Dp(0)
