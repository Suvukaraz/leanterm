// Package prefs is the Settings window
// (frontend/src/components/prefs/PreferencesWindow.svelte and its pages): a
// copy of the settings edited in a notebook with the pages on the left,
// saved with "Save and Close".
package prefs

import (
	"encoding/json"
	"fmt"
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/form"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

// Service is what the window needs of services.ConfigService.
type Service interface {
	GetSettings() services.Settings
	SaveSettings(services.Settings) error
	ResetSettings() error
	ExportFull(path string, debug, includeSecrets bool, passphrase string) error
	GetVaultStatus() string
	SetMasterPassword(oldPwd, newPwd string) error
}

// Dialog is the Settings window; Open shows it, Layout draws it until it is
// closed.
type Dialog struct {
	Config Service
	// ChooseSaveFile asks where to export the configuration (it may block;
	// it is called in a goroutine).
	ChooseSaveFile func(title, name string) (string, error)
	// OpenFolder opens a folder in the file manager; LogsDir and
	// ScreenshotsDir are the default folders (empty settings).
	OpenFolder              func(dir string)
	LogsDir, ScreenshotsDir func() string
	// Saved is called after a successful save with the settings before and
	// after (the GUI applies them).
	Saved func(before, after services.Settings)
	// Invalidate asks for a frame (after work in a goroutine).
	Invalidate func()

	F form.Form

	open   bool
	s      services.Settings
	before services.Settings
	vault  string

	dlg   gtk.Dialog
	pages gtk.Notebook
	sub   [2]gtk.Notebook // the Main and Terminal options tabs
	lists [16]widget.List

	resetB, exportB, closeB, saveB widget.Clickable

	msg     gtk.MessageDialog
	msgOpen bool
	pending chan string // messages from goroutines

	pw       pwDialog
	keyHover []gtk.Hover
	keyWinW  int
}

// Page ids (PreferencesWindow.svelte's preferencePages).
var pageIDs = []string{"main", "terminal", "local_shell", "editor", "network", "variables", "local_commands", "remote_commands", "keepass", "keybindings"}

var pageTabs = []struct {
	title string
	icon  rune
}{
	{"Main Options", fonts.IconCog},
	{"Terminal Options", fonts.IconTerminal},
	{"Local Shell Options", fonts.IconCogs},
	{"Editor", fonts.IconFile},
	{"Network Settings", fonts.IconPowerOff},
	{"Global Variables", fonts.IconEdit},
	{"Local Commands", fonts.IconCogs},
	{"Remote Commands", fonts.IconSitemap},
	{"KeePass Integration", fonts.IconLock},
	{"Keybindings", fonts.IconSliders},
}

// Open shows the window on a page ("" = the last one) with a copy of the
// current settings.
func (d *Dialog) Open(page string) {
	d.s = clone(d.Config.GetSettings())
	d.before = clone(d.s)
	d.vault = d.Config.GetVaultStatus()
	d.F.Reset()
	if len(d.pages.Tabs) == 0 {
		for _, t := range pageTabs {
			d.pages.Tabs = append(d.pages.Tabs, &gtk.Tab{Title: t.title, Icon: t.icon})
		}
	}
	for i, id := range pageIDs {
		if id == page {
			d.pages.Selected = i
		}
	}
	d.ensureDefaults()
	d.open = true
}

// Opened reports whether the window is shown.
func (d *Dialog) Opened() bool { return d.open }

// Settings is the edited copy (for tests).
func (d *Dialog) Settings() *services.Settings { return &d.s }

func clone(s services.Settings) services.Settings {
	b, _ := json.Marshal(s)
	var out services.Settings
	if json.Unmarshal(b, &out) != nil {
		return s
	}
	return out
}

var defaultPalette = []string{
	"#2e3436", "#cc0000", "#4e9a06", "#c4a000", "#3465a4", "#75507b", "#06989a", "#d3d7cf",
	"#555753", "#ef2929", "#8ae234", "#fce94f", "#729fcf", "#ad7fa8", "#34e2e2", "#eeeeec",
}

// ensureDefaults fills what the pages expect (TerminalOptionsPage's palette).
func (d *Dialog) ensureDefaults() {
	if l := &d.s.Terminal.Look; len(l.Palette) < 16 {
		l.Palette = append([]string(nil), defaultPalette...)
	}
}

// restartRelevantChanged reports whether an option marked (*) changed.
func restartRelevantChanged(a, b services.Settings) bool {
	pick := func(x services.Settings) string {
		return fmt.Sprint(x.General.Layout, x.General.Theme, x.Appearance.TreePosition, x.Appearance.TabsInMainWindow,
			x.Appearance.ProtectedColor, x.Appearance.ProtectedColorTarget, x.Appearance.UnprotectedColor,
			x.Appearance.UnprotectedColorTarget, x.Terminal.Look.Transparency)
	}
	return pick(a) != pick(b)
}

func (d *Dialog) message(s string) {
	d.msg = gtk.MessageDialog{Type: gtk.MessageInfo, Title: "Settings", Secondary: s, Buttons: gtk.ButtonsOK}
	d.msgOpen = true
}

func (d *Dialog) post(s string) {
	if d.pending == nil {
		return
	}
	select {
	case d.pending <- s:
	default:
	}
	if d.Invalidate != nil {
		d.Invalidate()
	}
}

func (d *Dialog) save() {
	before := d.before
	if err := d.Config.SaveSettings(d.s); err != nil {
		d.message("The settings could not be saved:\n\n" + err.Error())
		return
	}
	d.open = false
	if d.Saved != nil {
		d.Saved(before, d.s)
	}
	if restartRelevantChanged(before, d.s) {
		d.message("Settings saved. Options marked with (*) take effect after restarting LeanTerm.")
	}
}

func (d *Dialog) reset() {
	if err := d.Config.ResetSettings(); err != nil {
		d.message("Reset failed: " + err.Error())
		return
	}
	d.s = clone(d.Config.GetSettings())
	d.F.Reset()
	d.ensureDefaults()
}

func (d *Dialog) export() {
	if d.ChooseSaveFile == nil {
		return
	}
	go func() {
		path, err := d.ChooseSaveFile("Export LeanTerm configuration", "leanterm-export.toml")
		switch {
		case err != nil:
			d.post("Export failed: " + err.Error())
		case path == "":
		default:
			if err := d.Config.ExportFull(path, false, false, ""); err != nil {
				d.post("Export failed: " + err.Error())
			} else {
				d.post("Configuration exported to\n" + path)
			}
		}
	}()
}

// Layout draws the window (and its message and password dialogs) over the
// main window.
func (d *Dialog) Layout(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	if d.pending == nil {
		d.pending = make(chan string, 4)
	}
	select {
	case s := <-d.pending:
		d.message(s)
	default:
	}
	if d.open {
		d.layoutWindow(gtx, th)
	}
	if d.pw.open {
		d.pw.layout(gtx, th, d)
	}
	if d.msgOpen {
		d.msg.Layout(gtx, th)
		if _, ok := d.msg.Response(); ok {
			d.msgOpen = false
		}
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// Active reports whether the window or one of its dialogs is shown.
func (d *Dialog) Active() bool { return d.open || d.pw.open || d.msgOpen }

func (d *Dialog) layoutWindow(gtx layout.Context, th *gtk.Theme) {
	d.F.Th = th
	d.dlg.Title, d.dlg.Banner, d.dlg.Width, d.dlg.Height, d.dlg.Closable = "Settings", "Settings", 880, 660, true
	if d.closeB.Clicked(gtx) || (d.dlg.CloseRequested() && !d.F.PickerOpen()) {
		d.open = false
		return
	}
	if d.saveB.Clicked(gtx) {
		d.save()
		if !d.open {
			return
		}
	}
	if d.resetB.Clicked(gtx) {
		d.reset()
	}
	if d.exportB.Clicked(gtx) {
		d.export()
	}
	pal := th.Pal
	d.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.pages.LayoutSide(gtx, th) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				gtk.Fill(gtx, pal.BgColor)
				return d.page(gtx, th)
			}),
		)
	}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		btn := func(c *widget.Clickable, label string, icon rune, def bool) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				return gtk.Button{Label: label, Icon: icon, Default: def}.Layout(gtx, th, c)
			}
		}
		return form.FlexRow(8,
			btn(&d.resetB, "Reset to defaults", fonts.IconUndo, false),
			btn(&d.exportB, "Export…", fonts.IconSave, false),
			form.Grow{W: func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
			}},
			btn(&d.closeB, "Close without saving", fonts.IconClose, false),
			btn(&d.saveB, "Save and Close", fonts.IconSave, true),
		)(gtx)
	})
	d.F.Overlays(gtx)
}

