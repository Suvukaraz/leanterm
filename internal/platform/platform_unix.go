//go:build !windows

package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func hideConsole(*exec.Cmd) {}

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// openFile waits for xdg-open: it returns quickly, and a non-zero exit means
// no application is associated with the file type.
func openFile(path string) error {
	if out, err := exec.Command(opener(), path).CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

func openURL(url string) error {
	return exec.Command(opener(), url).Start()
}

// AppDirName is the name of LeanTerm's folder inside ConfigBaseDir.
const AppDirName = "leanterm"

func configBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

func downloadsDir() string { return "" }
