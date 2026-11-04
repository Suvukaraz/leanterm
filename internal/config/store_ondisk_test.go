package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LeanEdit (a second process) changes only the editor settings and keeps what
// LeanTerm wrote to config.toml after LeanEdit loaded it.
func TestUpdateOnDiskKeepsOtherChanges(t *testing.T) {
	dir := t.TempDir()
	leanTerm, err := NewStore(StoreOptions{ConfigDir: dir, NoBackup: true})
	require.NoError(t, err)
	require.NoError(t, leanTerm.Load())
	require.NoError(t, leanTerm.Save())

	leanEdit, err := NewStore(StoreOptions{ConfigDir: dir, NoBackup: true})
	require.NoError(t, err)
	require.NoError(t, leanEdit.Load())

	// LeanTerm saves another setting after LeanEdit started.
	leanTerm.Config().Terminal.ScreenshotsFolder = "/tmp/shots"
	require.NoError(t, leanTerm.Save())

	require.NoError(t, leanEdit.UpdateOnDisk(func(cfg *Config) { cfg.Editor.TabWidth = 8 }))
	assert.Equal(t, 8, leanEdit.Config().Editor.TabWidth)

	data, err := os.ReadFile(leanTerm.ConfigPath())
	require.NoError(t, err)
	reloaded, err := leanTerm.parseConfig(data)
	require.NoError(t, err)
	assert.Equal(t, 8, reloaded.Editor.TabWidth)
	assert.Equal(t, "/tmp/shots", reloaded.Terminal.ScreenshotsFolder)

	// Invalid values are rejected and nothing is written.
	assert.Error(t, leanEdit.UpdateOnDisk(func(cfg *Config) { cfg.Editor.TabWidth = 99 }))
	data2, _ := os.ReadFile(leanTerm.ConfigPath())
	assert.Equal(t, data, data2)
}