// scroll shows a page's content with padding, scrolling when it is long.
func (d *Dialog) scroll(gtx layout.Context, list int, pad unit.Dp, w layout.Widget) layout.Dimensions {
	l := &d.lists[list]
	l.Axis = layout.Vertical
	return l.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(pad).Layout(gtx, w)
	})
}

func (d *Dialog) page(gtx layout.Context, th *gtk.Theme) layout.Dimensions {
	switch pageIDs[d.pages.Selected] {
	case "main":
		return d.mainPage(gtx, th)
	case "terminal":
		return d.terminalPage(gtx, th)
	case "local_shell":
		return d.scroll(gtx, 8, 10, d.localShellPage(th))
	case "editor":
		return d.scroll(gtx, 9, 10, d.editorPage(th))
	case "network":
		return d.scroll(gtx, 10, 10, d.networkPage(th))
	case "variables":
		return d.scroll(gtx, 11, 10, d.variablesPage(th))
	case "local_commands":
		return d.scroll(gtx, 12, 10, d.macros(th, &d.s.Macros.Local, "Global common local commands list", false))
	case "remote_commands":
		return d.scroll(gtx, 13, 10, d.macros(th, &d.s.Macros.Remote, "Global common remote commands list", true))
	case "keepass":
		return d.scroll(gtx, 14, 0, d.keepassPage(th))
	case "keybindings":
		return d.keybindingsPage(gtx, th)
	}
	return layout.Dimensions{}
}

