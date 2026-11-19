//go:build windows

package dnd

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	user32            = windows.NewLazySystemDLL("user32.dll")
	procDragAccept    = shell32.NewProc("DragAcceptFiles")
	procDragQueryFile = shell32.NewProc("DragQueryFileW")
	procDragQueryPt   = shell32.NewProc("DragQueryPoint")
	procDragFinish    = shell32.NewProc("DragFinish")
	procSetWindowLong = user32.NewProc("SetWindowLongPtrW")
	procCallWindowPrc = user32.NewProc("CallWindowProcW")
)

const (
	wmDropFiles = 0x0233
	gwlpWndProc = ^uintptr(3) // -4
)

type target struct {
	prev   uintptr
	onDrop func(Drop)
}

var (
	// targets maps HWND to *target. It is read by the window procedure on the
	// window's thread, so no lock may be held while calling into Windows:
	// SetWindowLongPtr and DragAcceptFiles send messages (WM_STYLECHANGING,
	// ...) synchronously to that thread.
	targets       sync.Map
	procGetWndPtr = user32.NewProc("GetWindowLongPtrW")
	wndProc       = syscall.NewCallback(dropWndProc)
)

// Enable makes the window accept dropped files; onDrop runs on the window's
// thread, so it should only hand the drop over (and invalidate the window).
// hwnd is the HWND from app.Win32ViewEvent.
func Enable(hwnd uintptr, onDrop func(Drop)) error {
	if v, ok := targets.Load(hwnd); ok {
		v.(*target).onDrop = onDrop
		return nil
	}
	// Window subclassing by replacing the window procedure; it works from any
	// thread of the process (SetWindowSubclass would need the window's
	// thread). The previous procedure is known before ours can run.
	prev, _, err := procGetWndPtr.Call(hwnd, gwlpWndProc)
	if prev == 0 {
		return err
	}
	targets.Store(hwnd, &target{prev: prev, onDrop: onDrop})
	if old, _, err := procSetWindowLong.Call(hwnd, gwlpWndProc, wndProc); old == 0 {
		targets.Delete(hwnd)
		return err
	}
	procDragAccept.Call(hwnd, 1)
	return nil
}

func dropWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	v, ok := targets.Load(hwnd)
	if !ok {
		return 0
	}
	t := v.(*target)
	if msg == wmDropFiles {
		hdrop := wparam
		n, _, _ := procDragQueryFile.Call(hdrop, 0xFFFFFFFF, 0, 0)
		d := Drop{}
		for i := uintptr(0); i < n; i++ {
			size, _, _ := procDragQueryFile.Call(hdrop, i, 0, 0)
			buf := make([]uint16, size+1)
			procDragQueryFile.Call(hdrop, i, uintptr(unsafe.Pointer(&buf[0])), size+1)
			d.Paths = append(d.Paths, windows.UTF16ToString(buf))
		}
		var pt struct{ X, Y int32 }
		procDragQueryPt.Call(hdrop, uintptr(unsafe.Pointer(&pt)))
		d.X, d.Y = int(pt.X), int(pt.Y)
		procDragFinish.Call(hdrop)
		if t.onDrop != nil && len(d.Paths) > 0 {
			t.onDrop(d)
		}
		return 0
	}
	r, _, _ := procCallWindowPrc.Call(t.prev, hwnd, msg, wparam, lparam)
	return r
}

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procPostMessage  = user32.NewProc("PostMessageW")
	procMoveMemory   = kernel32.NewProc("RtlMoveMemory")
)

// SimulateDrop posts WM_DROPFILES with paths to a window of this process, as
// the shell does after a drag from Explorer; for self-tests.
func SimulateDrop(hwnd uintptr, paths []string, x, y int) error {
	var names []uint16
	for _, p := range paths {
		u, err := windows.UTF16FromString(p)
		if err != nil {
			return err
		}
		names = append(names, u...)
	}
	names = append(names, 0)
	const header = 20 // DROPFILES: pFiles, pt.x, pt.y, fNC, fWide
	size := uintptr(header + 2*len(names))
	h, _, err := procGlobalAlloc.Call(0x0042, size) // GMEM_MOVEABLE|GMEM_ZEROINIT
	if h == 0 {
		return err
	}
	ptr, _, err := procGlobalLock.Call(h)
	if ptr == 0 {
		return err
	}
	// Built in Go memory and copied, as the global memory is only a uintptr.
	buf := make([]byte, size)
	put := func(off int, v uint32) {
		buf[off], buf[off+1], buf[off+2], buf[off+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	put(0, header)
	put(4, uint32(int32(x)))
	put(8, uint32(int32(y)))
	put(12, 0)
	put(16, 1) // wide characters
	for i, c := range names {
		buf[header+2*i], buf[header+2*i+1] = byte(c), byte(c>>8)
	}
	procMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&buf[0])), size)
	procGlobalUnlock.Call(h)
	if r, _, err := procPostMessage.Call(hwnd, wmDropFiles, h, 0); r == 0 {
		return err
	}
	return nil
}
