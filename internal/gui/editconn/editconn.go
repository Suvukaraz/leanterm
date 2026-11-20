// Package editconn is the Edit Connection dialog
// (frontend/src/components/edit/EditConnectionWindow.svelte and its tabs).
package editconn

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Dialog edits one connection; Open loads it, Layout draws it until it is
// closed or saved.
type Dialog struct {
	Config *services.ConfigService
	// GlobalLook is [terminal.look] (the personal look starts from it).
	GlobalLook func() config.TerminalLookConfig
	ReadOnly   bool
	// OpenFolder opens a folder in the file manager (session logs).
	OpenFolder func(string)
	// Saved is called after a successful save (the tree reloads).
	Saved func()

	F    form.Form
	open bool
	c    services.ConnectionEdit

	dlg     gtk.Dialog
	outer   gtk.Notebook
	inner   gtk.Notebook
	fwd     gtk.Notebook
	pages   widget.List
	errMsg  string
	errX    widget.Clickable
	closeB  widget.Clickable
	saveB   widget.Clickable
	cmdMsg  gtk.MessageDialog
	cmdOpen bool

	showPw, showPass, showSocks, showJump bool
	adv                                   []kv
	advFor                                string
}

type kv struct{ Key, Val string }

var outerTabs = []string{"Connection", "Options", "Pre Exec", "Post Exec", "Expect", "Remote Macros", "Local Macros", "User Variables", "Terminal Options"}
var innerTabs = []string{"Host", "Authentication", "Connection Details", "Session Logs", "Advanced Parameters", "Network Settings"}

// Open loads connection id for editing (a blank one when it is new).
func (d *Dialog) Open(id string) {
	c, err := d.Config.GetConnectionForEdit(id)
	if err != nil {
		c = services.ConnectionEdit{Connection: config.Connection{ID: id, Name: "New Connection", Title: "New Connection"}}
	}
	ensureDefaults(&c.Connection)
	d.c = c
	d.F.Reset()
	d.open, d.errMsg = true, ""
	d.dlg = gtk.Dialog{Title: "Edit Connection", Banner: "Edit Connection", Width: 760, Height: 620, Closable: true}
	d.outer = gtk.Notebook{}
	for _, t := range outerTabs {
		d.outer.Tabs = append(d.outer.Tabs, &gtk.Tab{Title: t})
	}
	d.inner = gtk.Notebook{}
	for _, t := range innerTabs {
		d.inner.Tabs = append(d.inner.Tabs, &gtk.Tab{Title: t})
	}
	d.fwd = gtk.Notebook{}
	d.adv, d.advFor = nil, ""
}

// Opened reports whether the dialog is shown.
func (d *Dialog) Opened() bool { return d.open }

func (d *Dialog) protected() bool { return d.ReadOnly || d.c.Protected }

// ensureDefaults fills what ensureDefaults() in the Svelte dialog fills.
func ensureDefaults(c *config.Connection) {
	if c.Name == "" {
		c.Name = "New Connection"
	}
	if c.Method == "" {
		c.Method = "ssh"
	}
	if c.Port == 0 {
		c.Port = 22
		if c.Method == "telnet" {
			c.Port = 23
		}
	}
	def := func(s *string, v string) {
		if *s == "" {
			*s = v
		}
	}
	defInt := func(n *int, v int) {
		if *n == 0 {
			*n = v
		}
	}
	def(&c.SFTPBrowser, "global")
	def(&c.Auth.Type, "password")
	def(&c.Network.Mode, "global")
	defInt(&c.Network.Socks.Port, 1080)
	defInt(&c.Network.Jump.Port, 22)
	def(&c.SSH.IPVersion, "any")
	def(&c.Telnet.IPVersion, "any")
	def(&c.Telnet.EscapeChar, "^]")
	def(&c.Telnet.TerminalType, "XTERM-256COLOR")
	def(&c.Telnet.LineEnding, "crlf")
	defInt(&c.SendString.Interval, 60)
	def(&c.Terminal.OpenIn, "tab")
	defInt(&c.Terminal.WindowWidth, 800)
	defInt(&c.Terminal.WindowHeight, 600)
	for len(c.Variables) < 10 {
		c.Variables = append(c.Variables, config.UserVariable{ID: fmt.Sprintf("v%d", len(c.Variables))})
	}
}

