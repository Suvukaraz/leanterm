package main

import (
	"log"
	"time"

	"gioui.org/app"

	"github.com/Suvukaraz/leanterm/internal/gui/dnd"
)

// hwndOf is set per platform: the native window handle for dnd.Enable.
func (u *ui) enableDrop(e app.ViewEvent) {
	if h := hwndOf(e); h != 0 {
		if err := dnd.Enable(h, u.onDrop); err != nil {
			u.dropMsg = "File drop: " + err.Error()
			return
		}
		u.dropMsg = ""
		log.Print("file drop enabled")
		if u.selfDrop != "" {
			path := u.selfDrop
			u.selfDrop = ""
			go func() {
				time.Sleep(time.Second)
				if err := dnd.SimulateDrop(h, []string{path}, 40, 60); err != nil {
					log.Print("self-test drop: ", err)
				}
			}()
		}
	}
}
