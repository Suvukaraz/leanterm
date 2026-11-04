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

// AllowForegroundForOthers lets another process (the running LeanEdit this
// start hands its files to) bring its window to the front. Windows only lets
// the process the user just started take the focus.
func AllowForegroundForOthers() {
	const asfwAny = ^uintptr(0) // ASFW_ANY = (DWORD)-1
	proc := syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")
	if proc.Find() != nil {
		return
	}
	_, _, _ = proc.Call(asfwAny)
}

// SingleInstancePossible reports whether the single instance lock can work
// (always on Windows: it uses a named mutex).
func SingleInstancePossible() bool { return true }
