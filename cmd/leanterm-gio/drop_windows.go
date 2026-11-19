package main

import "gioui.org/app"

func hwndOf(e app.ViewEvent) uintptr {
	if w, ok := e.(app.Win32ViewEvent); ok && w.Valid() {
		return w.HWND
	}
	return 0
}
