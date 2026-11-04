package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileNamePart(t *testing.T) {
	assert.Equal(t, "Local_Shell", fileNamePart("Local Shell"))
	assert.Equal(t, "root@host_a_b", fileNamePart(`root@host a/b`))
	assert.Equal(t, "terminal", fileNamePart("  ..  "))
}

func TestScreenshotFolder(t *testing.T) {
	dir := t.TempDir()
	store, err := config.NewStore(config.StoreOptions{ConfigDir: dir, NoBackup: true})
	require.NoError(t, err)
	require.NoError(t, store.Load())
	ss := NewSystemService(nil)
	ss.SetStore(store)

	// Default: <config dir>/screenshots, readable name, user-only permissions.
	png := "data:image/png;base64,iVBORw0KGgo="
	path, err := ss.SaveScreenshot("", "Local Shell", png)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "screenshots"), filepath.Dir(path))
	assert.True(t, strings.HasPrefix(filepath.Base(path), "Local_Shell_"))
	if fi, err := os.Stat(path); assert.NoError(t, err) && os.PathSeparator == '/' {
		assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	}

	// A second screenshot in the same second gets its own file.
	path2, err := ss.SaveScreenshot("", "Local Shell", png)
	require.NoError(t, err)
	assert.NotEqual(t, path, path2)

	// Configured folder.
	custom := filepath.Join(dir, "shots")
	store.Config().Terminal.ScreenshotsFolder = custom
	path3, err := ss.SaveScreenshot("", "x", png)
	require.NoError(t, err)
	assert.Equal(t, custom, filepath.Dir(path3))
	assert.Equal(t, filepath.Join(dir, "session_logs"), ss.SessionLogsDir())
}