// Validate checks the connection like the Svelte dialog's validate().
func Validate(c *config.Connection) string {
	if strings.TrimSpace(c.Host) == "" {
		return "Missing IP/Hostname: Host cannot be empty."
	}
	if c.Port < 1 || c.Port > 65535 {
		return "Invalid Port: Port must be between 1 and 65535."
	}
	if c.Auth.Type == "password" && strings.TrimSpace(c.Auth.User) == "" {
		return "Missing Username: User is required for User / Password authentication."
	}
	seen := map[string]bool{}
	for _, f := range c.SSH.LocalForwards {
		bind := f.BindAddress
		if bind == "" {
			bind = "127.0.0.1"
		}
		k := fmt.Sprintf("%s:%d", bind, f.Port)
		if seen[k] {
			return fmt.Sprintf("CONFIG ERROR: Local Port %d on Local IP '%s' defined in Local Port Forwarding is already in use!", f.Port, bind)
		}
		seen[k] = true
	}
	ports := map[int]bool{}
	for _, f := range c.SSH.RemoteForwards {
		if ports[f.Port] {
			return fmt.Sprintf("WARNING: Same Remote Port %d defined more than once!", f.Port)
		}
		ports[f.Port] = true
	}
	return ""
}

// CommandLine is the equivalent command of showCommandLine().
func CommandLine(c *config.Connection) string {
	host := c.Host
	if host == "" {
		host = "localhost"
	}
	user := ""
	if c.Auth.User != "" {
		user = c.Auth.User + "@"
	}
	ip := func(v string) string {
		switch v {
		case "4":
			return "-4 "
		case "6":
			return "-6 "
		}
		return ""
	}
	switch c.Method {
	case "telnet":
		bind, esc := "", ""
		if c.Telnet.BindAddress != "" {
			bind = "-b " + c.Telnet.BindAddress + " "
		}
		if c.Telnet.EscapeChar != "" {
			esc = "-e '" + c.Telnet.EscapeChar + "' "
		}
		return fmt.Sprintf("telnet %s%s%s%s %d", ip(c.Telnet.IPVersion), bind, esc, host, c.Port)
	case "sftp":
		return fmt.Sprintf("sftp -P %d %s%s", c.Port, user, host)
	}
	return fmt.Sprintf("ssh %s-p %d %s%s", ip(c.SSH.IPVersion), c.Port, user, host)
}

func (d *Dialog) save() {
	if d.protected() {
		return
	}
	d.syncAdv()
	if msg := Validate(&d.c.Connection); msg != "" {
		d.errMsg = msg
		return
	}
	// The ten variable rows: trailing empty ones are not stored.
	vars := d.c.Variables
	for len(vars) > 0 && vars[len(vars)-1].Value == "" && !vars[len(vars)-1].Hidden {
		vars = vars[:len(vars)-1]
	}
	d.c.Variables = vars
	if err := d.Config.SaveConnection(d.c.ID, d.c); err != nil {
		d.errMsg = "Failed to save connection: " + err.Error()
		ensureDefaults(&d.c.Connection)
		return
	}
	d.open = false
	if d.Saved != nil {
		d.Saved()
	}
}

