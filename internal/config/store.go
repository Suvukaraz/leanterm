package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	toml "github.com/pelletier/go-toml/v2"
)

var (
	ErrReadOnly       = errors.New("configuration is read-only")
	ErrNoValidConfig  = errors.New("You have no valid configuration file to load! Please inspect configuration manually.")
)

type StoreOptions struct {
	ConfigDir string
	NoBackup  bool
	ReadOnly  bool
}

type Store struct {
	mu           sync.Mutex
	opts         StoreOptions
	cfg          *Config
	dirty        bool
	autoSaveTimer *time.Timer
	watcher      *fsnotify.Watcher
	stopWatcher  chan struct{}
	lastSaved    time.Time
	warnings     []string

	OnDirtyChange func(dirty bool)
	OnReload      func()
	OnConflict    func()
}

func (s *Store) ConfigDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opts.ConfigDir
}

func GetDefaultConfigDir() string {
	if env := os.Getenv("LEANTERM_CONFIG_DIR"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "leanterm")
	}
	dir := filepath.Join(home, ".config", "leanterm")
	migrateLegacyConfigDir(filepath.Join(home, ".config", "linxterm"), dir)
	return dir
}

// migrateLegacyConfigDir moves the configuration of the program's former name
// (LinXTerm) to the new location once, if only the old directory exists.
func migrateLegacyConfigDir(legacy, dir string) {
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return
	}
	if fi, err := os.Stat(legacy); err != nil || !fi.IsDir() {
		return
	}
	_ = os.Rename(legacy, dir)
}

func ResolveConfigDir(customDir string) string {
	if customDir == "" {
		return GetDefaultConfigDir()
	}
	if filepath.IsAbs(customDir) {
		return customDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return customDir
	}
	return filepath.Join(home, ".config", customDir)
}

func NewStore(opts StoreOptions) (*Store, error) {
	opts.ConfigDir = ResolveConfigDir(opts.ConfigDir)

	s := &Store{
		opts:        opts,
		cfg:         Default(),
		stopWatcher: make(chan struct{}),
	}

	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) ensureDirectories() error {
	dirs := []string{
		s.opts.ConfigDir,
		filepath.Join(s.opts.ConfigDir, "bak"),
		filepath.Join(s.opts.ConfigDir, "screenshots"),
		filepath.Join(s.opts.ConfigDir, "session_logs"),
		filepath.Join(s.opts.ConfigDir, "tmp"),
		filepath.Join(s.opts.ConfigDir, "state"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

func (s *Store) ConfigPath() string {
	return filepath.Join(s.opts.ConfigDir, "config.toml")
}

func (s *Store) BackupDir() string {
	return filepath.Join(s.opts.ConfigDir, "bak")
}

// RotateBackups shifts existing backups config.toml.0..8 to .1..9 and writes config.toml to .0
func (s *Store) RotateBackups() error {
	if s.opts.NoBackup {
		return nil
	}

	src := s.ConfigPath()
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	bakDir := s.BackupDir()
	for i := 8; i >= 0; i-- {
		oldFile := filepath.Join(bakDir, fmt.Sprintf("config.toml.%d", i))
		newFile := filepath.Join(bakDir, fmt.Sprintf("config.toml.%d", i+1))
		if _, err := os.Stat(oldFile); err == nil {
			_ = os.Rename(oldFile, newFile)
		}
	}

	dst := filepath.Join(bakDir, "config.toml.0")
	return copyFile(src, dst)
}

// Load loads the configuration from disk, handling backups, recovery and strict decoding.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfgPath := s.ConfigPath()

	// If missing, initialize defaults
	fi, err := os.Stat(cfgPath)
	if os.IsNotExist(err) {
		s.cfg = Default()
		s.dirty = false
		return nil
	}
	if err != nil {
		return err
	}

	// Rotate backups on startup before parsing
	if !s.opts.NoBackup && fi.Size() > 0 {
		_ = s.RotateBackups()
	}

	// Try reading file
	data, err := os.ReadFile(cfgPath)
	if err != nil || len(data) == 0 {
		return s.recoverFromBackup(cfgPath, "file empty or unreadable")
	}

	cfg, parseErr := s.parseConfig(data)
	if parseErr != nil {
		return s.recoverFromBackup(cfgPath, parseErr.Error())
	}

	// Upgrade schema if needed
	loadedVersion := cfg.SchemaVersion
	if err := MigrateUpgrades(cfg); err != nil {
		return err
	}

	// Validate configuration
	valRes := Validate(cfg)
	if valRes.HasErrors() {
		return fmt.Errorf("configuration validation failed:\n%w", valRes.Error())
	}
	s.warnings = append(s.warnings, valRes.Warnings...)

	s.cfg = cfg
	s.dirty = false
	s.lastSaved = time.Now()

	// Persist migrated configs right away so the file carries the new schema version.
	if loadedVersion != cfg.SchemaVersion && !s.opts.ReadOnly {
		if err := s.saveLocked(); err != nil {
			s.warnings = append(s.warnings, fmt.Sprintf("could not save migrated configuration: %v", err))
		}
	}

	s.startWatcher()

	return nil
}

func (s *Store) parseConfig(data []byte) (*Config, error) {
	cfg := Default()

	// Attempt strict decode first
	strictDec := toml.NewDecoder(bytes.NewReader(data))
	strictDec.DisallowUnknownFields()
	if err := strictDec.Decode(cfg); err != nil {
		// Check if it's strict unknown fields error
		var strictErr *toml.StrictMissingError
		if errors.As(err, &strictErr) {
			// Record warning and backup unknown settings
			timestamp := time.Now().Format("20060102-150405")
			unknownBak := filepath.Join(s.BackupDir(), fmt.Sprintf("config.toml.unknown-%s", timestamp))
			_ = os.WriteFile(unknownBak, data, 0600)

			s.warnings = append(s.warnings, fmt.Sprintf("Unknown settings were found in config.toml and will be removed on next save: %s", strictErr.String()))

			// Re-parse with standard non-strict decoder
			cfgLoose := Default()
			if errLoose := toml.Unmarshal(data, cfgLoose); errLoose != nil {
				return nil, errLoose
			}
			return cfgLoose, nil
		}
		return nil, err
	}

	return cfg, nil
}

func (s *Store) recoverFromBackup(badPath, reason string) error {
	timestamp := time.Now().Format("20060102150405")
	badRenamed := fmt.Sprintf("%s.%s.bad", badPath, timestamp)
	_ = os.Rename(badPath, badRenamed)

	// Try bak/config.toml.0..9
	bakDir := s.BackupDir()
	for i := 0; i <= 9; i++ {
		candidate := filepath.Join(bakDir, fmt.Sprintf("config.toml.%d", i))
		cData, err := os.ReadFile(candidate)
		if err != nil || len(cData) == 0 {
			continue
		}
		cfg, parseErr := s.parseConfig(cData)
		if parseErr == nil && !Validate(cfg).HasErrors() {
			s.cfg = cfg
			s.dirty = true
			s.warnings = append(s.warnings, fmt.Sprintf("Recovered configuration from backup %s (corrupt file was saved to %s: %s)", candidate, badRenamed, reason))
			return nil
		}
	}

	return fmt.Errorf("%w Please inspect %s manually (reason: %s)", ErrNoValidConfig, badRenamed, reason)
}

// Save writes the configuration atomically to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s.opts.ReadOnly {
		return ErrReadOnly
	}

	// Normalize colors
	s.normalizeColors()

	// Validate before saving
	valRes := Validate(s.cfg)
	if valRes.HasErrors() {
		return valRes.Error()
	}

	data, err := toml.Marshal(s.cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := s.atomicWrite(s.ConfigPath(), data); err != nil {
		return err
	}

	s.dirty = false
	s.lastSaved = time.Now()

	if s.OnDirtyChange != nil {
		s.OnDirtyChange(false)
	}

	return nil
}

// MarkDirty marks the configuration as modified and schedules an autosave if enabled.
func (s *Store) MarkDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dirty = true
	if s.OnDirtyChange != nil {
		s.OnDirtyChange(true)
	}

	if s.cfg.General.AutoSave && !s.opts.ReadOnly {
		if s.autoSaveTimer != nil {
			s.autoSaveTimer.Stop()
		}
		s.autoSaveTimer = time.AfterFunc(500*time.Millisecond, func() {
			_ = s.Save()
		})
	}
}

