// Package platform hides the differences between Linux/macOS and Windows for
// running shell commands and opening files or URLs with the default application.
package platform

import (
	"context"
	"os/exec"
)

// ShellCommand runs command line through the system shell: /bin/sh -c on Unix,
// cmd.exe /C on Windows (without a console window).
func ShellCommand(ctx context.Context, command string) *exec.Cmd {
	return shellCommand(ctx, command)
}

// Command is exec.CommandContext that opens no console window on Windows.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	hideConsole(cmd)
	return cmd
}

// OpenFile opens a local file with its default application and reports when no
// application is associated with it.
func OpenFile(path string) error {
	return openFile(path)
}

// OpenURL opens a URL (or a path) with the default application without waiting.
func OpenURL(url string) error {
	return openURL(url)
}

// ConfigBaseDir is the folder that holds the configuration folder of LeanTerm:
// ~/.config on Unix, %APPDATA% on Windows.
func ConfigBaseDir() (string, error) {
	return configBaseDir()
}

// DownloadsDir is the system's download folder, or "" when there is none beyond
// the XDG/home-folder conventions (Unix).
func DownloadsDir() string {
	return downloadsDir()
}
