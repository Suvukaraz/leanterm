package prefs

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
)

// ---- helpers ---------------------------------------------------------------------

func gapCol(gap unit.Dp, children ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var kids []layout.FlexChild
		for _, c := range children {
			if c == nil {
				continue
			}
			if len(kids) > 0 {
				kids = append(kids, layout.Rigid(layout.Spacer{Height: gap}.Layout))
			}
			c := c
			kids = append(kids, layout.Rigid(c))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
}

// box draws w with an inset on a rounded background.
func box(gtx layout.Context, fill color.NRGBA, radius unit.Dp, in layout.Inset, w layout.Widget) layout.Dimensions {
	return boxB(gtx, fill, color.NRGBA{}, radius, in, w)
}

func boxB(gtx layout.Context, fill, border color.NRGBA, radius unit.Dp, in layout.Inset, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := in.Layout(gtx, w)
	call := m.Stop()
	gtk.Box(gtx, d.Size, radius, fill, border)
	call.Add(gtx.Ops)
	return d
}

// label is a page's .field-label: em of the font size, at least minW wide.
func (d *Dialog) label(s string, em float32, minW unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := gtk.Text{S: s, Size: gtk.Em(em), Color: d.F.Th.Pal.FgColor}.Layout(gtx, d.F.Th)
		dims.Size.X = max(dims.Size.X, gtx.Dp(minW))
		return dims
	}
}

// text is plain text in em (muted for hints, italic for notes).
func (d *Dialog) text(s string, em float32, muted, italic bool) layout.Widget {
	c := d.F.Th.Pal.FgColor
	if muted {
		c = d.F.Th.Pal.MutedFg
	}
	return gtk.Text{S: s, Size: gtk.Em(em), Color: c, Italic: italic}.Widget(d.F.Th)
}

// wrapText is a hint that wraps in the width it gets.
func (d *Dialog) hint(s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return gtk.Text{S: s, Size: gtk.Em(0.85), Color: d.F.Th.Pal.MutedFg}.Layout(gtx, d.F.Th)
	}
}

// capped lays w out at most maxW wide, at the start of the room it gets.
func capped(maxW unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		full := gtx.Constraints.Max.X
		gtx.Constraints.Max.X = min(full, gtx.Dp(maxW))
		gtx.Constraints.Min.X = min(gtx.Constraints.Min.X, gtx.Constraints.Max.X)
		d := w(gtx)
		d.Size.X = max(d.Size.X, min(full, gtx.Constraints.Min.X))
		return d
	}
}

func spaced(w layout.Widget, gap unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return layout.Inset{Left: gap}.Layout(gtx, w) }
}

// seg is a piece of rich text (inline code, bold).
type seg struct {
	s          string
	mono, bold bool
}

func (d *Dialog) rich(em float32, c color.NRGBA, segs ...seg) layout.Widget {
	var kids []layout.Widget
	for _, s := range segs {
		s := s
		w := font.Weight(font.Normal)
		if s.bold {
			w = font.Bold
		}
		t := gtk.Text{S: s.s, Size: gtk.Em(em), Color: c, Weight: w, Mono: s.mono}.Widget(d.F.Th)
		if s.mono {
			pal := d.F.Th.Pal
			kids = append(kids, func(gtx layout.Context) layout.Dimensions {
				return box(gtx, pal.HeaderBg, 3, layout.Inset{Left: 4, Right: 4, Top: 2, Bottom: 2}, t)
			})
			continue
		}
		kids = append(kids, t)
	}
	return form.RowGap(0, kids...)
}

// subNotebook lays out a page with tabs on top (Notebook position="top").
func (d *Dialog) subNotebook(gtx layout.Context, th *gtk.Theme, n *gtk.Notebook, titles []string, body func(gtx layout.Context) layout.Dimensions) layout.Dimensions {
	if len(n.Tabs) != len(titles) {
		n.Tabs = nil
		for _, t := range titles {
			n.Tabs = append(n.Tabs, &gtk.Tab{Title: t})
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return n.Layout(gtx, th) }),
		layout.Flexed(1, body),
	)
}

// ---- Main Options ----------------------------------------------------------------

var (
	layoutOpts        = form.Opts("traditional", "Traditional", "compact", "Compact")
	themeOpts         = form.Opts("default", "default", "asbru-color", "asbru-color", "asbru-dark", "asbru-dark", "system", "system")
	schemeOpts        = form.Opts("dark", "Dark", "light", "Light")
	startSizeOpts     = form.Opts("maximized", "Maximized", "last", "With last size")
	startTreeOpts     = form.Opts("connections", "Connections", "favourites", "Favourites", "history", "History", "clusters", "Clusters")
	leftRightOpts     = form.Opts("left", "Left", "right", "Right")
	topBottomOpts     = form.Opts("top", "Top", "bottom", "Bottom")
	colorTargetOpts   = form.Opts("foreground", "Foreground", "background", "Background")
	forwardFormatOpts = form.Opts("full", "Use full names (Local Forwarding, Remote Forwarding, Dynamic Forwarding)", "short", "Use abreviatures (-L, -R, -D)")
	viewerOpts        = form.Opts("internal", "Internal viewer", "external", "External viewer")
)