// ---- master password ---------------------------------------------------------

type pwDialog struct {
	open                 bool
	oldPw, newPw, confPw string
	err                  string
	f                    form.Form
	dlg                  gtk.Dialog
	cancel, ok           widget.Clickable
}

func (d *Dialog) openMasterPassword() {
	d.pw = pwDialog{open: true}
}

func (p *pwDialog) submit(d *Dialog) {
	switch {
	case p.newPw != p.confPw:
		p.err = "New passwords do not match!"
	case len(p.newPw) < 4:
		p.err = "Password must be at least 4 characters long."
	default:
		if err := d.Config.SetMasterPassword(p.oldPw, p.newPw); err != nil {
			p.err = "Failed to set password: " + err.Error()
			return
		}
		p.open = false
		d.s.General.ProtectWithPassword = true
		d.vault = d.Config.GetVaultStatus()
	}
}

func (p *pwDialog) layout(gtx layout.Context, th *gtk.Theme, d *Dialog) {
	p.f.Th = th
	p.dlg.Title, p.dlg.Banner, p.dlg.Width, p.dlg.Closable = "Set Master Password", "Set Master Password", 400, true
	if p.cancel.Clicked(gtx) || p.dlg.CloseRequested() {
		p.open = false
		return
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			p.submit(d)
		}
	}
	if p.ok.Clicked(gtx) {
		p.submit(d)
	}
	if !p.open {
		return
	}
	pal := th.Pal
	field := func(label string, v *string, ph string) layout.Widget {
		// <label>: a column, gap 4px, 0.9em.
		return gapCol(4,
			gtk.Text{S: label, Size: gtk.Em(0.9), Color: pal.FgColor}.Widget(th),
			p.f.Entry(v, form.EntryOpts{Placeholder: ph, Password: true, Width: -1}),
		)
	}
	p.dlg.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return gapCol(8,
			form.If(p.err != "", func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return box(gtx, pal.ErrorBg, 3, layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10},
					gtk.Text{S: p.err, Size: gtk.Em(0.85), Color: pal.ErrorFg}.Widget(th))
			}),
			field("Current Master Password (leave empty if not set):", &p.oldPw, "Old password"),
			field("New Master Password:", &p.newPw, "New password"),
			field("Confirm New Master Password:", &p.confPw, "Confirm new password"),
		)(gtx)
	}, func(gtx layout.Context) layout.Dimensions {
		return gtk.Button{Label: "Cancel"}.Layout(gtx, th, &p.cancel)
	}, func(gtx layout.Context) layout.Dimensions {
		return gtk.Button{Label: "Save Password", Default: true}.Layout(gtx, th, &p.ok)
	})
}
