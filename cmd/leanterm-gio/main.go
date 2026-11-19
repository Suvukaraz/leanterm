// Command leanterm-gio is the prototype of LeanTerm's Gio GUI (stages 0 and 1
// of the migration away from the web layer). It runs LeanTerm's backend
// (internal/gui/appcore): a local shell per compiled-in terminal engine and
// the saved connections given with -connect open as terminal tabs, the
// backend's questions (passwords, host keys) appear as dialogs, a LeanEdit tab
// runs on gvcode, and files dropped from the desktop open in the editor.
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

	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/gui/appcore"
	"github.com/Suvukaraz/leanterm/internal/gui/codeedit"
	"github.com/Suvukaraz/leanterm/internal/gui/dialogs"
	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
	"github.com/Suvukaraz/leanterm/internal/gui/fonts"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
	"github.com/Suvukaraz/leanterm/internal/gui/termview"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// tab is a terminal tab, bound to a session of the backend.
type tab struct {
	sess   *termview.Session
	view   *termview.View
	engine string
	kind   string
	nb     *gtk.Tab
}

type ui struct {
	w        *app.Window
	th       *gtk.Theme
	core     *appcore.Core
	prompter *dialogs.Prompter
	nb       gtk.Notebook
	tabs     []*tab // terminal tabs; the editor tab is last in nb.Tabs
	editor   *codeedit.Editor
	edTab    *gtk.Tab

	// Sessions opened and closed by the backend (any goroutine).
	opened chan session.SessionInfo
	closed chan string
	// engines for the next local shells, in the order they were opened.
	engineQueue []string
	engine      string

	drops    chan dnd.Drop
	statusCh chan string
	dropMsg  string
	focus    bool
	selfDrop string
}