// Layout draws the dialog over the window.
func (d *Dialog) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	if !d.open {
		return layout.Dimensions{}
	}
	d.F.Th = th
	d.F.Disabled = d.protected()
	if d.closeB.Clicked(gtx) || d.dlg.CloseRequested() {
		d.open = false
		return layout.Dimensions{}
	}
	if d.saveB.Clicked(gtx) {
		d.save()
		if !d.open {
			return layout.Dimensions{}
		}
	}
	if d.errX.Clicked(gtx) {
		d.errMsg = ""
	}
	pal := th.Pal
	d.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if d.errMsg == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return banner(gtx, th, d.errMsg, &d.errX)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.outer.Layout(gtx, th) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtk.Fill(gtx, pal.BgColor)
				return d.page(gtx, th)
			}),
		)
	},
		func(gtx layout.Context) layout.Dimensions {
			if !d.protected() {
				return layout.Dimensions{}
			}
			return gtk.Text{S: "\U0001F512 This node is protected against modification", Size: gtk.Em(0.9), Color: pal.WarningFg}.Layout(gtx, th)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Close without saving", Icon: fonts.IconClose}.Layout(gtx, th, &d.closeB)
		},
		func(gtx layout.Context) layout.Dimensions {
			return gtk.Button{Label: "Save and Close", Icon: fonts.IconSave, Default: true, Disabled: d.protected()}.Layout(gtx, th, &d.saveB)
		},
	)
	if d.cmdOpen {
		d.cmdMsg.Layout(gtx, th)
		if _, ok := d.cmdMsg.Response(); ok {
			d.cmdOpen = false
		}
	}
	d.F.Overlays(gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// banner is .validation-error-banner: the error colors, ⚠, the text and ×.
func banner(gtx layout.Context, th *gtk.Theme, msg string, x *widget.Clickable) layout.Dimensions {
	pal := th.Pal
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := layout.Inset{Top: 6, Bottom: 6, Left: 12, Right: 12}
	call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
		return m.Layout(gtx, form.FlexRow(8,
			gtk.Text{S: "⚠", Size: gtk.Em(1.1), Color: pal.ErrorFg}.Widget(th),
			form.Grow{W: gtk.Text{S: msg, Size: gtk.Em(0.9), Color: pal.ErrorFg}.Widget(th)},
			func(gtx layout.Context) layout.Dimensions {
				return x.Layout(gtx, gtk.Text{S: "×", Size: gtk.Em(1.2), Color: pal.ErrorFg}.Widget(th))
			},
		))
	})
	dims.Size.X = gtx.Constraints.Max.X
	r := image.Rectangle{Max: dims.Size}
	gtk.FillRect(gtx, r, pal.ErrorBorder)
	gtk.FillRect(gtx, r.Inset(max(1, gtx.Dp(1))), pal.ErrorBg)
	call.Add(gtx.Ops)
	return dims
}

// scroll shows a tab page with padding 10px, scrolling when it is long.
func (d *Dialog) scroll(gtx layout.Context, w layout.Widget) layout.Dimensions {
	d.pages.Axis = layout.Vertical
	return d.pages.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(10).Layout(gtx, w)
	})
}

func (d *Dialog) hint(th *gtk.Theme, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(32).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.N.Layout(gtx, gtk.Text{S: s, Color: th.Pal.MutedFg}.Widget(th))
		})
	}
}

func (d *Dialog) page(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	c := &d.c
	sftpOnly := c.Method == "sftp"
	switch d.outer.Selected {
	case 0:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.inner.Layout(gtx, th) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return d.scroll(gtx, d.connectionTab(th)) }),
		)
	case 1:
		return d.optionsTab(gtx, th)
	case 2:
		return d.scroll(gtx, d.prePost(th, &c.PreExec, "Pre-connections execute", "Execute commands LOCALLY BEFORE the connection is launched"))
	case 3:
		return d.scroll(gtx, d.prePost(th, &c.PostExec, "Post-connections execute", "Execute commands LOCALLY AFTER the connection has closed"))
	case 4:
		if sftpOnly {
			return d.hint(th, "Expect rules are not available for SFTP browser-only connections.")(gtx)
		}
		return d.scroll(gtx, d.expectTab(th))
	case 5, 6:
		if sftpOnly {
			kind := "Remote"
			if d.outer.Selected == 6 {
				kind = "Local"
			}
			return d.hint(th, kind+" Macros are not available for SFTP browser-only connections.")(gtx)
		}
		if d.outer.Selected == 5 {
			return d.scroll(gtx, d.macros(th, &c.Macros.Remote, "Remote commands list", true))
		}
		return d.scroll(gtx, d.macros(th, &c.Macros.Local, "Local commands list", false))
	case 7:
		return d.scroll(gtx, d.variablesTab(th))
	case 8:
		if sftpOnly {
			return d.hint(th, "Terminal options are not available for SFTP browser-only connections.")(gtx)
		}
		return d.scroll(gtx, d.terminalTab(th))
	}
	return layout.Dimensions{}
}