func (s *Store) HasUnsavedChanges() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *Store) Config() *Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *Store) Warnings() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.warnings...)
}

func (s *Store) normalizeColors() {
	s.cfg.Appearance.ProtectedColor = strings.ToLower(s.cfg.Appearance.ProtectedColor)
	s.cfg.Appearance.UnprotectedColor = strings.ToLower(s.cfg.Appearance.UnprotectedColor)
	s.cfg.Terminal.Colors.Connected = strings.ToLower(s.cfg.Terminal.Colors.Connected)
	s.cfg.Terminal.Colors.Disconnected = strings.ToLower(s.cfg.Terminal.Colors.Disconnected)
	s.cfg.Terminal.Colors.NewData = strings.ToLower(s.cfg.Terminal.Colors.NewData)
	s.cfg.Terminal.Look.Foreground = strings.ToLower(s.cfg.Terminal.Look.Foreground)
	s.cfg.Terminal.Look.Background = strings.ToLower(s.cfg.Terminal.Look.Background)
	s.cfg.Terminal.Look.Bold = strings.ToLower(s.cfg.Terminal.Look.Bold)
	for i := range s.cfg.Terminal.Look.Palette {
		s.cfg.Terminal.Look.Palette[i] = strings.ToLower(s.cfg.Terminal.Look.Palette[i])
	}
}

func (s *Store) atomicWrite(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file for config: %w", err)
	}
	tmpName := tmpFile.Name()

	// Ensure cleanup if something fails before rename
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write config data: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to fsync config file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions 0600
	if err := os.Chmod(tmpName, 0600); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to atomically rename config file: %w", err)
	}

	// Fsync parent directory
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}

func (s *Store) startWatcher() {
	if s.watcher != nil {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	s.watcher = watcher

	cfgPath := s.ConfigPath()
	_ = watcher.Add(filepath.Dir(cfgPath))

	go func() {
		for {
			select {
			case <-s.stopWatcher:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name != cfgPath {
					continue
				}

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Ignore events triggered right after our own saves
					s.mu.Lock()
					timeSinceSave := time.Since(s.lastSaved)
					isDirty := s.dirty
					s.mu.Unlock()

					if timeSinceSave < 200*time.Millisecond {
						continue
					}

					if !isDirty {
						// Reload silently
						_ = s.reloadFromFile()
						if s.OnReload != nil {
							s.OnReload()
						}
					} else {
						// Conflict: external modification with unsaved local changes
						if s.OnConflict != nil {
							s.OnConflict()
						}
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

func (s *Store) reloadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.ConfigPath())
	if err != nil {
		return err
	}
	cfg, err := s.parseConfig(data)
	if err != nil {
		return err
	}
	if Validate(cfg).HasErrors() {
		return errors.New("external config is invalid")
	}

	s.cfg = cfg
	s.dirty = false
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.autoSaveTimer != nil {
		s.autoSaveTimer.Stop()
	}
	if s.watcher != nil {
		close(s.stopWatcher)
		_ = s.watcher.Close()
		s.watcher = nil
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
