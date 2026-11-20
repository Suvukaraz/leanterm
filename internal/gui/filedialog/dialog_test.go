//go:build !windows

package filedialog

import (
	"strings"
	"testing"
)

func TestFallbackArgs(t *testing.T) {
	for _, tc := range []struct {
		o      Options
		zenity string
		kde    string
	}{
		{Options{Title: "Open", Directory: "/home/me"}, "--file-selection --title=Open --filename=/home/me/", "--title Open --getopenfilename /home/me"},
		{Options{Title: "Save", Directory: "/tmp", Filename: "log.txt", Save: true}, "--file-selection --title=Save --filename=/tmp/log.txt --save --confirm-overwrite", "--title Save --getsavefilename /tmp/log.txt"},
		{Options{Title: "Dir", Folder: true}, "--file-selection --title=Dir --directory", "--title Dir --getexistingdirectory ."},
	} {
		if got := strings.Join(zenityArgs(tc.o), " "); got != tc.zenity {
			t.Errorf("zenity %q", got)
		}
		if got := strings.Join(kdialogArgs(tc.o), " "); got != tc.kde {
			t.Errorf("kdialog %q", got)
		}
	}
	if pathFromURI("file:///home/me/a%20b.txt") != "/home/me/a b.txt" {
		t.Error("pathFromURI")
	}
}
