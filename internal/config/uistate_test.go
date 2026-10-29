package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIStateDialogGeometryRoundTrip(t *testing.T) {
	store, err := NewStore(StoreOptions{ConfigDir: t.TempDir()})
	require.NoError(t, err)
	defer store.Close()

	st := store.LoadUIState()
	assert.Empty(t, st.Dialogs)

	st.Dialogs = map[string]DialogGeometry{
		"preferences": {X: 40, Y: 30, Width: 900, Height: 640},
	}
	require.NoError(t, store.SaveUIState(st))

	loaded := store.LoadUIState()
	assert.Equal(t, DialogGeometry{X: 40, Y: 30, Width: 900, Height: 640}, loaded.Dialogs["preferences"])
	assert.Equal(t, st.CommandPanelWidth, loaded.CommandPanelWidth)
}
