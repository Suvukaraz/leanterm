package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/wol"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type SystemService struct {
	app           *application.App
	store         *config.Store
	isLocked      bool
	unlockHandler func(pwd string) error
	quitting      atomic.Bool
}

func NewSystemService(app *application.App) *SystemService {
	return &SystemService{app: app}
}

func (ss *SystemService) SetApp(app *application.App) {
	ss.app = app
}

func (ss *SystemService) SetStore(store *config.Store) {
	ss.store = store
}

func (ss *SystemService) SetUnlockHandler(h func(pwd string) error) {
	ss.unlockHandler = h
}

func (ss *SystemService) WakeOnLan(macStr, host string, port int, broadcast bool) error {
	return wol.SendMagicPacket(macStr, host, port, broadcast)
}

func (ss *SystemService) LookupArpMAC(ip string) (string, error) {
	return wol.LookupArpMAC(ip)
}

// SaveScreenshot saves base64 PNG data into a 0600 temp file in configDir/screenshots (R6)
func (ss *SystemService) SaveScreenshot(connID string, base64PNG string) (string, error) {
	idx := strings.Index(base64PNG, ",")
	rawB64 := base64PNG
	if idx >= 0 {
		rawB64 = base64PNG[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode screenshot base64: %w", err)
	}

	dir := os.TempDir()
	if ss.store != nil {
		dir = filepath.Join(ss.store.ConfigDir(), "screenshots")
	}
	_ = os.MkdirAll(dir, 0700)

	tmpFile, err := os.CreateTemp(dir, "shot-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create screenshot file: %w", err)
	}
	defer tmpFile.Close()

	_ = tmpFile.Chmod(0600)
	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("failed to write screenshot: %w", err)
	}

	filePath := tmpFile.Name()

	if ss.store != nil && connID != "" {
		cfg := ss.store.Config()
		for i := range cfg.Connections {
			if cfg.Connections[i].ID == connID {
				cfg.Connections[i].Screenshots = append(cfg.Connections[i].Screenshots, filePath)
				_ = ss.store.Save()
				break
			}
		}
	}

	return filePath, nil
}

func (ss *SystemService) OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return errors.New("unsupported platform")
	}
	return cmd.Start()
}

func (ss *SystemService) OpenPath(path string) error {
	return ss.OpenURL(path)
}

func (ss *SystemService) AppInfo() map[string]string {
	return map[string]string{
		"name":        "LeanTerm",
		"version":     "0.8.4",
		"license":     "GPLv3",
		"description": "LeanTerm Connection Manager (rewrite of Ásbrú Connection Manager in Go + Wails + Svelte)",
	}
}

func (ss *SystemService) LockGUI() {
	ss.isLocked = true
}

func (ss *SystemService) UnlockGUI(password string) error {
	if ss.unlockHandler != nil {
		if err := ss.unlockHandler(password); err != nil {
			return err
		}
	}
	ss.isLocked = false
	return nil
}

func (ss *SystemService) IsLocked() bool {
	return ss.isLocked
}

func (ss *SystemService) Quit(force bool) {
	ss.quitting.Store(true)
	if ss.app != nil {
		ss.app.Quit()
	}
}

// IsQuitting reports whether Quit was called, i.e. closing the window must not
// be intercepted any more.
func (ss *SystemService) IsQuitting() bool {
	return ss.quitting.Load()
}

// ChooseFile shows a native file (or folder) chooser and returns the selected
// absolute path, or "" if the user cancelled.
func (ss *SystemService) ChooseFile(title, startPath string, folder bool) (string, error) {
	if ss.app == nil {
		return "", errors.New("application not ready")
	}
	dlg := ss.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(!folder).
		CanChooseDirectories(folder).
		CanCreateDirectories(true).
		ShowHiddenFiles(true)
	if dir := startDirectory(startPath); dir != "" {
		dlg.SetDirectory(dir)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

// ChooseSaveFile shows a native "save as" dialog and returns the chosen path ("" = cancelled).
func (ss *SystemService) ChooseSaveFile(title, defaultName string) (string, error) {
	if ss.app == nil {
		return "", errors.New("application not ready")
	}
	dlg := ss.app.Dialog.SaveFile().
		SetMessage(title).
		CanCreateDirectories(true).
		SetFilename(defaultName)
	if home, err := os.UserHomeDir(); err == nil {
		dlg.SetDirectory(home)
	}
	return dlg.PromptForSingleSelection()
}

// startDirectory picks a sensible start folder for a chooser from a current value.
func startDirectory(p string) string {
	if p == "" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return p
	}
	return filepath.Dir(p)
}
