package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/platform"
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

// ScreenshotsDir is the folder for terminal screenshots: Preferences > Terminal
// Options > Screenshots, else <config dir>/screenshots.
func (ss *SystemService) ScreenshotsDir() string {
	return ss.folder(func(cfg *config.Config) string { return cfg.Terminal.ScreenshotsFolder }, "screenshots")
}

// SessionLogsDir is the folder for session logs: Preferences > Terminal Options >
// Session Logs, else <config dir>/session_logs. "Save session log…" starts there.
func (ss *SystemService) SessionLogsDir() string {
	return ss.folder(func(cfg *config.Config) string { return cfg.Terminal.SessionLogs.Folder }, "session_logs")
}

func (ss *SystemService) folder(configured func(*config.Config) string, fallback string) string {
	if ss.store == nil {
		return os.TempDir()
	}
	if dir := expandHome(strings.TrimSpace(configured(ss.store.Config()))); dir != "" {
		return dir
	}
	return filepath.Join(ss.store.ConfigDir(), fallback)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

// fileNamePart turns a session title into something usable in a file name.
func fileNamePart(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, strings.Join(strings.Fields(name), "_"))
	name = strings.Trim(name, ". ")
	if len(name) > 60 {
		name = name[:60]
	}
	if name == "" {
		return "terminal"
	}
	return name
}

// SaveScreenshot stores a PNG of a terminal (base64, optionally a data URL) as
// "<name>_<date>_<time>.png" in ScreenshotsDir and returns its path. For a saved
// connection the file is also listed in its Info tab.
func (ss *SystemService) SaveScreenshot(connID, name, base64PNG string) (string, error) {
	if idx := strings.Index(base64PNG, ","); idx >= 0 {
		base64PNG = base64PNG[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(base64PNG)
	if err != nil {
		return "", fmt.Errorf("failed to decode screenshot: %w", err)
	}
	if len(data) == 0 {
		return "", errors.New("the screenshot is empty")
	}

	dir := ss.ScreenshotsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create the screenshots folder %s: %w", dir, err)
	}
	base := fileNamePart(name) + "_" + time.Now().Format("20060102_150405")
	filePath := filepath.Join(dir, base+".png")
	for i := 2; ; i++ {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filePath = filepath.Join(dir, fmt.Sprintf("%s_%d.png", base, i))
	}
	// Terminal contents can be confidential: readable by the user only.
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write the screenshot: %w", err)
	}

	if ss.store != nil && connID != "" {
		cfg := ss.store.Config()
		for i := range cfg.Connections {
			if cfg.Connections[i].ID == connID {
				cfg.Connections[i].Screenshots = append(cfg.Connections[i].Screenshots, filePath)
				ss.store.MarkDirty()
				break
			}
		}
	}
	return filePath, nil
}

// ChooseSaveFileIn shows a "save as" dialog starting in dir ("" = cancelled).
func (ss *SystemService) ChooseSaveFileIn(title, dir, defaultName string) (string, error) {
	if ss.app == nil {
		return "", errors.New("application not ready")
	}
	if dir != "" {
		_ = os.MkdirAll(dir, 0700)
	}
	dlg := ss.app.Dialog.SaveFile().
		SetMessage(title).
		CanCreateDirectories(true).
		SetFilename(defaultName)
	if dir = startDirectory(dir); dir != "" {
		dlg.SetDirectory(dir)
	}
	if win := ss.app.Window.Current(); win != nil {
		dlg.AttachToWindow(win)
	}
	return dlg.PromptForSingleSelection()
}

// DefaultLogName is the suggested file name for "Save session log…".
func (ss *SystemService) DefaultLogName(name string) string {
	return fileNamePart(name) + "_" + time.Now().Format("20060102_150405") + ".log"
}

func (ss *SystemService) OpenURL(url string) error {
	return platform.OpenURL(url)
}

func (ss *SystemService) OpenPath(path string) error {
	return ss.OpenURL(path)
}

func (ss *SystemService) AppInfo() map[string]string {
	return map[string]string{
		"name":        "LeanTerm",
		"version":     "1.1.1",
		"license":     "GPLv3",
		"description": "SSH, Telnet and SFTP connection manager for Linux and Windows (Go + Wails + Svelte)",
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