const (
	promptCommand  = `(([#%:>~$\] ])(?!\g{-1})){3,4}|(\w[@/]\w|sftp).*?[#%>~$\]]|…`
	promptUser     = `([lL]ogin|[uU]suario|([uU]ser-?)*[nN]ame.*|[uU]ser)\s*:\s*$`
	promptPassword = `([pP]ass|[pP]ass[wW]or[dt](\s+for\s+|\w+@[\w\-\.]+)*|[cC]ontrase.a|Enter passphrase for key '.+')\s*:\s*$`
	promptHostKey  = `^.+ontinue connecting \(([^/]+)\/([^/]+)(?:[^)]+)?\)\?\s*$`
	promptChanged  = `.*ffending .*key in (.+?)\:(\d+).*`
	promptAnyKey   = `.*(any key to continue|tecla para continuar).*`
)

func vaultDisplay(s string) string {
	switch s {
	case "passphrase":
		return "Master password (age/scrypt)"
	case "session-only":
		return "Not stored (this session only)"
	case "locked":
		return "Locked"
	}
	return "System keyring"
}

func (d *Dialog) mainPage(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	return d.subNotebook(gtx, th, &d.sub[0], []string{"Behaviour", "Look and Feel", "Advanced"}, func(gtx layout.Context) layout.Dimensions {
		var w layout.Widget
		switch d.sub[0].Selected {
		case 0:
			w = d.mainBehaviour(th)
		case 1:
			w = d.mainLook(th)
		default:
			w = d.mainAdvanced(th)
		}
		return d.scroll(gtx, d.sub[0].Selected, 12, w)
	})
}

func (d *Dialog) mainBehaviour(th *gtk.Theme) layout.Widget {
	f, g := &d.F, &d.s.General
	lbl := func(s string) layout.Widget { return d.label(s, 0.95, 0) }
	startSize := "last"
	if g.StartMainMaximized {
		startSize = "maximized"
	}
	sizeRadio := func(gtx layout.Context) layout.Dimensions {
		v := startSize
		dims := f.Radio(&v, startSizeOpts, false, false)(gtx)
		if v != startSize {
			g.StartMainMaximized = v == "maximized"
			g.RememberMainSize = v != "maximized"
		}
		return dims
	}
	return gapCol(12,
		form.Grid(10, 6, 160, 260,
			form.GridRow{Label: lbl("Main Window Layout (*):"), Field: f.Combo(&g.Layout, layoutOpts, false)},
			form.GridRow{Label: lbl("Colour scheme:"), Field: f.Combo(&g.ColorScheme, schemeOpts, false)},
			form.GridRow{Label: lbl("Icons Theme (*):"), Field: f.Combo(&g.Theme, themeOpts, false)},
		),
		f.Frame("At start", form.Col(sizeRadio, f.Check(&g.StartAtLogin, "Start at session startup", false))),
		f.Frame("At exit", form.Columns2(
			f.Check(&g.ConfirmExit, "Confirm exit", false),
			f.Check(&g.SaveOnExit, "Auto save on exit", false),
			f.Check(&g.AutoSave, "Automatically save every configuration change", false),
		)),
		form.Col(
			form.Row(f.Check(&g.ProtectWithPassword, "Protect with password", false), f.Button("Set...", 0, false, d.openMasterPassword)),
			spaced(d.rich(0.85, th.Pal.MutedFg, seg{s: "Passwords are stored in: "}, seg{s: vaultDisplay(d.vault), bold: true}), 24),
		),
		form.Col(
			f.Check(&g.ForceSplit50, "Force split tabs to 50%", false),
			f.Check(&g.HideOnConnect, "Hide on connection", false),
			f.Check(&g.AutoAcceptHostKeys, "Auto-accept or delete unknown or offending SSH keys", false),
			f.Check(&g.AppendGroupNameToTitle, "Automatically append group name to title for new connections", false),
			f.Check(&g.SortGroupsFirst, "Sort groups first in the connection tree", false),
		),
		d.text("(*) Requires restart to take effect", 0.85, true, true),
	)
}

