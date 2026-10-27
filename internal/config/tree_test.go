package config

import (
	"os"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestTree(t *testing.T) (*Store, *secrets.Vault, *TreeManager, func()) {
	tmpDir, err := os.MkdirTemp("", "leanterm-tree-*")
	require.NoError(t, err)

	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	require.NoError(t, err)
	require.NoError(t, store.Load())

	vault := secrets.NewVault(secrets.VaultOptions{ConfigDir: tmpDir})
	tm := NewTreeManager(store, vault)

	cleanup := func() {
		_ = store.Close()
		_ = vault.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return store, vault, tm, cleanup
}

func TestTreeAddAndRename(t *testing.T) {
	_, _, tm, cleanup := setupTestTree(t)
	defer cleanup()

	grp, err := tm.AddGroup("", "Production")
	require.NoError(t, err)
	assert.NotEmpty(t, grp.ID)
	assert.Equal(t, "Production", grp.Name)

	conn, err := tm.AddConnection(grp.ID, "web01", "")
	require.NoError(t, err)
	assert.Equal(t, "web01", conn.Name)
	assert.Equal(t, "web01 - Production", conn.Title)

	// Rename
	require.NoError(t, tm.RenameNode(conn.ID, "web-primary", "web-primary - Production"))
	tree := tm.BuildTree()
	require.Len(t, tree.Children, 1)
	assert.Equal(t, "Production", tree.Children[0].Name)
	require.Len(t, tree.Children[0].Children, 1)
	assert.Equal(t, "web-primary", tree.Children[0].Children[0].Name)
}

func TestTreeProtectionRules(t *testing.T) {
	_, _, tm, cleanup := setupTestTree(t)
	defer cleanup()

	grp, err := tm.AddGroup("", "ProtectedGroup")
	require.NoError(t, err)
	conn, err := tm.AddConnection(grp.ID, "safe-server", "")
	require.NoError(t, err)

	// Protect group
	require.NoError(t, tm.SetProtected([]string{grp.ID}, true))

	// Adding to protected group should fail
	_, err = tm.AddConnection(grp.ID, "new-server", "")
	assert.ErrorIs(t, err, ErrNodeProtected)

	// Deleting protected group should fail
	err = tm.DeleteNodes([]string{grp.ID})
	assert.ErrorIs(t, err, ErrNodeProtected)

	// Deleting child of protected group should fail
	err = tm.DeleteNodes([]string{conn.ID})
	assert.ErrorIs(t, err, ErrNodeProtected)

	// Unprotect
	require.NoError(t, tm.SetProtected([]string{grp.ID}, false))
	require.NoError(t, tm.DeleteNodes([]string{conn.ID}))
	assert.Empty(t, tm.BuildTree().Children[0].Children)
}

func TestTreeMoveAndCyclePrevention(t *testing.T) {
	_, _, tm, cleanup := setupTestTree(t)
	defer cleanup()

	g1, err := tm.AddGroup("", "G1")
	require.NoError(t, err)
	g2, err := tm.AddGroup(g1.ID, "G2")
	require.NoError(t, err)
	g3, err := tm.AddGroup(g2.ID, "G3")
	require.NoError(t, err)

	// Trying to move G1 into G3 should fail (cycle)
	err = tm.MoveNodes([]string{g1.ID}, g3.ID)
	assert.ErrorIs(t, err, ErrInvalidParent)

	// Move G3 to root should succeed
	require.NoError(t, tm.MoveNodes([]string{g3.ID}, ""))
	tree := tm.BuildTree()
	require.Len(t, tree.Children, 2)
}

func TestTreeCloneAndSecretDuplication(t *testing.T) {
	_, vault, tm, cleanup := setupTestTree(t)
	defer cleanup()

	conn, err := tm.AddConnection("", "ssh-host", "")
	require.NoError(t, err)
	vault.SetConnection(conn.ID, "password", "super-secret-pw")

	clonedID, err := tm.CloneNode(conn.ID)
	require.NoError(t, err)
	assert.NotEqual(t, conn.ID, clonedID)

	assert.Equal(t, "super-secret-pw", vault.GetConnection(clonedID, "password"))
}

func TestTreeSearch(t *testing.T) {
	_, _, tm, cleanup := setupTestTree(t)
	defer cleanup()

	g, _ := tm.AddGroup("", "Datacenter A")
	c1, _ := tm.AddConnection(g.ID, "alpha", "")
	c2, _ := tm.AddConnection(g.ID, "beta", "")
	c3, _ := tm.AddConnection("", "alpha-backup", "")

	found := tm.Search("alpha", "name", "", false)
	assert.Equal(t, c1.ID, found)

	// Next match
	foundNext := tm.Search("alpha", "name", c1.ID, false)
	assert.Equal(t, c3.ID, foundNext)

	// Backwards
	foundPrev := tm.Search("alpha", "name", c3.ID, true)
	assert.Equal(t, c1.ID, foundPrev)

	_ = c2
}
