// Package dnd accepts files dropped from the desktop (file managers) onto a
// Gio window, which Gio itself does not support.
package dnd

import "errors"

// ErrUnsupported is returned where dropping files is not implemented yet.
var ErrUnsupported = errors.New("dropping files is not supported on this platform yet")

// Drop is a set of files dropped at a window position (in pixels).
type Drop struct {
	Paths []string
	X, Y  int
}
