// Package filedialog shows the system's file dialogs for the native GUI:
// the XDG desktop portal (or zenity/kdialog) on Linux, the common dialogs
// on Windows. Choose blocks until the dialog closes; call it outside the
// GUI's event handling.
package filedialog

import (
	"errors"
	"net/url"
	"strings"
)

// Options describe a dialog.
type Options struct {
	Title string
	// Directory is the folder the dialog starts in.
	Directory string
	// Filename is the suggested name of a save dialog.
	Filename string
	// Save asks for a file to write; Folder for a folder to open.
	Save, Folder bool
	// Parent is the window the dialog belongs to (Windows: its HWND).
	Parent uintptr
}

// ErrUnavailable means the system offers no file dialog (no portal, no
// zenity or kdialog).
var ErrUnavailable = errors.New("no file dialog available (install xdg-desktop-portal, zenity or kdialog)")

// Choose shows the dialog and returns the chosen path, "" when cancelled.
func Choose(o Options) (string, error) { return choose(o) }

// pathFromURI turns a file:// URI into a path.
func pathFromURI(u string) string {
	if !strings.HasPrefix(u, "file://") {
		return u
	}
	p, err := url.Parse(u)
	if err != nil {
		return strings.TrimPrefix(u, "file://")
	}
	return p.Path
}