func (d *Dialog) mainLook(th *gtk.Theme) layout.Widget {
	f, a, g := &d.F, &d.s.Appearance, &d.s.General
	lbl := func(s string) layout.Widget { return d.label(s, 0.95, 0) }
	useUnprot := a.UnprotectedColor != ""
	unprot := func(gtx layout.Context) layout.Dimensions {
		v := useUnprot
		dims := f.Check(&v, "custom", false)(gtx)
		if v != useUnprot {
			if v {
				a.UnprotectedColor = "#cccccc"
			} else {
				a.UnprotectedColor = ""
			}
		}
		return dims
	}
	format := "short"
	if a.StatusBarForwardsFullNames {
		format = "full"
	}
	formatRadio := func(gtx layout.Context) layout.Dimensions {
		v := format
		dims := f.Radio(&v, forwardFormatOpts, true, !a.StatusBarForwards)(gtx)
		if v != format {
			a.StatusBarForwardsFullNames = v == "full"
		}
		return dims
	}
	return gapCol(12,
		f.Frame("Start on", f.Radio(&a.StartTreeOn, startTreeOpts, false, false)),
		form.Columns2(
			f.Check(&a.ShowScreenshots, "Show screenshots", false),
			f.Check(&a.ShowStatistics, "Show statistics", false),
			f.Check(&a.SelectionToClipboard, "Mouse selection copies content to global clipboard", false),
			f.Check(&g.AllowMultipleInstances, "Allow more than one instance to run at the same time", false),
		),
		form.Row(lbl("Show connections on (*):"), f.Radio(&a.TreePosition, leftRightOpts, false, false)),
		form.Row(lbl("Show main button bar on:"), f.Radio(&a.ButtonBarPosition, topBottomOpts, false, false)),
		form.Columns2(
			f.Check(&a.ShowTreeTooltips, "Show connections tooltips", false),
			f.Check(&a.ShowTreeLines, "Show lines in connections tree", false),
			f.Check(&a.TreeOverlayScrolling, "Enable overlay scrolling for connections tree", false),
			f.Check(&a.ShowTreeTitles, "Show tree titles", false),
		),
		f.Frame("Font for", form.Grid(10, 6, 160, 260,
			form.GridRow{Label: lbl("Info tab:"), Field: f.Font(&a.InfoFont, false, false)},
			form.GridRow{Label: lbl("Connection list:"), Field: f.Font(&a.TreeFont, false, false)},
		)),
		f.Frame("Color for", form.Col(
			form.Wrap(10, lbl("Protected nodes (*):"), f.Radio(&a.ProtectedColorTarget, colorTargetOpts, false, false), f.Color(&a.ProtectedColor, false)),
			form.Wrap(10, lbl("Unprotected nodes (*):"), unprot,
				form.If(useUnprot, f.Radio(&a.UnprotectedColorTarget, colorTargetOpts, false, false)),
				form.If(useUnprot, f.Color(&a.UnprotectedColor, false))),
		)),
		form.Col(
			f.Check(&a.AutoHideConnectionsList, "Hide connections list (on terminal open)", false),
			f.Check(&a.AutoHideButtonBar, "Auto hide main button bar (on terminal open)", false),
			f.Check(&a.ShowTreeOnHover, "Auto show/hide when entering/leaving connections list", false),
			f.Check(&a.TabsInMainWindow, "Put Terminal TABS in Main Window (*)", false),
		),
		f.Frame("Port forwarding", form.Col(
			f.Check(&a.StatusBarForwards, "Show port forwarding information in status bar", false),
			formatRadio,
		)),
	)
}

func (d *Dialog) mainAdvanced(th *gtk.Theme) layout.Widget {
	f, p, a := &d.F, &d.s.Prompts, &d.s.Appearance
	lbl := func(s string) layout.Widget { return d.label(s, 0.95, 0) }
	e := func(v *string, ph string) layout.Widget {
		return f.Entry(v, form.EntryOpts{Placeholder: ph, Width: -1})
	}
	return gapCol(12,
		f.Frame("Prompts (empty = built-in default)", gapCol(6,
			form.Grid(10, 6, 160, 0,
				form.GridRow{Label: lbl("Command prompt:"), Field: e(&p.Command, promptCommand)},
				form.GridRow{Label: lbl("USERNAME prompt:"), Field: e(&p.Username, promptUser)},
				form.GridRow{Label: lbl("PASSWORD prompt:"), Field: e(&p.Password, promptPassword)},
				form.GridRow{Label: lbl("Host-key verification string:"), Field: e(&p.HostKeyVerification, promptHostKey)},
				form.GridRow{Label: lbl("Remote host key changed:"), Field: e(&p.RemoteHostKeyChanged, promptChanged)},
				form.GridRow{Label: lbl("Press any key to continue:"), Field: e(&p.PressAnyKey, promptAnyKey)},
				form.GridRow{Label: lbl("Select-by-word characters:"), Field: e(&d.s.Terminal.Look.WordChars, "-.:_/")},
			),
			d.text("USERNAME/PASSWORD/Press any key prompts are used for Telnet logins.", 0.85, true, true),
		)),
		f.Frame("When opening screenshots, use", form.Col(
			f.Radio(&a.ScreenshotViewer, viewerOpts, false, false),
			form.Row(lbl("External viewer:"), f.Entry(&a.ScreenshotViewerCommand, form.EntryOpts{Placeholder: "xdg-open", Disabled: a.ScreenshotViewer != "external"})),
		)),
	)
}

