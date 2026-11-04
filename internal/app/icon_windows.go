package app

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

// SetWindowIcon replaces a window's icon (title bar, task bar) with the given
// PNG. LeanEdit uses it: the executable's own icon is LeanTerm's.
func SetWindowIcon(hwnd unsafe.Pointer, png []byte) {
	if hwnd == nil || len(png) == 0 {
		return
	}
	const wmSetIcon, iconSmall, iconBig = 0x0080, 0, 1
	if big, err := w32.CreateLargeHIconFromImage(png); err == nil && big != 0 {
		w32.SendMessage(w32.HWND(hwnd), wmSetIcon, iconBig, uintptr(big))
	}
	if small, err := w32.CreateSmallHIconFromImage(png); err == nil && small != 0 {
		w32.SendMessage(w32.HWND(hwnd), wmSetIcon, iconSmall, uintptr(small))
	}
}
