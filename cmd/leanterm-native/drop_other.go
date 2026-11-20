//go:build !windows

package main

import "gioui.org/app"

// maybeEnableDrop does nothing: Gio has no drop target on X11/Wayland yet
// (dropped files open in LeanEdit where drops work).
func (u *ui) maybeEnableDrop(e app.ViewEvent) {}
