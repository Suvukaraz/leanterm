//go:build windows

package main

import (
	"log"

	"gioui.org/app"

	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
)

// maybeEnableDrop subclasses the window to accept dropped files (see dnd).
//
// Gio's window thread waits while the program handles an event, and
// subclassing sends the window messages: called here directly, both wait
// for each other (the window hung at its start). It runs in its own
// goroutine, as in cmd/leanterm-gio.
func (u *ui) maybeEnableDrop(e app.ViewEvent) {
	ev, ok := e.(app.Win32ViewEvent)
	if !ok || !ev.Valid() || ev.HWND == u.dropHWND {
		return
	}
	u.dropHWND = ev.HWND
	h := uintptr(ev.HWND)
	go func() {
		if err := dnd.Enable(h, u.onDrop); err != nil {
			log.Print("file drop disabled: ", err)
			return
		}
		log.Print("file drop enabled")
	}()
}