var (
	methodOptions = form.Opts("ssh", "SSH", "telnet", "Telnet", "sftp", "SFTP (file browser only)")
	authOptions   = form.Opts("password", "User / Password ", "publickey", "Private Key ", "manual", "Manual")
	networkModes  = form.Opts("global", "Use global proxy options", "direct", "Use a direct connection", "socks", "Use a SOCKS proxy", "jump", "Use a Jump Host")
	sftpOptions   = form.Opts("global", "Use global setting", "open", "Open automatically", "closed", "Do not open automatically")
	ipOptions     = form.Opts("any", "any", "4", "4", "6", "6")
	lineEndings   = form.Opts("crlf", "CR LF", "crnul", "CR NUL", "lf", "LF")
)

// spaced is a gap before a label inside a row (margin-left: 12px).
func spaced(w layout.Widget, gap unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: gap}.Layout(gtx, w)
	}
}

func (d *Dialog) connectionTab(th *gtk.Theme) layout.Widget {
	f, c := &d.F, &d.c
	switch d.inner.Selected {
	case 0: // Host
		return form.Section(
			form.Row(f.Label("Method:"), func(gtx layout.Context) layout.Dimensions {
				prev := c.Method
				dims := f.Combo(&c.Method, methodOptions, false)(gtx)
				if c.Method != prev {
					// handleMethodChange: the default port of the method.
					if c.Method == "telnet" && c.Port == 22 {
						c.Port = 23
					} else if c.Method != "telnet" && c.Port == 23 {
						c.Port = 22
					}
					if c.Method == "telnet" && c.Auth.Type == "publickey" {
						c.Auth.Type = "password"
					}
				}
				return dims
			}),
			form.Row(f.Label("Host:"), f.Entry(&c.Host, form.EntryOpts{Placeholder: "IP or Hostname"}),
				spaced(f.Label("Port:"), 12), f.Spin(&c.Port, 1, 65535, 1, false)),
			form.Row(f.Label("TAB / Window Title:"), f.Entry(&c.Title, form.EntryOpts{Placeholder: "TAB/Window title"})),
			form.RowGap(12,
				f.Check(&c.LaunchAtStartup, "Launch at startup", false),
				f.Check(&c.AutoReconnect, "Reconnect on disconnection", false),
				form.RowGap(6, f.Text("Wait", false), f.Spin(&c.SendDelayMs, 0, 5000, 5, false), f.Text("millisecs for automated char sending", false)),
			),
		)
	case 1: // Authentication
		opts := authOptions
		if c.Method == "telnet" {
			opts = []form.Option{authOptions[0], authOptions[2]}
		}
		var detail layout.Widget
		switch c.Auth.Type {
		case "password":
			detail = f.Frame("User / Password", form.Row(
				f.Label("User:"), f.Entry(&c.Auth.User, form.EntryOpts{Placeholder: "Username"}),
				spaced(f.Label("Password:"), 12), f.Entry(&c.Password, form.EntryOpts{Placeholder: "Password", Password: !d.showPw}),
				f.Check(&d.showPw, "Show", false)))
		case "publickey":
			detail = f.Frame("Private Key", form.Col(
				form.Row(f.Label("Private key file:"), f.File(&c.Auth.KeyFile, "Select Private Key File", false, false),
					f.Button("Clear", 0, f.Disabled, func() { c.Auth.KeyFile = "" })),
				form.Row(f.Label("User:"), f.Entry(&c.Auth.KeyUser, form.EntryOpts{Placeholder: "Username for key"}),
					spaced(f.Label("Passphrase:"), 12), f.Entry(&c.KeyPassphrase, form.EntryOpts{Placeholder: "Passphrase", Password: !d.showPass}),
					f.Check(&d.showPass, "Show", false))))
		case "manual":
			detail = f.Hint("Username and password will be requested interactively upon connecting.")
		}
		return form.Section(
			f.Radio(&c.Auth.Type, opts, true, false),
			detail,
			form.If(c.Method != "telnet", func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 10}.Layout(gtx, f.Check(&c.Auth.Exclusive, `Use selected auth method exclusively (no fallback nor "Advanced Options" auth bypass)`, false))
			}),
		)
	case 2: // Connection Details
		off := !c.SendString.Enabled
		return form.Section(f.Frame("Programmed String Sending", form.Col(
			form.Row(f.Check(&c.SendString.Enabled, "Programatically send string", false), f.Entry(&c.SendString.Text, form.EntryOpts{Placeholder: "String to send", Disabled: off})),
			form.Row(f.Check(&c.SendString.AppendReturn, "With <RETURN>", off), spaced(f.Text("Every (secs):", false), 12),
				f.Spin(&c.SendString.Interval, 1, 3600, 1, off), f.Check(&c.SendString.OnlyWhenIdle, "Idle Only", off)),
		)))
	case 3: // Session Logs
		off := !c.SessionLogs.Enabled
		return form.Section(f.Frame("Auto save Session logs", form.Col(
			f.Check(&c.SessionLogs.Enabled, "Enable session logs", false),
			form.Row(f.Check(&c.SessionLogs.StripControlChars, "Remove CONTROL characters", off), f.Check(&c.SessionLogs.Timestamps, "Add timestamp", off)),
			form.Row(f.Text("Files to save (0=infinite):", false), f.Spin(&c.SessionLogs.Keep, 0, 999, 1, off)),
			form.Row(f.Label("Save in:"), f.Entry(&c.SessionLogs.Folder, form.EntryOpts{Placeholder: "Log folder path", Disabled: off}),
				f.Button("Open folder", 0, false, func() {
					if d.OpenFolder != nil {
						d.OpenFolder(c.SessionLogs.Folder)
					}
				})),
			form.Row(f.Label("Logfile name pattern:"), f.Entry(&c.SessionLogs.Pattern, form.EntryOpts{Placeholder: "<DATE_Y><DATE_M><DATE_D>_<NAME>.log", Disabled: off})),
		)))
	case 4: // Advanced Parameters
		id := c.ID
		return form.Section(
			form.Row(f.Label("Connection UUID:"), f.Entry(&id, form.EntryOpts{ReadOnly: true}),
				f.Button("Get Command line", 0, false, func() {
					d.cmdMsg = gtk.MessageDialog{Type: gtk.MessageInfo, Title: "Equivalent Command Line", Primary: "Equivalent terminal command:", Secondary: CommandLine(&c.Connection), Buttons: gtk.ButtonsOK}
					d.cmdOpen = true
				})),
			form.If(c.Method == "ssh", form.Col(
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 4}.Layout(gtx, form.Row(f.Label("SFTP browser:"), f.Combo(&c.SFTPBrowser, sftpOptions, false)))
				},
				form.Row(f.Label("SFTP start folder:"), f.Entry(&c.SFTPStartDir, form.EntryOpts{Placeholder: "(login directory)"})),
			)),
		)
	case 5: // Network Settings
		var detail layout.Widget
		n := &c.Network
		switch n.Mode {
		case "socks":
			detail = f.Frame("SOCKS Proxy Settings", form.Col(
				form.Row(f.Label("IP / Host:"), f.Entry(&n.Socks.Host, form.EntryOpts{Placeholder: "Proxy host"}), spaced(f.Label("Port:"), 12), f.Spin(&n.Socks.Port, 1, 65535, 1, false)),
				form.Row(f.Label("User:"), f.Entry(&n.Socks.User, form.EntryOpts{Placeholder: "Proxy username"}),
					spaced(f.Label("Password:"), 12), f.Entry(&c.SocksPassword, form.EntryOpts{Placeholder: "Proxy password", Password: !d.showSocks}), f.Check(&d.showSocks, "Show", false)),
			))
		case "jump":
			detail = f.Frame("Jump Host Settings", form.Col(
				form.Row(f.Label("IP / Host:"), f.Entry(&n.Jump.Host, form.EntryOpts{Placeholder: "Jump host"}), spaced(f.Label("Port:"), 12), f.Spin(&n.Jump.Port, 1, 65535, 1, false)),
				form.Row(f.Label("User:"), f.Entry(&n.Jump.User, form.EntryOpts{Placeholder: "Jump username"}),
					spaced(f.Label("Password:"), 12), f.Entry(&c.JumpPassword, form.EntryOpts{Placeholder: "Jump password", Password: !d.showJump}), f.Check(&d.showJump, "Show", false)),
				form.Row(f.Label("Private key file:"), f.File(&n.Jump.KeyFile, "Select Jump Host Private Key", false, false), f.Button("Clear", 0, f.Disabled, func() { n.Jump.KeyFile = "" })),
				form.If(c.Method == "ssh", f.Check(&n.PseudoJump, "Pseudo jump server", false)),
			))
		}
		return form.Section(f.Radio(&n.Mode, networkModes, true, false), detail)
	}
	return nil
}

