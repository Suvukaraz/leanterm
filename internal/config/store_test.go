package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	defer store.Close()

	// Initial load should give defaults
	require.NoError(t, store.Load())
	cfg := store.Config()
	assert.Equal(t, "traditional", cfg.General.Layout)

	// Modify fields
	cfg.General.Layout = "compact"
	cfg.Appearance.StartTreeOn = "favourites"
	cfg.Groups = append(cfg.Groups, Group{ID: "grp1", Name: "Servers"})
	cfg.Connections = append(cfg.Connections, DefaultConnection("conn1", "grp1", "prod-web"))

	store.MarkDirty()
	assert.True(t, store.HasUnsavedChanges())

	require.NoError(t, store.Save())
	assert.False(t, store.HasUnsavedChanges())

	// Reload with a fresh store
	store2, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	defer store2.Close()

	require.NoError(t, store2.Load())
	cfg2 := store2.Config()

	assert.Equal(t, "compact", cfg2.General.Layout)
	assert.Equal(t, "favourites", cfg2.Appearance.StartTreeOn)
	require.Len(t, cfg2.Groups, 1)
	assert.Equal(t, "Servers", cfg2.Groups[0].Name)
	require.Len(t, cfg2.Connections, 1)
	assert.Equal(t, "prod-web", cfg2.Connections[0].Name)
}

func TestStoreReadOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-ro-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir, ReadOnly: true})
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Load())
	err = store.Save()
	assert.ErrorIs(t, err, ErrReadOnly)
}

func TestStoreCorruptRecovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-corrupt-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	defer store.Close()

	// 1. Save a valid configuration
	store.Config().General.Theme = "asbru-dark"
	require.NoError(t, store.Save())

	// 2. Rotate to backup
	require.NoError(t, store.RotateBackups())

	// 3. Corrupt config.toml
	cfgFile := store.ConfigPath()
	require.NoError(t, os.WriteFile(cfgFile, []byte("NOT VALID TOML {{{[[["), 0600))

	// 4. Load should detect corruption, recover from backup, and rename corrupt file to .bad
	require.NoError(t, store.Load())
	assert.Equal(t, "asbru-dark", store.Config().General.Theme)

	// Check that a .bad file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	foundBad := false
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".bad" {
			foundBad = true
			break
		}
	}
	assert.True(t, foundBad, "expected corrupt file to be renamed with .bad extension")
}

func TestStoreUnknownFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-unknown-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	defer store.Close()

	// Write TOML with an unknown field
	tomlWithUnknown := `
schema_version = 2
[general]
layout = "traditional"
unknown_legacy_key = "some_value"
`
	require.NoError(t, os.WriteFile(store.ConfigPath(), []byte(tomlWithUnknown), 0600))

	// Load should succeed, record warning, and write unknown backup
	require.NoError(t, store.Load())
	assert.NotEmpty(t, store.Warnings())
	assert.Contains(t, store.Warnings()[0], "Unknown settings were found in config.toml")

	// Verify backup was created in bak/
	bakFiles, err := os.ReadDir(store.BackupDir())
	require.NoError(t, err)
	foundUnknownBak := false
	for _, f := range bakFiles {
		if len(f.Name()) > 19 && f.Name()[:19] == "config.toml.unknown" {
			foundUnknownBak = true
			break
		}
	}
	assert.True(t, foundUnknownBak, "expected unknown settings backup in bak/")
}

func TestStoreAutosaveDebounce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-autosave-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Load())
	store.Config().General.AutoSave = true
	store.Config().General.Theme = "asbru-color"

	store.MarkDirty()
	assert.True(t, store.HasUnsavedChanges())

	// Wait 600ms for debounce timer (500ms) to fire
	time.Sleep(650 * time.Millisecond)
	assert.False(t, store.HasUnsavedChanges(), "autosave should have completed")

	// Read file on disk
	data, err := os.ReadFile(store.ConfigPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), "asbru-color")
}

func TestMigrateLegacyConfigDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-migrate-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	legacy := filepath.Join(tmpDir, "linxterm")
	dir := filepath.Join(tmpDir, "leanterm")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	migrateLegacyConfigDir(legacy, dir)
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Fatalf("config not moved: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy dir still exists")
	}

	// An existing new directory is never touched.
	_ = os.MkdirAll(legacy, 0700)
	migrateLegacyConfigDir(legacy, dir)
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy dir should be left alone: %v", err)
	}
}