// ---- Terminal Options ------------------------------------------------------------

var (
	lastTabOpts   = form.Opts("nothing", "Do nothing", "close", "Close", "hide", "Hide")
	openNewInOpts = form.Opts("tab", "Tabs", "window", "Windows")
	macrosBarOpts = form.Opts("never", "Never", "combo", "In combo box", "buttons", "As buttons")
	tabsPosOpts   = form.Opts("top", "Top", "bottom", "Bottom", "left", "Left", "right", "Right")
	cursorOpts    = form.Opts("block", "block", "ibeam", "ibeam", "underline", "underline")
	backspaceOpts = form.Opts("auto", "auto", "ascii-backspace", "ascii-backspace (^H)", "ascii-delete", "ascii-delete (^?)", "delete-sequence", "delete-sequence (ESC [3~)", "tty", "tty")
	encodingOpts  = form.Opts("UTF-8", "UTF-8", "ISO-8859-1", "ISO-8859-1 (Latin 1)", "ISO-8859-15", "ISO-8859-15", "Windows-1252", "Windows-1252", "ASCII", "ASCII")
)

// engines are the terminal engines of the native GUI ("rio" from the web
// GUI's settings means the pure Go engine as well).
var engines = []struct{ value, name, detail string }{
	{"ghostty", "Ghostty", "libghostty, default"},
	{"vt", "VT", "pure Go (x/vt)"},
}

func engineSelected(cur, v string) bool {
	switch v {
	case "vt":
		return cur == "vt" || cur == "rio"
	default:
		return cur != "vt" && cur != "rio"
	}
}

func (d *Dialog) terminalPage(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	return d.subNotebook(gtx, th, &d.sub[1], []string{"Behaviour", "Look and Feel", "Advanced", "File browser"}, func(gtx layout.Context) layout.Dimensions {
		var w layout.Widget
		switch d.sub[1].Selected {
		case 0:
			w = d.termBehaviour(th)
		case 1:
			w = d.termLook(th)
		case 2:
			w = d.termAdvanced(th)
		default:
			w = d.termFiles(th)
		}
		return d.scroll(gtx, 3+d.sub[1].Selected, 10, w)
	})
}

func (d *Dialog) engineChoice(th *gtk.Theme) layout.Widget {
	t := &d.s.Terminal
	var kids []any
	for _, e := range engines {
		e := e
		kids = append(kids, form.Grow{W: func(gtx layout.Context) layout.Dimensions {
			c := d.F.Clicked("engine:" + e.value)
			if c.Clicked(gtx) {
				t.Engine = e.value
			}
			sel := engineSelected(t.Engine, e.value)
			pal := th.Pal
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				m := op.Record(gtx.Ops)
				dims := layout.Inset{Top: 14, Bottom: 14, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(gtk.Text{S: e.name, Size: gtk.Em(1.25), Weight: font.Bold, Color: pal.FgColor}.Widget(th)),
						layout.Rigid(layout.Spacer{Height: 2}.Layout),
						layout.Rigid(gtk.Text{S: e.detail, Size: gtk.Em(0.85), Color: pal.MutedFg}.Widget(th)),
					)
				})
				call := m.Stop()
				border, fill := pal.BorderColor, color.NRGBA{}
				if sel {
					border, fill = pal.Accent, pal.BtnFocusBg
				}
				// border 2px, radius 6px.
				gtk.Box(gtx, dims.Size, 6, border, color.NRGBA{})
				r := image.Rectangle{Max: dims.Size}.Inset(gtx.Dp(2))
				inner := op.Offset(r.Min).Push(gtx.Ops)
				bg := fill
				if bg.A == 0 {
					bg = pal.BgColor
				}
				gtk.Box(gtx, r.Size(), 4, bg, color.NRGBA{})
				inner.Pop()
				call.Add(gtx.Ops)
				return dims
			})
		}})
	}
	return form.FlexRow(10, kids...)
}

