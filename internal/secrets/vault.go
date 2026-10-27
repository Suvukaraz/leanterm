package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"filippo.io/age"
	toml "github.com/pelletier/go-toml/v2"
)

type VaultMode string

const (
	ModeKeyring     VaultMode = "keyring"
	ModePassphrase  VaultMode = "passphrase"
	ModeSessionOnly VaultMode = "session-only"
	ModeLocked      VaultMode = "locked"
)

// VaultDocument is the TOML structure stored inside secrets.age
type VaultDocument struct {
	SchemaVersion int                          `toml:"schema_version"`
	Global        map[string]string            `toml:"global,omitempty"`
	Connections   map[string]map[string]string `toml:"connections,omitempty"`
}

type VaultOptions struct {
	ConfigDir       string
	ProtectWithPwd  bool
	MasterPassword  string
	ReadOnly        bool
	NoBackup        bool
}

type Vault struct {
	mu             sync.Mutex
	configDir      string
	mode           VaultMode
	masterPassword string
	keyringMgr     *KeyringManager
	global         map[string]string
	connections    map[string]map[string]string
	dirty          bool
	saveTimer      *time.Timer
	readOnly       bool
	noBackup       bool
	warnings       []string
}

func NewVault(opts VaultOptions) *Vault {
	v := &Vault{
		configDir:      opts.ConfigDir,
		mode:           ModeKeyring,
		masterPassword: opts.MasterPassword,
		keyringMgr:     NewKeyringManager(),
		global:         make(map[string]string),
		connections:    make(map[string]map[string]string),
		readOnly:       opts.ReadOnly,
		noBackup:       opts.NoBackup,
	}

	if opts.ProtectWithPwd || opts.MasterPassword != "" {
		v.mode = ModePassphrase
	}

	return v
}

func (v *Vault) SecretsFilePath() string {
	return filepath.Join(v.configDir, "secrets.age")
}

func (v *Vault) BackupDir() string {
	return filepath.Join(v.configDir, "bak")
}

func (v *Vault) Status() VaultMode {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.mode
}

func (v *Vault) Warnings() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.warnings...)
}

func (v *Vault) RotateBackups() error {
	if v.noBackup {
		return nil
	}

	src := v.SecretsFilePath()
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	bakDir := v.BackupDir()
	_ = os.MkdirAll(bakDir, 0700)

	for i := 8; i >= 0; i-- {
		oldFile := filepath.Join(bakDir, fmt.Sprintf("secrets.age.%d", i))
		newFile := filepath.Join(bakDir, fmt.Sprintf("secrets.age.%d", i+1))
		if _, err := os.Stat(oldFile); err == nil {
			_ = os.Rename(oldFile, newFile)
		}
	}

	dst := filepath.Join(bakDir, "secrets.age.0")
	return copyFile(src, dst)
}

// Load loads and decrypts secrets.age
func (v *Vault) Load() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	path := v.SecretsFilePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// New vault
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read secrets.age: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	// Rotate backups on start
	if !v.noBackup {
		_ = v.RotateBackups()
	}

	var decrypted []byte
	if v.mode == ModePassphrase {
		if v.masterPassword == "" {
			v.mode = ModeLocked
			return nil
		}
		decrypted, err = DecryptWithPassphrase(data, v.masterPassword)
		if err != nil {
			return err
		}
	} else {
		// Try keyring
		identity, err := v.keyringMgr.GetIdentity()
		if err != nil {
			// Keyring locked or not found
			v.mode = ModeSessionOnly
			v.warnings = append(v.warnings, "Stored passwords are unavailable: system keyring is locked or missing.")
			return nil
		}

		r, err := age.Decrypt(bytes.NewReader(data), identity)
		if err != nil {
			// Decryption failed: file might be encrypted with a different key
			timestamp := time.Now().Format("20060102150405")
			badFile := fmt.Sprintf("%s.%s.bad", path, timestamp)
			_ = os.Rename(path, badFile)
			v.warnings = append(v.warnings, fmt.Sprintf("Stored passwords cannot be decrypted. Start with an empty password store? (The old file is kept as %s)", badFile))
			return nil
		}

		decrypted, err = io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("failed to read decrypted secrets: %w", err)
		}
	}

	var doc VaultDocument
	if err := toml.Unmarshal(decrypted, &doc); err != nil {
		return fmt.Errorf("failed to parse decrypted secrets: %w", err)
	}

	if doc.Global != nil {
		v.global = doc.Global
	}
	if doc.Connections != nil {
		v.connections = doc.Connections
	}
	v.dirty = false

	return nil
}

