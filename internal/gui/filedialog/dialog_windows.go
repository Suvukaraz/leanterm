//go:build windows

package filedialog

import (
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32           = windows.NewLazySystemDLL("comdlg32.dll")
	getOpenFileNameW   = comdlg32.NewProc("GetOpenFileNameW")
	getSaveFileNameW   = comdlg32.NewProc("GetSaveFileNameW")
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	shBrowseForFolderW = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDW   = shell32.NewProc("SHGetPathFromIDListW")
	ole32              = windows.NewLazySystemDLL("ole32.dll")
	coInitializeEx     = ole32.NewProc("CoInitializeEx")
	coTaskMemFree      = ole32.NewProc("CoTaskMemFree")
	user32             = windows.NewLazySystemDLL("user32.dll")
	sendMessageW       = user32.NewProc("SendMessageW")
)

// openFileName is OPENFILENAMEW.
type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

const (
	ofnOverwritePrompt = 0x2
	ofnNoChangeDir     = 0x8
	ofnPathMustExist   = 0x800
	ofnFileMustExist   = 0x1000
	ofnExplorer        = 0x80000

	bifReturnOnlyFSDirs = 0x1
	bifNewDialogStyle   = 0x40
	bffmInitialized     = 1
	bffmSetSelectionW   = 0x400 + 103
)

// browseInfo is BROWSEINFOW.
type browseInfo struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	image       int32
}

var browseCallback = syscall.NewCallback(func(hwnd, msg, lParam, data uintptr) uintptr {
	if msg == bffmInitialized && data != 0 {
		sendMessageW.Call(hwnd, bffmSetSelectionW, 1, data)
	}
	return 0
})

func choose(o Options) (string, error) {
	// The dialogs run their own message loop on this thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	coInitializeEx.Call(0, 2) // COINIT_APARTMENTTHREADED
	title, _ := windows.UTF16PtrFromString(o.Title)

	if o.Folder && !o.Save {
		var start *uint16
		if o.Directory != "" {
			start, _ = windows.UTF16PtrFromString(o.Directory)
		}
		name := make([]uint16, windows.MAX_PATH)
		bi := browseInfo{owner: o.Parent, displayName: &name[0], title: title,
			flags: bifReturnOnlyFSDirs | bifNewDialogStyle, callback: browseCallback, lParam: uintptr(unsafe.Pointer(start))}
		pidl, _, _ := shBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl == 0 {
			return "", nil
		}
		defer coTaskMemFree.Call(pidl)
		path := make([]uint16, windows.MAX_PATH)
		if ok, _, _ := shGetPathFromIDW.Call(pidl, uintptr(unsafe.Pointer(&path[0]))); ok == 0 {
			return "", nil
		}
		return windows.UTF16ToString(path), nil
	}

	buf := make([]uint16, 32*1024)
	if o.Save && o.Filename != "" {
		copy(buf, windows.StringToUTF16(o.Filename))
	}
	var dir *uint16
	if o.Directory != "" {
		dir, _ = windows.UTF16PtrFromString(filepath.Clean(o.Directory))
	}
	filter, _ := windows.UTF16FromString("All files (*.*)\x00*.*\x00")
	ofn := openFileName{
		owner: o.Parent, filter: &filter[0], filterIndex: 1, file: &buf[0], maxFile: uint32(len(buf)),
		initialDir: dir, title: title, flags: ofnExplorer | ofnNoChangeDir | ofnPathMustExist,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	proc := getOpenFileNameW
	if o.Save {
		proc = getSaveFileNameW
		ofn.flags |= ofnOverwritePrompt
	} else {
		ofn.flags |= ofnFileMustExist
	}
	if ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn))); ok == 0 {
		return "", nil // cancelled (or failed: CommDlgExtendedError)
	}
	return windows.UTF16ToString(buf), nil
}