func (d *Dialog) prePost(th *gtk.Theme, items *[]config.PrePostExec, title, info string) layout.Widget {
	f := &d.F
	return form.Section(
		f.Text(info, true),
		form.List(f, items, title, "Add Command", func() config.PrePostExec { return config.PrePostExec{Default: true} },
			func(i int, it *config.PrePostExec, ctl form.RowCtl) layout.Widget {
				return form.FlexRow(6,
					f.Check(&it.Ask, "Ask: ", false),
					f.Check(&it.Default, "Default: ", false),
					form.Grow{W: f.Entry(&it.Command, form.EntryOpts{Placeholder: "Command to execute locally", Width: -1})},
					f.DeleteButton(it, ctl.Delete),
				)
			}),
	)
}

func (d *Dialog) macros(th *gtk.Theme, items *[]config.MacroConfig, title string, remote bool) layout.Widget {
	f := &d.F
	return form.List(f, items, title, "Add Macro", func() config.MacroConfig { return config.MacroConfig{AppendReturn: true} },
		func(i int, it *config.MacroConfig, ctl form.RowCtl) layout.Widget {
			return form.FlexRow(6,
				f.Check(&it.Confirm, "Confirm", false),
				form.Grow{W: f.Entry(&it.Command, form.EntryOpts{Placeholder: "Command", Width: -1})},
				form.Grow{W: f.Entry(&it.Description, form.EntryOpts{Placeholder: "Description", Width: -1})},
				form.If(remote, f.Check(&it.AppendReturn, "Intro", false)),
				f.DeleteButton(it, ctl.Delete),
			)
		})
}