func (d *Dialog) termBehaviour(th *gtk.Theme) layout.Widget {
	f, t := &d.F, &d.s.Terminal
	l := &t.SessionLogs
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 140) }
	logsOff := !l.Enabled
	choose := func(title string, p *string, def func() string) func() {
		return func() {
			if f.ChooseFile == nil {
				return
			}
			dir := *p
			if dir == "" && def != nil {
				dir = def()
			}
			choose := f.ChooseFile
			go func() {
				if path, err := choose(title, dir, true); err == nil && path != "" {
					f.Later(func() { *p = path })
				}
			}()
		}
	}
	open := func(p *string, def func() string) func() {
		return func() {
			dir := *p
			if dir == "" && def != nil {
				dir = def()
			}
			if d.OpenFolder != nil {
				d.OpenFolder(dir)
			}
		}
	}
	return gapCol(10,
		f.Frame("Terminal engine", form.Col(d.engineChoice(th), d.hint("Applies to newly opened terminals."))),
		f.Check(&t.CloseOnDisconnect, "Close Terminal on disconnection: close container (TAB or Window) when connection is closed", false),
		f.Check(&t.UnsplitDisconnected, "Auto unsplit disconnected Terminals", false),
		f.Frame("Timeouts (seconds)", form.Row(lbl("Time to connect:"), f.Spin(&t.TimeoutConnect, 0, 3600, 1, false),
			spaced(lbl("Expect Cmd exec:"), 16), f.Spin(&t.TimeoutCommand, 0, 3600, 1, false))),
		f.Frame("Session Logs", form.Col(
			f.Check(&l.Enabled, "Enable session logs", false),
			form.Wrap(10, f.Check(&l.StripControlChars, "Remove CONTROL characters", logsOff), f.Check(&l.Timestamps, "Add timestamp", logsOff)),
			form.Row(d.text("Files to save (0=infinite):", 1, false, false), f.Spin(&l.Keep, 0, 999, 1, logsOff)),
			form.Row(lbl("Save in:"), f.Entry(&l.Folder, form.EntryOpts{Placeholder: "Default"}),
				f.Button("Choose…", 0, false, choose("Session logs folder", &l.Folder, d.LogsDir)),
				f.Button("Open Folder", 0, false, open(&l.Folder, d.LogsDir))),
			d.hint(`Empty: "session_logs" in the configuration folder. "Save session log…" in the terminal's context menu also starts here.`),
			form.Row(lbl("Logfile name pattern:"), f.Entry(&l.Pattern, form.EntryOpts{Placeholder: "<DATE_Y><DATE_M><DATE_D>_<NAME>.log", Disabled: logsOff})),
		)),
		f.Frame("Screenshots", form.Col(
			form.Row(lbl("Save in:"), f.Entry(&t.ScreenshotsFolder, form.EntryOpts{Placeholder: "Default"}),
				f.Button("Choose…", 0, false, choose("Screenshots folder", &t.ScreenshotsFolder, d.ScreenshotsDir)),
				f.Button("Open Folder", 0, false, open(&t.ScreenshotsFolder, d.ScreenshotsDir))),
			d.hint(`"Take Screenshot" in the terminal's context menu saves a PNG here. Empty: "screenshots" in the configuration folder.`),
		)),
		f.Frame("When closing last TAB", f.Radio(&t.WhenLastTabClosed, lastTabOpts, false, false)),
		f.Frame("Open NEW connections on", form.Row(f.Radio(&t.OpenNewIn, openNewInOpts, false, false),
			spaced(lbl("Width:"), 16), f.Spin(&t.WindowWidth, 200, 4000, 1, false),
			spaced(lbl("Height:"), 12), f.Spin(&t.WindowHeight, 150, 3000, 1, false))),
	)
}