func main() {
	file := flag.String("file", "", "file to open in the editor tab")
	configDir := flag.String("config-dir", "", "configuration folder (default: LeanTerm's)")
	connect := flag.String("connect", "", "comma-separated saved connections (id or name) to open")
	engine := flag.String("engine", "", "terminal engine for connections: ghostty or vt (default: "+term.Default()+")")
	noLocal := flag.Bool("no-local", false, "do not open local shells")
	selfDrop := flag.String("selftest-drop", "", "self-test: drop this file onto the window once it is shown (Windows)")
	quitAfter := flag.Duration("quit-after", 0, "self-test: quit after this time")
	flag.Parse()

	core, err := appcore.Open(appcore.Options{ConfigDir: *configDir})
	if err != nil {
		log.Fatal(err)
	}

	w := new(app.Window)
	w.Option(app.Title("LeanTerm"), app.Size(unit.Dp(1100), unit.Dp(680)))
	u := &ui{
		w: w, th: gtk.New(core.Store.Config().General.ColorScheme), core: core, prompter: dialogs.NewPrompter(w.Invalidate),
		opened: make(chan session.SessionInfo, 16), closed: make(chan string, 16), engine: *engine,
		drops: make(chan dnd.Drop, 4), statusCh: make(chan string, 4), focus: true, selfDrop: *selfDrop,
	}
	core.Sessions.SetPrompter(u.prompter)
	core.Bus.On(events.SessionOpened, func(_ string, data any) {
		m, _ := data.(map[string]any)
		switch info := m["session"].(type) {
		case *session.SessionInfo:
			u.opened <- *info
		case session.SessionInfo:
			u.opened <- info
		}
		w.Invalidate()
	})
	core.Bus.On(events.SessionClosed, func(_ string, data any) {
		if m, ok := data.(map[string]any); ok {
			if id, ok := m["tmpID"].(string); ok {
				u.closed <- id
				w.Invalidate()
			}
		}
	})
	if *quitAfter > 0 {
		time.AfterFunc(*quitAfter, func() {
			log.Print("self-test: quitting")
			u.closeAll()
			os.Exit(0)
		})
	}

	u.editor = codeedit.New("welcome.go", welcome)
	if *file != "" {
		if ed, err := codeedit.Open(*file); err == nil {
			u.editor = ed
		} else {
			log.Print(err)
		}
	}
	u.edTab = &gtk.Tab{Title: "LeanEdit · " + filepath.Base(u.editor.Path), Icon: fonts.IconFileCode, IconColor: u.th.Pal.Accent}
	u.nb.Tabs = []*gtk.Tab{u.edTab}

	if err := dnd.Enable(0, nil); err != nil {
		u.dropMsg = "File drop: " + err.Error()
	}

	// A local shell per engine compiled in (the default engine first), then
	// the connections asked for.
	if !*noLocal {
		engines := []string{term.Default()}
		for _, n := range term.Available() {
			if n != engines[0] {
				engines = append(engines, n)
			}
		}
		for _, name := range engines {
			u.engineQueue = append(u.engineQueue, name)
			if _, err := core.Sessions.OpenLocalShell("tab"); err != nil {
				log.Print(err)
			}
		}
	}
	for _, c := range strings.Split(*connect, ",") {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		conn, ok := core.FindConnection(c)
		if !ok {
			log.Printf("no saved connection %q", c)
			continue
		}
		if _, err := core.Sessions.Open(session.OpenRequest{ConnIDs: []string{conn.ID}, Where: "tab"}); err != nil {
			log.Printf("%s: %v", c, err)
		}
	}

	go func() {
		err := u.run()
		u.closeAll()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func (u *ui) closeAll() {
	for _, t := range u.tabs {
		t.sess.Close()
	}
	u.core.Close()
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

// sessionEvents adds and removes tabs for sessions the backend opened/closed.
func (u *ui) sessionEvents() {
	for {
		select {
		case info := <-u.opened:
			engine, startup := u.engine, false
			if info.Kind == "local" && len(u.engineQueue) > 0 {
				engine, u.engineQueue, startup = u.engineQueue[0], u.engineQueue[1:], true
			}
			s, err := termview.Attach(u.core.Sessions, u.core.Bus, info.TmpID, engine, u.w.Invalidate)
			if err != nil {
				log.Printf("%s: %v", info.Title, err)
				continue
			}
			t := &tab{sess: s, view: termview.New(s.Engine), engine: s.Engine.Name(), kind: info.Kind, nb: &gtk.Tab{Title: info.Title}}
			t.view.OnResize = s.Resize
			u.tabs = append(u.tabs, t)
			// Terminal tabs come before the editor tab. A session opened later
			// is selected; the shells of the start leave the first one selected.
			u.nb.Tabs = append(append(u.nb.Tabs[:len(u.nb.Tabs)-1:len(u.nb.Tabs)-1], t.nb), u.edTab)
			if !startup {
				u.nb.Selected = len(u.tabs) - 1
			}
			u.focus = true
		case id := <-u.closed:
			for i, t := range u.tabs {
				if t.sess.TmpID != id {
					continue
				}
				t.sess.Close()
				u.tabs = append(u.tabs[:i], u.tabs[i+1:]...)
				u.nb.Tabs = append(u.nb.Tabs[:i], u.nb.Tabs[i+1:]...)
				if u.nb.Selected >= len(u.nb.Tabs) {
					u.nb.Selected = len(u.nb.Tabs) - 1
				}
				break
			}
		default:
			return
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
				u.edTab.Title = "LeanEdit · " + filepath.Base(ed.Path)
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
	u.sessionEvents()
	u.handleDrops()
	for _, t := range u.tabs {
		t.nb.Title, t.nb.Icon, t.nb.IconColor = u.tabLook(t)
	}
	gtk.Fill(gtx, u.th.Pal.BgColor)
	prev := u.nb.Selected
	layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.nb.Layout(gtx, u.th) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if u.nb.Selected < len(u.tabs) {
						t := u.tabs[u.nb.Selected]
						if (u.focus || prev != u.nb.Selected) && !u.prompter.Active() {
							t.view.Focus(gtx)
							u.focus = false
						}
						return t.view.Layout(gtx, u.th.Shaper)
					}
					gtk.Fill(gtx, u.th.Pal.BaseColor)
					return u.editor.Layout(gtx, u.th.Shaper)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					icon, c, left, right := u.status()
					kids := []layout.FlexChild{}
					if icon != 0 {
						kids = append(kids, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.th.IconBox(gtx, icon, 16, c) }))
					}
					kids = append(kids, layout.Flexed(1, u.th.StatusText(left)))
					for _, r := range right {
						kids = append(kids, layout.Rigid(u.th.StatusText(r)))
					}
					return u.th.Statusbar(gtx, kids...)
				}),
			)
		}),
		// The backend's questions (passwords, host keys) on top.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !u.prompter.Active() {
				// After a dialog the terminal gets the keyboard back.
				return layout.Dimensions{}
			}
			u.focus = true
			return u.prompter.Layout(gtx, u.th)
		}),
	)
}

// tabLook gives a terminal tab its title and an icon colored by the state.
func (u *ui) tabLook(t *tab) (string, rune, color.NRGBA) {
	icon := fonts.IconTerminal
	switch t.kind {
	case "ssh":
		icon = fonts.IconServer
	case "telnet":
		icon = fonts.IconPlug
	}
	c := u.th.Pal.WarningFg
	switch t.sess.State() {
	case "connected":
		c = u.th.Pal.SuccessFg
	case "disconnected", "closed":
		c = u.th.Pal.ErrorFg
	}
	title := t.sess.Title()
	if t.kind == "local" {
		title += " · " + t.engine
	}
	return title, icon, c
}

func (u *ui) status() (rune, color.NRGBA, string, []string) {
	if u.nb.Selected < len(u.tabs) {
		t := u.tabs[u.nb.Selected]
		_, _, c := u.tabLook(t)
		icon := fonts.IconPlug
		if st := t.sess.State(); st == "disconnected" || st == "closed" {
			icon = fonts.IconPowerOff
		}
		left := "Status: " + strings.ToUpper(t.sess.State())
		if s := t.sess.Status(); s != "" {
			left += " · " + s
		}
		var right []string
		if u.dropMsg != "" {
			right = append(right, u.dropMsg)
		}
		if d := t.sess.Dir(); d != "" {
			right = append(right, "Folder: "+d)
		}
		right = append(right, "Engine: "+t.engine)
		return icon, c, left, right
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