func (d *Dialog) variablesTab(th *gtk.Theme) layout.Widget {
	f, c := &d.F, &d.c
	rows := []layout.Widget{f.Text("Define variables accessible via <V:0> to <V:9> in commands, titles, and expect rules.", true)}
	for i := 0; i < 10 && i < len(c.Variables); i++ {
		v := &c.Variables[i]
		rows = append(rows, form.FlexRow(8,
			gtk.Text{S: fmt.Sprintf("<V:%d>", i), Mono: true, Color: th.Pal.FgColor}.Widget(th),
			form.Grow{W: f.Entry(&v.Value, form.EntryOpts{Placeholder: fmt.Sprintf("Value for <V:%d>", i), Password: v.Hidden, Width: -1})},
			f.Check(&v.Hidden, "Hide", false),
			f.Button(fmt.Sprintf("Clear %d", i), 0, f.Disabled, func() { v.Value = "" }),
		))
	}
	return form.Col(rows...)
}

func (d *Dialog) expectTab(th *gtk.Theme) layout.Widget {
	f, c := &d.F, &d.c
	pal := th.Pal
	items := &c.Expect
	n := len(*items)
	var cards []layout.Widget
	if n == 0 {
		cards = append(cards, f.Hint(`No Expect rules defined. Click "Add Expect Rule" below to automate interactive prompts.`))
	}
	for i := 0; i < n; i++ {
		i := i
		r := &(*items)[i]
		head := form.FlexRow(4,
			form.Grow{W: f.Check(&r.Enabled, fmt.Sprintf("Expect #%d", i+1), false)},
			f.IconButton(r, fonts.IconChevUp, i == 0, func() { (*items)[i-1], (*items)[i] = (*items)[i], (*items)[i-1] }),
			f.IconButton(&r.ID, fonts.IconChevDown, i == n-1, func() { (*items)[i+1], (*items)[i] = (*items)[i], (*items)[i+1] }),
			f.DeleteButton(&r.Pattern, func() { *items = append((*items)[:i:i], (*items)[i+1:]...) }),
		)
		match := form.Opts("next", "Next", "stop", "Stop", "goto", "Goto")
		fail := form.Opts("close", "Close session", "stop", "Stop", "goto", "Goto")
		body := form.Col(
			form.FlexRow(8, f.Label("Expect:"), form.Grow{W: f.Entry(&r.Pattern, form.EntryOpts{Placeholder: "Regular expression pattern", Width: -1})}),
			form.FlexRow(8, f.Label("Send:"), form.Grow{W: f.Entry(&r.Send, form.EntryOpts{Placeholder: "String to send", Password: r.Hidden, Width: -1})},
				f.Check(&r.Hidden, "Hide", false), f.Check(&r.AppendReturn, "Return", false)),
			form.Row(f.Label("If match:"), f.Radio(&r.OnMatch, match, false, false), form.If(r.OnMatch == "goto", f.Spin(&r.OnMatchGoto, 1, n, 1, false))),
			form.Row(f.Label("If fail:"), f.Radio(&r.OnFail, fail, false, false), form.If(r.OnFail == "goto", f.Spin(&r.OnFailGoto, 1, n, 1, false)),
				spaced(f.Text("Timeout (sec):", false), 16), f.Spin(&r.Timeout, 0, 3600, 1, false)),
		)
		cards = append(cards, card(th, pal, head, body))
	}
	add := func(gtx layout.Context) layout.Dimensions {
		c := d.F.Clicked("add-expect")
		if c.Clicked(gtx) && !d.F.Disabled {
			*items = append(*items, config.ExpectEntry{Enabled: true, AppendReturn: true, OnMatch: "next", OnMatchGoto: 1, OnFail: "stop", OnFailGoto: 1, Timeout: 10})
		}
		return gtk.Button{Label: "Add Expect Rule", Icon: fonts.IconPlus, Disabled: d.F.Disabled}.Layout(gtx, th, c)
	}
	return form.Section(append(cards, add)...)
}