func (d *Dialog) termLook(th *gtk.Theme) layout.Widget {
	f, t := &d.F, &d.s.Terminal
	l := &t.Look
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 140) }
	palette := func(gtx layout.Context) layout.Dimensions {
		// 8 columns, gap 6px; a color button over its index.
		gap := gtx.Dp(6)
		cw := (gtx.Constraints.Max.X - 7*gap) / 8
		y, rowH := 0, 0
		for i := range l.Palette {
			if i == 8 {
				y += rowH + gap
			}
			idx := fmt.Sprint(i)
			if i >= 8 {
				idx = fmt.Sprintf("B%d", i-8)
			}
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(cw, 0), Max: image.Pt(cw, gtx.Constraints.Max.Y)}
			o := op.Offset(image.Pt((i%8)*(cw+gap), y)).Push(gtx.Ops)
			dims := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(g,
				layout.Rigid(f.Color(&l.Palette[i], false)),
				layout.Rigid(layout.Spacer{Height: 2}.Layout),
				layout.Rigid(gtk.Text{S: idx, Size: gtk.Em(0.75), Color: th.Pal.MutedFg}.Widget(th)),
			)
			o.Pop()
			rowH = max(rowH, dims.Size.Y)
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y+rowH)}
	}
	return gapCol(10,
		f.Check(&t.StartMaximized, "Start Terminals maximized: maximize either TABBED or OWN window", false),
		f.Check(&t.ShowStatusBar, "Show status bar on every terminal", false),
		f.Check(&t.ChangeMainTitle, "Change main window title with terminal title", false),
		f.Frame("Show MACROS", form.Col(f.Radio(&t.MacrosBar, macrosBarOpts, false, false),
			f.Check(&t.ShowGlobalMacros, "Show GLOBAL commands in terminals", false))),
		f.Frame("Tab Label Status Colors", form.Wrap(10,
			lbl("Connected color:"), f.Color(&t.Colors.Connected, false),
			spaced(lbl("Disconnected color:"), 12), f.Color(&t.Colors.Disconnected, false),
			spaced(lbl("New data color:"), 12), f.Color(&t.Colors.NewData, false))),
		f.Frame("Tabs", form.Row(lbl("Tabs position:"), f.Radio(&t.TabsPosition, tabsPosOpts, false, false))),
		f.Frame("Terminal UI", form.Col(
			form.Wrap(10, lbl("Text color:"), f.Color(&l.Foreground, false),
				spaced(lbl("Back color:"), 12), f.Color(&l.Background, false),
				spaced(lbl("Bold Color:"), 12), f.Color(&l.Bold, false)),
			form.Wrap(10, d.text("Bold is:", 1, false, false), f.Check(&l.BoldIsBright, "also bright", false), f.Check(&l.BoldLikeText, "same as text", false)),
			form.Row(lbl("Font:"), f.Font(&l.Font, true, false)),
			form.Wrap(10, lbl("Cursor shape:"), f.Combo(&l.CursorShape, cursorOpts, false),
				spaced(lbl("Scrollback lines:"), 16), f.Spin(&l.ScrollbackLines, -1, 100000, 1, false)),
			form.Row(lbl("Transparency (*):"), f.Scale(&l.Transparency, 0, 1, false)),
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6}.Layout(gtx, gapCol(6,
					gtk.Text{S: "16-Color ANSI Palette:", Size: gtk.Em(0.9), Weight: font.Bold, Color: th.Pal.FgColor}.Widget(th),
					func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 4}.Layout(gtx, palette)
					},
				))
			},
		)),
	)
}

func (d *Dialog) termAdvanced(th *gtk.Theme) layout.Widget {
	f, t := &d.F, &d.s.Terminal
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 140) }
	return gapCol(10,
		form.Row(lbl("Character encoding:"), f.Combo(&t.Look.Encoding, encodingOpts, false)),
		form.Row(lbl("Backspace:"), f.Combo(&t.Look.Backspace, backspaceOpts, false)),
		f.Check(&t.Debug, "Expect DEBUG", false),
		f.Check(&t.HideConnectionsSubmenu, "Hide Connections submenu", false),
		f.Check(&t.AudibleBell, "Audible Bell", false),
		f.Check(&t.ConfirmChains, "Confirm Chains selection / execution", false),
		f.Check(&t.SkipFirstChainExpect, "Skip first Chain expect", false),
	)
}

func (d *Dialog) termFiles(th *gtk.Theme) layout.Widget {
	f, s := &d.F, &d.s.SFTPBrowser
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 140) }
	return gapCol(10,
		f.Frame("Behaviour", form.Col(
			f.Check(&s.AutoOpen, "Open file browser automatically (SSH and local shells)", false),
			f.Check(&s.FollowTerminalFolder, "Follow terminal folder (OSC 7)", false),
			f.Check(&s.ShowHidden, "Show hidden files", false),
			f.Check(&s.ConfirmDelete, "Confirm before deleting", false),
		)),
		f.Frame("Transfers", form.Col(
			f.Check(&s.PreserveTimestamps, "Preserve timestamps", false),
			f.Check(&s.PreservePermissions, "Preserve permissions", false),
			f.Check(&s.UsePartFiles, "Write to temporary .part files and rename when complete", false),
			form.Row(lbl("Parallel transfers:"), f.Spin(&s.ParallelTransfers, 1, 8, 1, false)),
			form.Row(lbl("Default download folder:"), f.File(&s.DownloadDir, "Choose Default Download Folder", true, false),
				f.Entry(&s.DownloadDir, form.EntryOpts{Placeholder: "(empty = system Downloads folder)"})),
		)),
		f.Frame("Look and Feel", form.Col(
			form.Row(lbl("Panel width:"), f.Spin(&s.Width, 200, 1200, 10, false), d.text("px", 1, false, false)),
			form.Row(lbl("Show panel on:"), f.Radio(&s.Position, leftRightOpts, false, false)),
		)),
	)
}

// ---- Local Shell, Editor, Network -------------------------------------------------

