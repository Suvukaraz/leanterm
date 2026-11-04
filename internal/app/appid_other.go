//go:build !windows && !linux

package app

import "unsafe"

// SetAppUserModelID is Windows only; on Linux the GTK application id does this.
func SetAppUserModelID(string) {}

// SetWindowIcon is Windows only; on Linux the desktop file names the icon.
func SetWindowIcon(unsafe.Pointer, []byte) {}

// AllowForegroundForOthers is Windows only.
func AllowForegroundForOthers() {}

// SingleInstancePossible reports whether the single instance lock can work.
func SingleInstancePossible() bool { return true }