// card is .expect-card: bordered, base color, padding 8px 10px, a header
// with a line under it.
func card(th *gtk.Theme, pal *gtk.Palette, head, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		call, dims := record(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dd := layout.Inset{Bottom: 4}.Layout(gtx, head)
						gtk.FillRect(gtx, image.Rect(0, dd.Size.Y-1, gtx.Constraints.Max.X, dd.Size.Y), pal.BorderColor)
						return dd
					}),
					layout.Rigid(layout.Spacer{Height: 6}.Layout),
					layout.Rigid(body),
				)
			})
		})
		dims.Size.X = gtx.Constraints.Max.X
		r := image.Rectangle{Max: dims.Size}
		gtk.FillRect(gtx, r, pal.BorderColor)
		gtk.FillRect(gtx, r.Inset(max(1, gtx.Dp(1))), pal.BaseColor)
		call.Add(gtx.Ops)
		return dims
	}
}

var (
	cursorShapes = form.Opts("block", "block", "ibeam", "ibeam", "underline", "underline")
	backspaces   = form.Opts("auto", "auto", "ascii-backspace", "ascii-backspace (^H)", "ascii-delete", "ascii-delete (^?)", "delete-sequence", "delete-sequence (ESC [3~)", "tty", "tty")
	encodings    = form.Opts("UTF-8", "UTF-8", "ISO-8859-1", "ISO-8859-1 (Latin 1)", "ISO-8859-15", "ISO-8859-15", "Windows-1252", "Windows-1252", "ASCII", "ASCII")
	openIn       = form.Opts("tab", "Tab", "window", "Window")
)

func (d *Dialog) globalLook() *config.TerminalLookConfig {
	l := config.TerminalLookConfig{Foreground: "#ffffff", Background: "#000000", Bold: "#ffffff", BoldLikeText: true, BoldIsBright: true,
		CursorShape: "block", ScrollbackLines: 5000, WordChars: `-A-Za-z0-9_.~:/?#[]@!$&'()*+,;=`, Encoding: "UTF-8", Backspace: "auto",
		Font: config.FontConfig{Family: "Monospace", Size: 10}}
	if d.GlobalLook != nil {
		l = d.GlobalLook()
	}
	l.Palette = append([]string(nil), l.Palette...)
	return &l
}

