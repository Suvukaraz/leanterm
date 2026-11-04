package app

import (
	"syscall"
	"unsafe"
)

// SetAppUserModelID gives the process its own AppUserModelID, so Windows shows
// its windows as a separate application (task bar button and icon) even though
// the executable is the same as LeanTerm's.
func SetAppUserModelID(id string) {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return
	}
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID")
	if proc.Find() != nil {
		return
	}
	_, _, _ = proc.Call(uintptr(unsafe.Pointer(p)))
}
