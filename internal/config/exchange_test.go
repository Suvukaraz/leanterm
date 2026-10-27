package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportAndImportWithArmoredSecrets(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-exchange-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := Default()
	grp := Group{ID: "grp-1", Name: "Remote Servers"}
	conn := DefaultConnection("conn-1", "grp-1", "web01")
	conn.Host = "192.168.1.50"
	conn.Auth.User = "admin"
	cfg.Groups = append(cfg.Groups, grp)
	cfg.Connections = append(cfg.Connections, conn)

	vault := secrets.NewVault(secrets.VaultOptions{ConfigDir: tmpDir})
	vault.SetConnection("conn-1", "password", "SuperSecretPW123")

	exportFile := filepath.Join(tmpDir, "export.toml")
	exportPass := "export-passphrase-456"

	// 1. Export with passwords
	err = ExportNodes(cfg, vault, []string{"grp-1", "conn-1"}, exportFile, true, exportPass, nil)
	require.NoError(t, err)

	// Verify file was written and contains armored block
	data, err := os.ReadFile(exportFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "-----BEGIN AGE ENCRYPTED FILE-----")

	// 2. Import into a new target config and vault
	targetCfg := Default()
	targetVault := secrets.NewVault(secrets.VaultOptions{ConfigDir: filepath.Join(tmpDir, "target")})

	report, err := ImportNodes(targetCfg, targetVault, "", exportFile, exportPass)
	require.NoError(t, err)
	assert.Equal(t, 1, report.GroupsImported)
	assert.Equal(t, 1, report.ConnectionsImported)
	assert.Empty(t, report.Warnings)

	// Check that connection secret was decrypted into target vault
	importedConnID := targetCfg.Connections[0].ID
	assert.Equal(t, "SuperSecretPW123", targetVault.GetConnection(importedConnID, "password"))
}

func TestImportIDCollisionRemapping(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-collision-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	exportFile := filepath.Join(tmpDir, "export.toml")
	cfg := Default()
	grp := Group{ID: "collision-id", Name: "Existing Group"}
	conn := DefaultConnection("conn-collision", "collision-id", "Existing Conn")
	cfg.Groups = append(cfg.Groups, grp)
	cfg.Connections = append(cfg.Connections, conn)

	err = ExportNodes(cfg, nil, []string{"collision-id", "conn-collision"}, exportFile, false, "", nil)
	require.NoError(t, err)

	// Target config already has these IDs
	targetCfg := Default()
	targetCfg.Groups = append(targetCfg.Groups, Group{ID: "collision-id", Name: "Original Group"})
	targetCfg.Connections = append(targetCfg.Connections, DefaultConnection("conn-collision", "collision-id", "Original Conn"))

	report, err := ImportNodes(targetCfg, nil, "", exportFile, "")
	require.NoError(t, err)
	assert.Equal(t, 1, report.GroupsImported)
	assert.Equal(t, 1, report.ConnectionsImported)

	// Verify that target config now has 2 groups and 2 connections with unique IDs
	assert.Len(t, targetCfg.Groups, 2)
	assert.Len(t, targetCfg.Connections, 2)
	assert.NotEqual(t, targetCfg.Groups[0].ID, targetCfg.Groups[1].ID)
	assert.NotEqual(t, targetCfg.Connections[0].ID, targetCfg.Connections[1].ID)
	// Parent of imported connection should match new group ID
	assert.Equal(t, targetCfg.Groups[1].ID, targetCfg.Connections[1].Parent)
}

func TestImportR14DangerousCommandWarning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-r14-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	exportFile := filepath.Join(tmpDir, "export-r14.toml")
	cfg := Default()
	conn := DefaultConnection("c-cmd", "", "Dangerous Conn")
	conn.Title = "Server <CMD:whoami>"
	conn.PreExec = []PrePostExec{
		{Command: "rm -rf /tmp/foo", Ask: false, Default: true},
	}
	cfg.Connections = append(cfg.Connections, conn)

	err = ExportNodes(cfg, nil, []string{"c-cmd"}, exportFile, false, "", nil)
	require.NoError(t, err)

	targetCfg := Default()
	report, err := ImportNodes(targetCfg, nil, "", exportFile, "")
	require.NoError(t, err)
	assert.Len(t, report.R14Commands, 1)
	assert.Contains(t, report.R14Commands[0], "<CMD:...> in Title")
	assert.Contains(t, report.R14Commands[0], "PreExec: 'rm -rf /tmp/foo'")
}

func TestExportBulkEditRegexSafe(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-bulk-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	exportFile := filepath.Join(tmpDir, "export-bulk.toml")
	cfg := Default()
	conn := DefaultConnection("c-bulk", "", "srv01")
	conn.Host = "10.0.0.5"
	cfg.Connections = append(cfg.Connections, conn)

	bulk := []BulkChanges{
		{
			Field:        "host",
			NewValue:     "10.1.0.$1",
			UseRegex:     true,
			RegexPattern: `^10\.0\.0\.(\d+)$`,
		},
	}

	err = ExportNodes(cfg, nil, []string{"c-bulk"}, exportFile, false, "", bulk)
	require.NoError(t, err)

	targetCfg := Default()
	_, err = ImportNodes(targetCfg, nil, "", exportFile, "")
	require.NoError(t, err)

	assert.Equal(t, "10.1.0.5", targetCfg.Connections[0].Host)
}
