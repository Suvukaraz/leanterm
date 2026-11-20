//go:build windows

package main

import (
	"log"

	"gioui.org/app"

	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
)

// maybeEnableDrop subclasses the window to accept dropped files (see dnd).
func (u *ui) maybeEnableDrop(e app.ViewEvent) {
	if ev, ok := e.(app.Win32ViewEvent); ok && ev.Valid() {
		if err := dnd.Enable(uintptr(ev.HWND), u.onDrop); err != nil {
			log.Print("file drop disabled: ", err)
		}
	}
}
