package main

import (
	"log"
	"time"

	"gioui.org/app"

	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
)

// enableDrop makes the window accept dropped files. On Windows the window's
// thread waits while an event is handled here, and enabling drops sends it
// messages synchronously, so that happens on its own goroutine once the event
// is back with Gio; the result comes through u.statusCh.
func (u *ui) enableDrop(e app.ViewEvent) {
	log.Printf("view event %T (valid %v)", e, e.Valid())
	h := hwndOf(e)
	if h == 0 {
		return
	}
	selfDrop := u.selfDrop
	u.selfDrop = ""
	go func() {
		if err := dnd.Enable(h, u.onDrop); err != nil {
			u.setStatus("File drop: " + err.Error())
			return
		}
		log.Print("file drop enabled")
		u.setStatus("")
		if selfDrop != "" {
			time.Sleep(time.Second)
			if err := dnd.SimulateDrop(h, []string{selfDrop}, 40, 60); err != nil {
				log.Print("self-test drop: ", err)
			}
		}
	}()
}

// setStatus hands a drop status message to the GUI goroutine.
func (u *ui) setStatus(s string) {
	if s != "" {
		log.Print(s)
	}
	select {
	case u.statusCh <- s:
	default:
	}
	u.w.Invalidate()
}
