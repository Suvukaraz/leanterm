//go:build !windows

package main

import "gioui.org/app"

// maybeEnableDrop does nothing: Gio has no drop target on X11/Wayland yet
// (the editor is not implemented either).
func (u *ui) maybeEnableDrop(e app.ViewEvent) {}
