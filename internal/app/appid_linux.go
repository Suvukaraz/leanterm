package app

import (
	"unsafe"

	"github.com/godbus/dbus/v5"
)

// SetAppUserModelID is Windows only; on Linux the GTK application id does this.
func SetAppUserModelID(string) {}

// SetWindowIcon is Windows only; on Linux the desktop file names the icon.
func SetWindowIcon(unsafe.Pointer, []byte) {}

// AllowForegroundForOthers is Windows only.
func AllowForegroundForOthers() {}

// SingleInstancePossible reports whether the single instance lock can work: on
// Linux it is a D-Bus name, and without a session bus Wails would stop the
// program instead of starting it.
func SingleInstancePossible() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
