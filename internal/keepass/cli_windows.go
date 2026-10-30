//go:build windows

package keepass

import (
	"os"
	"os/exec"
	"path/filepath"
)

// cliPath finds keepassxc-cli.exe: in PATH, else in the default install folders
// (the KeePassXC installer does not add itself to PATH).
func cliPath() string {
	if p, err := exec.LookPath("keepassxc-cli.exe"); err == nil {
		return p
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		dir := filepath.Join(base, "KeePassXC")
		if env == "LOCALAPPDATA" {
			dir = filepath.Join(base, "Programs", "KeePassXC")
		}
		if p := filepath.Join(dir, "keepassxc-cli.exe"); fileExists(p) {
			return p
		}
	}
	return "keepassxc-cli.exe"
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
