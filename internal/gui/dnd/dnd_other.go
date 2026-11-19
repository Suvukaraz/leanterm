//go:build !windows

package dnd

// Enable is not implemented for X11 and Wayland yet: Gio's event loops ignore
// the XDND client messages and the Wayland data-device drop, so this needs a
// change in Gio itself (see docs/gio-migration.md).
func Enable(uintptr, func(Drop)) error { return ErrUnsupported }

// SimulateDrop is only implemented on Windows.
func SimulateDrop(uintptr, []string, int, int) error { return ErrUnsupported }
