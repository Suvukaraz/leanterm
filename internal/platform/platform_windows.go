//go:build windows

package platform

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	cmd := exec.CommandContext(ctx, comspec)
	// cmd.exe parses its own command line: pass the whole line (program name
	// included) verbatim instead of letting Go quote the command as a single
	// argument. /S strips the outer quotes.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       syscall.EscapeArg(comspec) + ` /S /C "` + command + `"`,
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd
}

func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}

// openFile uses ShellExecute, which fails when no application is associated.
func openFile(path string) error {
	return shellExecute(path)
}

func openURL(url string) error {
	return shellExecute(url)
}

func shellExecute(target string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// AppDirName is the name of LeanTerm's folder inside ConfigBaseDir.
const AppDirName = "LeanTerm"

func configBaseDir() (string, error) {
	return os.UserConfigDir() // %APPDATA%
}

func downloadsDir() string {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
	if err != nil {
		return ""
	}
	return dir
}