// Save encrypts and writes secrets.age atomically
func (v *Vault) Save() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.readOnly || v.mode == ModeSessionOnly || v.mode == ModeLocked {
		return nil
	}

	doc := VaultDocument{
		SchemaVersion: 1,
		Global:        v.global,
		Connections:   v.connections,
	}

	plainData, err := toml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	var encryptedData []byte

	if v.mode == ModePassphrase {
		if v.masterPassword == "" {
			return errors.New("cannot save vault: master password is empty")
		}
		enc, err := EncryptWithPassphrase(plainData, v.masterPassword)
		if err != nil {
			return err
		}
		encryptedData = enc
	} else {
		// Keyring mode
		identity, err := v.keyringMgr.GetIdentity()
		if err != nil {
			// Generate new identity if not found
			if errors.Is(err, ErrKeyringNotFound) {
				newId, genErr := age.GenerateX25519Identity()
				if genErr != nil {
					return genErr
				}
				if setErr := v.keyringMgr.SetIdentity(newId); setErr != nil {
					// Keyring not writable, fallback to session-only
					v.mode = ModeSessionOnly
					v.warnings = append(v.warnings, "No system keyring available. Passwords will be kept for this session only.")
					return nil
				}
				identity = newId
			} else {
				v.mode = ModeSessionOnly
				v.warnings = append(v.warnings, "Keyring is unavailable. Passwords will be kept for this session only.")
				return nil
			}
		}

		var buf bytes.Buffer
		w, err := age.Encrypt(&buf, identity.Recipient())
		if err != nil {
			return fmt.Errorf("failed to initialize age encryption: %w", err)
		}
		if _, err := w.Write(plainData); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		encryptedData = buf.Bytes()
	}

	if err := atomicWriteFile(v.SecretsFilePath(), encryptedData); err != nil {
		return err
	}

	v.dirty = false
	return nil
}

func (v *Vault) scheduleSave() {
	v.dirty = true
	if v.saveTimer != nil {
		v.saveTimer.Stop()
	}
	v.saveTimer = time.AfterFunc(500*time.Millisecond, func() {
		_ = v.Save()
	})
}

func (v *Vault) GetGlobal(key string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.global[key]
}

func (v *Vault) SetGlobal(key, val string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.global == nil {
		v.global = make(map[string]string)
	}
	if val == "" {
		delete(v.global, key)
	} else {
		v.global[key] = val
	}
	v.scheduleSave()
}

func (v *Vault) GetConnection(connID, key string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if c, ok := v.connections[connID]; ok {
		return c[key]
	}
	return ""
}

func (v *Vault) SetConnection(connID, key, val string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.connections == nil {
		v.connections = make(map[string]map[string]string)
	}
	c, ok := v.connections[connID]
	if !ok {
		c = make(map[string]string)
		v.connections[connID] = c
	}
	if val == "" {
		delete(c, key)
	} else {
		c[key] = val
	}
	v.scheduleSave()
}

func (v *Vault) DeleteConnection(connID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.connections, connID)
	v.scheduleSave()
}

func (v *Vault) DuplicateConnection(oldID, newID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if src, ok := v.connections[oldID]; ok {
		dst := make(map[string]string, len(src))
		for k, val := range src {
			dst[k] = val
		}
		if v.connections == nil {
			v.connections = make(map[string]map[string]string)
		}
		v.connections[newID] = dst
		v.scheduleSave()
	}
}

func (v *Vault) GarbageCollect(validConnIDs map[string]bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	modified := false
	for connID := range v.connections {
		if !validConnIDs[connID] {
			delete(v.connections, connID)
			modified = true
		}
	}
	if modified {
		v.scheduleSave()
	}
}

func (v *Vault) SetMasterPassword(oldPwd, newPwd string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.mode == ModePassphrase && v.masterPassword != "" && v.masterPassword != oldPwd {
		return ErrWrongPassword
	}

	v.masterPassword = newPwd
	v.mode = ModePassphrase

	// Save immediately with new password
	return v.saveUnlocked()
}

func (v *Vault) DisableMasterPassword(currentPwd string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.mode == ModePassphrase && v.masterPassword != "" && v.masterPassword != currentPwd {
		return ErrWrongPassword
	}

	// Try switching to keyring mode
	v.masterPassword = ""
	v.mode = ModeKeyring

	return v.saveUnlocked()
}

func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	v.masterPassword = password
	v.mode = ModePassphrase
	v.mu.Unlock()

	return v.Load()
}

func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.masterPassword = ""
	v.mode = ModeLocked
	v.global = make(map[string]string)
	v.connections = make(map[string]map[string]string)
}

func (v *Vault) saveUnlocked() error {
	if v.readOnly {
		return nil
	}

	doc := VaultDocument{
		SchemaVersion: 1,
		Global:        v.global,
		Connections:   v.connections,
	}

	plainData, err := toml.Marshal(doc)
	if err != nil {
		return err
	}

	var encryptedData []byte
	if v.mode == ModePassphrase {
		enc, err := EncryptWithPassphrase(plainData, v.masterPassword)
		if err != nil {
			return err
		}
		encryptedData = enc
	} else {
		// Keyring mode
		identity, err := v.keyringMgr.GetIdentity()
		if err != nil {
			newId, genErr := age.GenerateX25519Identity()
			if genErr != nil {
				return genErr
			}
			if setErr := v.keyringMgr.SetIdentity(newId); setErr != nil {
				v.mode = ModeSessionOnly
				return nil
			}
			identity = newId
		}

		var buf bytes.Buffer
		w, err := age.Encrypt(&buf, identity.Recipient())
		if err != nil {
			return err
		}
		if _, err := w.Write(plainData); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		encryptedData = buf.Bytes()
	}

	return atomicWriteFile(v.SecretsFilePath(), encryptedData)
}

func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.saveTimer != nil {
		v.saveTimer.Stop()
	}
	return nil
}

func atomicWriteFile(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	_ = os.MkdirAll(dir, 0700)

	tmpFile, err := os.CreateTemp(dir, "secrets-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
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
