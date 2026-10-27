package secrets

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestVaultPassphraseModeRoundTripNonASCII(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-vault-pwd-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	masterPwd := "Mäster-Pässwörd!€🔒"
	vault := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: masterPwd,
	})

	// Set complex non-ASCII secrets
	vault.SetGlobal("socks_password", "söcks-s3cret!€")
	vault.SetConnection("c1", "password", "pässwörd-with-ümlaut-and-emöji-🚀")
	vault.SetConnection("c1", "expect:e1", "h!dd3n_s3nd_valüe")

	require.NoError(t, vault.Save())

	// Reopen with correct password
	vault2 := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: masterPwd,
	})
	require.NoError(t, vault2.Load())

	assert.Equal(t, "söcks-s3cret!€", vault2.GetGlobal("socks_password"))
	assert.Equal(t, "pässwörd-with-ümlaut-and-emöji-🚀", vault2.GetConnection("c1", "password"))
	assert.Equal(t, "h!dd3n_s3nd_valüe", vault2.GetConnection("c1", "expect:e1"))

	// Reopen with wrong password
	vaultWrong := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: "wrong-password",
	})
	err = vaultWrong.Load()
	assert.ErrorIs(t, err, ErrWrongPassword)
}

func TestVaultKeyringMode(t *testing.T) {
	keyring.MockInit()

	tmpDir, err := os.MkdirTemp("", "leanterm-vault-keyring-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	vault := NewVault(VaultOptions{
		ConfigDir: tmpDir,
	})

	vault.SetConnection("c2", "password", "keyring-stored-pwd-123")
	require.NoError(t, vault.Save())

	// Reopen using keyring
	vault2 := NewVault(VaultOptions{
		ConfigDir: tmpDir,
	})
	require.NoError(t, vault2.Load())
	assert.Equal(t, "keyring-stored-pwd-123", vault2.GetConnection("c2", "password"))
}

func TestVaultMasterPasswordChange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-vault-change-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	vault := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: "oldPassword1",
	})

	vault.SetConnection("c3", "password", "secret-value")
	require.NoError(t, vault.Save())

	// Change password
	require.NoError(t, vault.SetMasterPassword("oldPassword1", "newPassword2"))

	// Reload with new password
	vaultNew := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: "newPassword2",
	})
	require.NoError(t, vaultNew.Load())
	assert.Equal(t, "secret-value", vaultNew.GetConnection("c3", "password"))

	// Reload with old password should fail
	vaultOld := NewVault(VaultOptions{
		ConfigDir:      tmpDir,
		ProtectWithPwd: true,
		MasterPassword: "oldPassword1",
	})
	assert.ErrorIs(t, vaultOld.Load(), ErrWrongPassword)
}

func TestVaultDuplicateAndGarbageCollect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-vault-gc-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	vault := NewVault(VaultOptions{
		ConfigDir: tmpDir,
	})

	vault.SetConnection("c1", "password", "pwd1")
	vault.DuplicateConnection("c1", "c1-clone")
	assert.Equal(t, "pwd1", vault.GetConnection("c1-clone", "password"))

	// Garbage collect
	activeIDs := map[string]bool{
		"c1-clone": true,
	}
	vault.GarbageCollect(activeIDs)

	assert.Equal(t, "", vault.GetConnection("c1", "password"), "deleted connection secret should be removed")
	assert.Equal(t, "pwd1", vault.GetConnection("c1-clone", "password"), "active connection secret should be retained")
}
