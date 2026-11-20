package prefs

import (
	"errors"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/gtk"
	"github.com/Suvukaraz/leanterm/internal/services"
)

type fakeService struct {
	s        services.Settings
	saved    []services.Settings
	resets   int
	pwOld    string
	pwNew    string
	pwErr    error
	vault    string
	exported string
}

func (f *fakeService) GetSettings() services.Settings { return f.s }
func (f *fakeService) SaveSettings(s services.Settings) error {
	f.saved = append(f.saved, s)
	f.s = s
	return nil
}
func (f *fakeService) ResetSettings() error {
	f.resets++
	f.s = services.Settings{General: config.GeneralConfig{Layout: "traditional"}}
	return nil
}
func (f *fakeService) ExportFull(path string, debug, secrets bool, pass string) error {
	f.exported = path
	return nil
}
func (f *fakeService) GetVaultStatus() string { return f.vault }
func (f *fakeService) SetMasterPassword(o, n string) error {
	if f.pwErr != nil {
		return f.pwErr
	}
	f.pwOld, f.pwNew, f.vault = o, n, "passphrase"
	return nil
}

func settings() services.Settings {
	return services.Settings{
		General:   config.GeneralConfig{Layout: "traditional", Theme: "asbru-dark", ColorScheme: "dark"},
		Terminal:  config.TerminalConfig{Look: config.TerminalLookConfig{Palette: []string{"#000000"}}},
		Variables: []config.VariableConfig{{Name: "a", Value: "1"}},
	}
}

func TestEditsACopyAndSaves(t *testing.T) {
	svc := &fakeService{s: settings(), vault: "keyring"}
	d := &Dialog{Config: svc}
	var before, after services.Settings
	d.Saved = func(b, a services.Settings) { before, after = b, a }
	d.Open("")
	if len(d.s.Terminal.Look.Palette) != 16 {
		t.Fatalf("palette not filled: %v", d.s.Terminal.Look.Palette)
	}
	d.s.Variables[0].Value = "2"
	d.s.SFTPBrowser.Width = 500
	if svc.s.Variables[0].Value != "1" {
		t.Fatal("the dialog edited the service's settings, not a copy")
	}
	d.save()
	if d.Opened() || len(svc.saved) != 1 || svc.saved[0].Variables[0].Value != "2" {
		t.Fatalf("not saved: open=%v saved=%v", d.Opened(), svc.saved)
	}
	if before.SFTPBrowser.Width != 0 || after.SFTPBrowser.Width != 500 {
		t.Errorf("Saved got %d -> %d", before.SFTPBrowser.Width, after.SFTPBrowser.Width)
	}
	if d.msgOpen {
		t.Error("restart hint without a (*) change")
	}

	d.Open("")
	d.s.General.Layout = "compact"
	d.save()
	if !d.msgOpen || d.msg.Secondary != "Settings saved. Options marked with (*) take effect after restarting LeanTerm." {
		t.Errorf("no restart hint: %v %q", d.msgOpen, d.msg.Secondary)
	}
}

func TestResetAndPages(t *testing.T) {
	svc := &fakeService{s: settings()}
	d := &Dialog{Config: svc}
	d.Open("keybindings")
	if pageIDs[d.pages.Selected] != "keybindings" {
		t.Fatalf("start page %d", d.pages.Selected)
	}
	d.s.General.Layout = "compact"
	d.reset()
	if svc.resets != 1 || d.s.General.Layout != "traditional" || len(d.s.Terminal.Look.Palette) != 16 {
		t.Errorf("reset: %d %q", svc.resets, d.s.General.Layout)
	}
}

func TestMasterPassword(t *testing.T) {
	svc := &fakeService{s: settings(), vault: "keyring"}
	d := &Dialog{Config: svc}
	d.Open("")
	d.openMasterPassword()
	p := &d.pw
	p.newPw, p.confPw = "abcd", "abce"
	p.submit(d)
	if p.err != "New passwords do not match!" || !p.open {
		t.Errorf("mismatch: %q", p.err)
	}
	p.newPw, p.confPw = "abc", "abc"
	p.submit(d)
	if p.err != "Password must be at least 4 characters long." {
		t.Errorf("short: %q", p.err)
	}
	svc.pwErr = errors.New("wrong old password")
	p.newPw, p.confPw = "abcd", "abcd"
	p.submit(d)
	if p.err != "Failed to set password: wrong old password" || !p.open {
		t.Errorf("error: %q", p.err)
	}
	svc.pwErr = nil
	p.oldPw = "old"
	p.submit(d)
	if p.open || svc.pwOld != "old" || svc.pwNew != "abcd" || !d.s.General.ProtectWithPassword || d.vault != "passphrase" {
		t.Errorf("not set: open=%v %q %q protect=%v vault=%q", p.open, svc.pwOld, svc.pwNew, d.s.General.ProtectWithPassword, d.vault)
	}
}

// ui draws the dialog headless through Gio's router.
type ui struct {
	r   input.Router
	ops op.Ops
	th  *gtk.Theme
	now time.Time
	d   *Dialog
}

func (u *ui) frame() {
	u.ops.Reset()
	u.now = u.now.Add(500 * time.Millisecond)
	gtx := layout.Context{Ops: &u.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(1000, 800)}, Source: u.r.Source(), Now: u.now}
	u.d.Layout(gtx, u.th)
	u.r.Frame(&u.ops)
}

func (u *ui) click(x, y float32) {
	t := time.Duration(u.now.UnixNano())
	u.r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(x, y), Time: t},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: f32.Pt(x, y), Buttons: pointer.ButtonPrimary, Time: t},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x, y), Time: t},
	)
	u.frame()
	u.frame()
}

// TestEveryPageDraws lays out every page and sub page in both color schemes,
// and picks the pages by clicking the tabs on the left.
func TestEveryPageDraws(t *testing.T) {
	for _, scheme := range []string{"dark", "light"} {
		svc := &fakeService{s: settings()}
		svc.s.Network.Mode = "jump"
		u := &ui{th: gtk.New(scheme), now: time.Now(), d: &Dialog{Config: svc}}
		u.d.Open("")
		u.d.s.LocalShell.Look = &config.TerminalLookConfig{}
		u.frame()
		for i := range pageIDs {
			u.d.pages.Selected = i
			u.frame()
			for j := 0; j < 4; j++ {
				u.d.sub[0].Selected, u.d.sub[1].Selected = j%3, j
				u.frame()
			}
		}
		// The tab column starts at the dialog body's left (60 + 16); scan
		// down from the last page selected until it is selected again.
		u.d.pages.Selected = len(pageIDs) - 1
		u.frame()
		seen := map[int]bool{}
		for y := float32(60); y < 500 && len(seen) < len(pageIDs); y += 6 {
			u.click(130, y)
			seen[u.d.pages.Selected] = true
		}
		if len(seen) != len(pageIDs) {
			t.Errorf("%s: clicking the tabs selected pages %v", scheme, seen)
		}
	}
}