func (d *Dialog) localShellPage(th *gtk.Theme) layout.Widget {
	f, sh := &d.F, &d.s.LocalShell
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 130) }
	use := sh.Look != nil
	personal := func(gtx layout.Context) layout.Dimensions {
		v := use
		dims := f.Check(&v, "Use these personal options (otherwise the global Terminal UI settings apply)", false)(gtx)
		if v != use {
			if v {
				l := d.s.Terminal.Look
				l.Palette = append([]string(nil), l.Palette...)
				sh.Look = &l
			} else {
				sh.Look = nil
			}
		}
		return dims
	}
	rows := []layout.Widget{
		f.Frame("Local Shell Launch Settings", gapCol(8,
			f.Check(&sh.Autostart, "Start a shell upon startup", false),
			form.Row(lbl("Shell Binary:"), f.Entry(&sh.Binary, form.EntryOpts{Placeholder: "(empty = $SHELL)"})),
			form.Row(lbl("Shell Options:"), f.Entry(&sh.Options, form.EntryOpts{Placeholder: "e.g. -l"})),
			form.Row(lbl("Initial Directory:"), f.Entry(&sh.Directory, form.EntryOpts{Placeholder: "(empty = $HOME)"})),
		)),
		personal,
	}
	if l := sh.Look; l != nil {
		rows = append(rows, f.Frame("Terminal Look Override", gapCol(8,
			form.Wrap(8, lbl("Text color:"), f.Color(&l.Foreground, false),
				spaced(lbl("Back color:"), 12), f.Color(&l.Background, false),
				spaced(lbl("Bold Color:"), 12), f.Color(&l.Bold, false)),
			form.Row(lbl("Font:"), f.Font(&l.Font, true, false)),
			form.Wrap(8, lbl("Cursor shape:"), f.Combo(&l.CursorShape, cursorOpts, false),
				spaced(lbl("Scrollback lines:"), 16), f.Spin(&l.ScrollbackLines, -1, 100000, 1, false)),
		)))
	}
	return gapCol(10, rows...)
}

var openInOpts = form.Opts("editor", "LeanEdit", "default_app", "Default application")

func (d *Dialog) editorPage(th *gtk.Theme) layout.Widget {
	f, e := &d.F, &d.s.Editor
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 130) }
	return gapCol(10,
		f.Frame("Font", form.Row(lbl("Editor font:"), f.Font(&e.Font, true, false))),
		// LeanEdit on its own has no LeanTerm file browsers to configure.
		form.If(!d.standalone, f.Frame("Opening files", gapCol(8,
			form.Row(lbl("Double-click on a text file opens it in:"), f.Radio(&e.OpenTextFilesIn, openInOpts, false, false)),
			d.hint(`Applies to the SFTP and the local file browser. "Open in Editor" and "Open with default application" stay available in the context menu.`),
		))),
		f.Frame("Windows", gapCol(8,
			f.Check(&e.SingleInstance, "Open files in the running LeanEdit as new tabs (single instance)", false),
			d.hint(`For LeanEdit started on its own (desktop, "Open with"). Off: every file opens in a new window. File → New Window always opens one. Applies to the next start; the editor in LeanTerm is not affected.`),
		)),
		f.Frame("Editing", gapCol(8,
			form.Row(lbl("Tab width:"), f.Spin(&e.TabWidth, 1, 16, 1, false)),
			f.Check(&e.IndentWithTabs, "Indent with tabs instead of spaces", false),
			f.Check(&e.WordWrap, "Wrap long lines", false),
			f.Check(&e.ShowWhitespace, "Show spaces and tabs", false),
			f.Check(&e.ShowMinimap, "Show scrollbar minimap", false),
		)),
		f.Frame("Large files", gapCol(8,
			form.Row(lbl("Ask once above:"), f.Spin(&e.LargeFileMB, 1, 100000, 1, false), d.text("MB", 1, false, false)),
			form.Row(lbl("Ask twice above:"), f.Spin(&e.HugeFileMB, 1, 100000, 1, false), d.text("MB", 1, false, false)),
			d.hint("There is no size limit. Files above the second limit open without syntax highlighting (View → Syntax Highlighting turns it on)."),
		)),
	)
}

var proxyModeOpts = form.Opts("direct", "Do not use proxy (Direct connection)", "socks", "SOCKS proxy", "jump", "Use Jump Server")