func (d *Dialog) terminalTab(th *gtk.Theme) layout.Widget {
	f, c := &d.F, &d.c
	t := &c.Terminal
	off := !t.UsePersonal
	use := t.UsePersonal
	usePersonal := func(gtx layout.Context) layout.Dimensions {
		dims := f.Check(&use, "Use these personal options", false)(gtx)
		if use != t.UsePersonal {
			t.UsePersonal = use
			if use && t.Look == nil {
				t.Look = d.globalLook()
			}
		}
		return dims
	}
	reset := f.Button("Reset to DEFAULT values", fonts.IconUndo, f.Disabled || off, func() {
		t.TimeoutConnect, t.TimeoutCommand, t.TabColor, t.OpenIn, t.WindowWidth, t.WindowHeight = 0, 10, "", "tab", 800, 600
		t.Prompts = config.PromptsConfig{}
		t.Look = d.globalLook()
	})
	rows := []layout.Widget{
		form.Row(usePersonal, reset),
		f.Frame("Prompt RegExp", form.Col(
			form.FlexRow(8, f.Label("Command prompt:"), form.Grow{W: f.Entry(&t.Prompts.Command, form.EntryOpts{Placeholder: `Default: (?!\g{-1})[\$#>]\s*$`, Disabled: off, Width: -1})}),
			form.FlexRow(8, f.Label("Username RegExp:"), form.Grow{W: f.Entry(&t.Prompts.Username, form.EntryOpts{Placeholder: `Default: (user(name)?|login)\s*:\s*$`, Disabled: off, Width: -1})}),
			form.FlexRow(8, f.Label("Password RegExp:"), form.Grow{W: f.Entry(&t.Prompts.Password, form.EntryOpts{Placeholder: `Default: (password|passphrase)\s*:\s*$`, Disabled: off, Width: -1})}),
		)),
	}
	if l := t.Look; l != nil {
		rows = append(rows,
			f.Frame("Terminal UI", form.Col(
				form.Row(f.Label("Text color:"), f.Color(&l.Foreground, off), spaced(f.Label("Back color:"), 12), f.Color(&l.Background, off),
					f.Check(&l.BoldLikeText, "Bold color like Text color", off)),
				form.Row(f.Label("Font:"), f.Font(&l.Font, true, off)),
				form.Row(f.Label("Tab background color:"), f.Color(&t.TabColor, off)),
				form.Row(f.Label("Cursor Shape:"), f.Combo(&l.CursorShape, cursorShapes, off), spaced(f.Label("Scrollback lines:"), 12), f.Spin(&l.ScrollbackLines, -1, 100000, 1, off)),
				form.Row(f.Label("Transparency:"), f.Scale(&l.Transparency, 0, 1, off)),
			)),
			f.Frame("Select Word CHARS", f.Entry(&l.WordChars, form.EntryOpts{Placeholder: `-A-Za-z0-9_.~:/?#[]@!$&'()*+,;=`, Disabled: off, Width: -1})),
		)
	} else {
		rows = append(rows, f.Frame("Terminal UI", f.Text("Check \"Use these personal options\" to set a personal look.", true)))
	}
	rows = append(rows,
		f.Frame("Time outs (seconds)", form.Row(f.Label("Connection:"), f.Spin(&t.TimeoutConnect, 0, 3600, 1, off),
			spaced(f.Label("Expect Cmd exec:"), 16), f.Spin(&t.TimeoutCommand, 0, 3600, 1, off))),
		f.Frame("Open NEW connection on", form.Row(f.Radio(&t.OpenIn, openIn, false, off),
			spaced(f.Label("Width:"), 16), f.Spin(&t.WindowWidth, 200, 4000, 1, off),
			spaced(f.Label("Height:"), 12), f.Spin(&t.WindowHeight, 150, 3000, 1, off))),
	)
	if l := t.Look; l != nil {
		rows = append(rows, f.Frame("Terminal Emulation", form.Row(f.Label("Backspace binding:"), f.Combo(&l.Backspace, backspaces, off),
			spaced(f.Label("Character Encoding:"), 16), f.Combo(&l.Encoding, encodings, off))))
	}
	return form.Section(rows...)
}

// syncAdv writes the advanced ssh_config rows into SSH.Options.
func (d *Dialog) syncAdv() {
	if d.advFor != d.c.ID {
		return
	}
	opts := map[string]string{}
	for _, o := range d.adv {
		if k := strings.TrimSpace(o.Key); k != "" {
			opts[k] = o.Val
		}
	}
	d.c.SSH.Options = opts
}
