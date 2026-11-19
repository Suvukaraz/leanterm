package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procCreateIconFromResEx = user32.NewProc("CreateIconFromResourceEx")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSendMessage         = user32.NewProc("SendMessageW")
)

// SetWindowIcon replaces a window's icon (title bar, task bar) with the given
// PNG. LeanEdit uses it: the executable's own icon is LeanTerm's.
func SetWindowIcon(hwnd unsafe.Pointer, png []byte) {
	if hwnd == nil || len(png) == 0 {
		return
	}
	const (
		wmSetIcon            = 0x0080
		iconSmall, iconBig   = 0, 1
		smCxIcon, smCxSmIcon = 11, 49
	)
	if big := iconFromPNG(png, smCxIcon); big != 0 {
		procSendMessage.Call(uintptr(hwnd), wmSetIcon, iconBig, big)
	}
	if small := iconFromPNG(png, smCxSmIcon); small != 0 {
		procSendMessage.Call(uintptr(hwnd), wmSetIcon, iconSmall, small)
	}
}

// iconFromPNG makes an icon of the system size given by metric (SM_CXICON,
// SM_CXSMICON); Windows takes PNG data as icon resource bits.
func iconFromPNG(png []byte, metric uintptr) uintptr {
	size, _, _ := procGetSystemMetrics.Call(metric)
	const iconResourceVersion = 0x00030000
	h, _, _ := procCreateIconFromResEx.Call(
		uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)),
		1, // icon, not cursor
		iconResourceVersion, size, size, 0)
	return h
}