func (d *Dialog) networkPage(th *gtk.Theme) layout.Widget {
	f, n := &d.F, &d.s.Network
	lbl := func(s string) layout.Widget { return d.label(s, 0.9, 120) }
	rows := []layout.Widget{f.Frame("Global Network Settings", f.Radio(&n.Mode, proxyModeOpts, true, false))}
	switch n.Mode {
	case "socks":
		s := &n.Socks
		rows = append(rows, f.Frame("Global SOCKS Proxy", gapCol(8,
			form.Row(lbl("IP / Host:"), f.Entry(&s.Host, form.EntryOpts{Placeholder: "Proxy host"}),
				spaced(lbl("Port:"), 12), f.Spin(&s.Port, 1, 65535, 1, false)),
			form.Row(lbl("User:"), f.Entry(&s.User, form.EntryOpts{Placeholder: "Proxy username"})),
		)))
	case "jump":
		j := &n.Jump
		rows = append(rows, f.Frame("Global Jump Server", gapCol(8,
			form.Row(lbl("IP / Host:"), f.Entry(&j.Host, form.EntryOpts{Placeholder: "Jump server host"}),
				spaced(lbl("Port:"), 12), f.Spin(&j.Port, 1, 65535, 1, false)),
			form.Row(lbl("User:"), f.Entry(&j.User, form.EntryOpts{Placeholder: "Jump server username"})),
			form.Row(lbl("Private key file:"), f.File(&j.KeyFile, "Select Jump Server Private Key", false, false),
				f.Button("Clear", 0, false, func() { j.KeyFile = "" })),
		)))
	}
	return gapCol(10, rows...)
}

// ---- Global Variables, commands ----------------------------------------------------

func (d *Dialog) note(th *gtk.Theme, segs ...seg) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return box(gtx, th.Pal.RowAlt, 4, layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.W.Layout(gtx, d.rich(0.9, th.Pal.MutedFg, segs...))
		})
	}
}

func (d *Dialog) variablesPage(th *gtk.Theme) layout.Widget {
	f := &d.F
	lbl := func(s string) layout.Widget { return d.label(s, 0.85, 0) }
	return gapCol(8,
		d.note(th, seg{s: "Global variables can be referenced anywhere using "}, seg{s: "<GV:variable_name>", mono: true}, seg{s: "."}),
		form.List(f, &d.s.Variables, "Global Variables", "Add Global Variable", func() config.VariableConfig { return config.VariableConfig{} },
			func(i int, it *config.VariableConfig, ctl form.RowCtl) layout.Widget {
				return form.FlexRow(6,
					lbl("Variable:"),
					// The wrappers grow (flex 1 and 2), the entries keep their width.
					form.Grow{W: capped(178, f.Entry(&it.Name, form.EntryOpts{Placeholder: "Name", Width: -1})), Weight: 1},
					spaced(lbl("Value:"), 8),
					form.Grow{W: capped(178, f.Entry(&it.Value, form.EntryOpts{Placeholder: "Value", Width: -1, Password: it.Hidden})), Weight: 2},
					f.Check(&it.Hidden, "Hide", false),
					f.DeleteButton(it, ctl.Delete),
				)
			}),
	)
}

// macros is MacrosTab.svelte (as in the Edit Connection dialog).
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

// ---- KeePass ----------------------------------------------------------------------

func (d *Dialog) keepassPage(th *gtk.Theme) layout.Widget {
	f, k := &d.F, &d.s.KeePass
	pal := th.Pal
	row := func(label string, w layout.Widget) layout.Widget {
		return form.FlexRow(12,
			func(gtx layout.Context) layout.Dimensions {
				dims := gtk.Text{S: label, Size: gtk.Em(0.9), Color: pal.FgColor}.Layout(gtx, th)
				dims.Size.X = gtx.Dp(140)
				return dims
			},
			form.Grow{W: w})
	}
	// 11px text: 0.825em of 13.33px.
	const small = 0.825
	note := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return boxB(gtx, pal.HeaderBg, pal.BorderColor, 4, layout.UniformInset(10), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return gapCol(11,
					gtk.Text{S: "Usage in Connections & Macros:", Size: gtk.Em(small), Weight: font.Bold, Color: pal.FgColor}.Widget(th),
					d.rich(small, pal.FgColor, seg{s: "Use the mask "}, seg{s: "<field|entry>", mono: true}, seg{s: " in host, user, password, macros, or expect strings."}),
					d.rich(small, pal.FgColor, seg{s: "Example: "}, seg{s: "<Password|Root/Servers/web1>", mono: true}, seg{s: " or "}, seg{s: "<UserName|Root/Servers/web1>", mono: true}, seg{s: "."}),
					d.rich(small, pal.FgColor, seg{s: "Requires "}, seg{s: "keepassxc-cli", mono: true}, seg{s: " installed in your system PATH."}),
				)(gtx)
			})
		})
	}
	return f.Frame("KeePassXC Integration", func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(6).Layout(gtx, gapCol(10,
			f.Check(&k.Enabled, "Enable KeePassXC integration", false),
			row("Database (.kdbx):", capped(178, f.Entry(&k.Database, form.EntryOpts{Placeholder: "/path/to/database.kdbx", Width: -1}))),
			row("Key File (optional):", capped(178, f.Entry(&k.KeyFile, form.EntryOpts{Placeholder: "/path/to/database.key", Width: -1}))),
			note,
		))
	})
}
