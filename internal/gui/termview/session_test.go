package termview

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Suvukaraz/leanterm/internal/gui/appcore"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// The whole path of a native terminal tab: backend (appcore), a local shell
// of the session manager, the terminal engine and keyboard input.
func TestSessionTabOnLocalShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	core, err := appcore.Open(appcore.Options{ConfigDir: filepath.Join(home, "cfg"), NoBackup: true})
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	cmd, want := "echo tab-$((40+2))\r", "tab-42"
	if runtime.GOOS == "windows" {
		core.Store.Config().LocalShell.Binary = "cmd.exe"
		cmd, want = "echo tab-%OS%\r", "tab-Windows_NT"
	} else {
		core.Store.Config().LocalShell.Binary = "/bin/sh"
	}

	tmpID, err := core.Sessions.OpenLocalShell("tab")
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range term.Available() {
		t.Run(engine, func(t *testing.T) {
			s, err := Attach(core.Sessions, core.Bus, tmpID, engine, func() {})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.Engine.Resize(100, 30, 8, 16)
			s.Resize(100, 30)

			screen := func() string {
				var f term.Frame
				s.Engine.Snapshot(&f)
				var sb strings.Builder
				for _, c := range f.Cells {
					sb.WriteString(c.Text)
				}
				return sb.String()
			}
			wait := func(what string, cond func() bool) {
				t.Helper()
				deadline := time.Now().Add(15 * time.Second)
				for !cond() {
					if time.Now().After(deadline) {
						t.Fatalf("no %s; screen %q, state %q", what, screen(), s.State())
					}
					time.Sleep(20 * time.Millisecond)
				}
			}
			// The greeting comes from the replay of the session's output.
			wait("greeting", func() bool { return strings.Contains(screen(), "CONNECTINGWITH") })
			wait("connected state", func() bool { return s.State() == "connected" })
			s.Engine.SendText(cmd)
			wait(want, func() bool { return strings.Contains(screen(), want) })
		})
	}
}
