// Command leanterm-gio is the prototype of LeanTerm's Gio GUI (stage 0 of the
// migration away from the web layer): terminal tabs for each compiled-in
// terminal engine with a local shell, a LeanEdit tab on gvcode, and files
// dropped from the desktop open in the editor.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/gui/codeedit"
	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
	"github.com/Suvukaraz/leanterm/internal/gui/termview"
	"github.com/Suvukaraz/leanterm/internal/gui/theme"
)

type terminalTab struct {
	engine string
	local  *termview.Local
	view   *termview.View
	err    error
}

type ui struct {
	w      *app.Window
	th     *theme.Theme
	nb     theme.Notebook
	terms  []*terminalTab
	editor *codeedit.Editor

	drops    chan dnd.Drop
	statusCh chan string
	dropMsg  string
	focus    bool
	selfDrop string
}

func main() {
	file := flag.String("file", "", "file to open in the editor tab")
	selfDrop := flag.String("selftest-drop", "", "self-test: drop this file onto the window once it is shown (Windows)")
	quitAfter := flag.Duration("quit-after", 0, "self-test: quit after this time")
	flag.Parse()

	w := new(app.Window)
	w.Option(app.Title("LeanTerm"), app.Size(unit.Dp(1100), unit.Dp(680)))
	u := &ui{w: w, th: theme.New(), drops: make(chan dnd.Drop, 4), statusCh: make(chan string, 4), focus: true, selfDrop: *selfDrop}
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, func() {
			log.Print("self-test: quitting")
			u.closeTerminals()
			os.Exit(0)
		})
	}

	// One terminal tab per engine compiled in, the default engine first.
	engines := []string{term.Default()}
	for _, n := range term.Available() {
		if n != engines[0] {
			engines = append(engines, n)
		}
	}
	for _, name := range engines {
		t := &terminalTab{engine: name}
		t.local, t.err = termview.StartLocal(name, w.Invalidate)
		if t.err == nil {
			t.view = termview.New(t.local.Engine)
			t.view.OnResize = t.local.Resize
		}
		u.terms = append(u.terms, t)
		u.nb.Tabs = append(u.nb.Tabs, &theme.Tab{Title: "Local shell · " + name, Icon: fonts.IconTerminal, IconColor: theme.Success})
	}

	u.editor = codeedit.New("welcome.go", welcome)
	if *file != "" {
		if ed, err := codeedit.Open(*file); err == nil {
			u.editor = ed
		} else {
			log.Print(err)
		}
	}
	u.nb.Tabs = append(u.nb.Tabs, &theme.Tab{Title: "LeanEdit · " + filepath.Base(u.editor.Path), Icon: fonts.IconFileCode, IconColor: theme.Accent})

	if err := dnd.Enable(0, nil); err != nil {
		u.dropMsg = "File drop: " + err.Error()
	}

	go func() {
		err := u.run()
		u.closeTerminals()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func (u *ui) closeTerminals() {
	for _, t := range u.terms {
		if t.local != nil {
			t.local.Close()
		}
	}
}

func (u *ui) run() error {
	var ops op.Ops
	for {
		switch e := u.w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.ViewEvent:
			u.enableDrop(e)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *ui) onDrop(d dnd.Drop) {
	select {
	case u.drops <- d:
	default:
	}
	u.w.Invalidate()
}

func (u *ui) handleDrops() {
	for {
		select {
		case s := <-u.statusCh:
			u.dropMsg = s
		case d := <-u.drops:
			u.dropMsg = fmt.Sprintf("Dropped at %d,%d: %s", d.X, d.Y, strings.Join(d.Paths, ", "))
			log.Print(u.dropMsg)
			if ed, err := codeedit.Open(d.Paths[0]); err == nil {
				u.editor = ed
				u.nb.Tabs[len(u.nb.Tabs)-1].Title = "LeanEdit · " + filepath.Base(ed.Path)
				u.nb.Selected = len(u.nb.Tabs) - 1
			} else {
				u.dropMsg += " (" + err.Error() + ")"
			}
		default:
			return
		}
	}
}

func (u *ui) layout(gtx layout.Context) {
	u.handleDrops()
	theme.Fill(gtx, theme.Bg)
	prev := u.nb.Selected
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.nb.Layout(gtx, u.th) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if u.nb.Selected < len(u.terms) {
				t := u.terms[u.nb.Selected]
				if t.err != nil {
					theme.Fill(gtx, theme.TerminalBg)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}
				if u.focus || prev != u.nb.Selected {
					t.view.Focus(gtx)
					u.focus = false
				}
				return t.view.Layout(gtx, u.th.Shaper)
			}
			theme.Fill(gtx, theme.Base)
			return u.editor.Layout(gtx, u.th.Shaper)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			icon, c, left, right := u.status()
			return theme.Statusbar(gtx, u.th, icon, c, left, right)
		}),
	)
}

func (u *ui) status() (rune, color.NRGBA, string, []string) {
	if u.nb.Selected < len(u.terms) {
		t := u.terms[u.nb.Selected]
		if t.err != nil {
			return fonts.IconWarning, theme.Error, "Status: " + t.err.Error(), nil
		}
		state, icon, ic := "CONNECTED", fonts.IconPlug, theme.Success
		if t.local.Exited() {
			state, icon, ic = "DISCONNECTED", fonts.IconPowerOff, theme.Error
		}
		left := "Status: " + state
		if s := t.local.Title(); s != "" {
			left += " · " + s
		}
		right := []string{"Engine: " + t.engine}
		if d := t.local.Dir(); d != "" {
			right = append([]string{"Folder: " + d}, right...)
		}
		if u.dropMsg != "" {
			right = append([]string{u.dropMsg}, right...)
		}
		return icon, ic, left, right
	}
	left, right := u.editor.StatusFields()
	if u.dropMsg != "" {
		right = append([]string{u.dropMsg}, right...)
	}
	return 0, color.NRGBA{}, left, right
}

const welcome = `// LeanTerm on Gio: stage 0 prototype.
// Drop a file from the file manager onto the window to open it here.
package monitor

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Host is one entry of the inventory.
type Host struct {
	Name    string
	Address string
	Port    int
}

// Check dials the host and reports the latency.
func Check(ctx context.Context, h Host) (time.Duration, error) {
	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", h.Address, h.Port))
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return time.Since(start), nil
}
`
