package services

import (
	"errors"
	"sync"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/secrets"
)

type ConnectionEdit struct {
	config.Connection
	Password      string `json:"password"`
	KeyPassphrase string `json:"keyPassphrase"`
	SocksPassword string `json:"socksPassword"`
	JumpPassword  string `json:"jumpPassword"`
}

type Settings struct {
	General     config.GeneralConfig     `json:"general"`
	Appearance  config.AppearanceConfig  `json:"appearance"`
	Prompts     config.PromptsConfig     `json:"prompts"`
	Terminal    config.TerminalConfig    `json:"terminal"`
	SFTPBrowser config.SFTPBrowserConfig `json:"sftpBrowser"`
	LocalShell  config.LocalShellConfig  `json:"localShell"`
	Network     config.NetworkConfig     `json:"network"`
	Variables   []config.VariableConfig  `json:"variables"`
	Macros      config.MacrosConfig      `json:"macros"`
	KeePass     config.KeePassConfig     `json:"keepass"`
	Keybindings config.KeybindingsConfig `json:"keybindings"`
}

type ConfigService struct {
	mu          sync.Mutex
	store       *config.Store
	vault       *secrets.Vault
	treeManager *config.TreeManager
}

func NewConfigService(store *config.Store, vault *secrets.Vault, treeManager *config.TreeManager) *ConfigService {
	return &ConfigService{
		store:       store,
		vault:       vault,
		treeManager: treeManager,
	}
}

func (cs *ConfigService) GetTree() config.TreeNode {
	tree := cs.treeManager.BuildTree()
	if tree == nil {
		return config.TreeNode{ID: "", Name: "My Connections", IsGroup: true}
	}
	return *tree
}

func (cs *ConfigService) GetConnection(id string) (config.Connection, error) {
	cfg := cs.store.Config()
	for _, c := range cfg.Connections {
		if c.ID == id {
			return c, nil
		}
	}
	return config.Connection{}, config.ErrNodeNotFound
}

func (cs *ConfigService) GetConnectionForEdit(id string) (ConnectionEdit, error) {
	conn, err := cs.GetConnection(id)
	if err != nil {
		return ConnectionEdit{}, err
	}

	edit := ConnectionEdit{Connection: conn}
	if cs.vault != nil {
		edit.Password = cs.vault.GetConnection(id, "password")
		edit.KeyPassphrase = cs.vault.GetConnection(id, "key_passphrase")
		edit.SocksPassword = cs.vault.GetConnection(id, "socks_password")
		edit.JumpPassword = cs.vault.GetConnection(id, "jump_password")
	}
	return edit, nil
}

func (cs *ConfigService) SaveConnection(id string, edit ConnectionEdit) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cfg := cs.store.Config()
	found := false
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == id {
			cfg.Connections[i] = edit.Connection
			found = true
			break
		}
	}
	if !found {
		// New connection
		cfg.Connections = append(cfg.Connections, edit.Connection)
	}

	// Save secrets in vault
	if cs.vault != nil {
		cs.vault.SetConnection(id, "password", edit.Password)
		cs.vault.SetConnection(id, "key_passphrase", edit.KeyPassphrase)
		cs.vault.SetConnection(id, "socks_password", edit.SocksPassword)
		cs.vault.SetConnection(id, "jump_password", edit.JumpPassword)
	}

	cs.store.MarkDirty()
	return nil
}

func (cs *ConfigService) AddNode(parentID string, isGroup bool, name, title string) (string, error) {
	if isGroup {
		grp, err := cs.treeManager.AddGroup(parentID, name)
		if err != nil {
			return "", err
		}
		return grp.ID, nil
	}
	conn, err := cs.treeManager.AddConnection(parentID, name, title)
	if err != nil {
		return "", err
	}
	return conn.ID, nil
}

func (cs *ConfigService) RenameNode(id, name, title string) error {
	return cs.treeManager.RenameNode(id, name, title)
}

func (cs *ConfigService) DeleteNodes(ids []string) error {
	return cs.treeManager.DeleteNodes(ids)
}

func (cs *ConfigService) MoveNodes(ids []string, newParent string) error {
	return cs.treeManager.MoveNodes(ids, newParent)
}

func (cs *ConfigService) CopyNodes(ids []string) error {
	cs.treeManager.CopyNodes(ids)
	return nil
}

func (cs *ConfigService) CutNodes(ids []string) error {
	return cs.treeManager.CutNodes(ids)
}

func (cs *ConfigService) PasteNodes(parent string) error {
	return cs.treeManager.PasteNodes(parent)
}

func (cs *ConfigService) CloneNode(id string) (string, error) {
	return cs.treeManager.CloneNode(id)
}

func (cs *ConfigService) SetProtected(ids []string, on bool) error {
	return cs.treeManager.SetProtected(ids, on)
}

func (cs *ConfigService) SetFavourite(id string, on bool) error {
	return cs.treeManager.SetFavourite(id, on)
}

func (cs *ConfigService) SetDescription(id, text string) error {
	return cs.treeManager.SetDescription(id, text)
}

func (cs *ConfigService) GetSettings() Settings {
	cfg := cs.store.Config()
	return Settings{
		General:     cfg.General,
		Appearance:  cfg.Appearance,
		Prompts:     cfg.Prompts,
		Terminal:    cfg.Terminal,
		SFTPBrowser: cfg.SFTPBrowser,
		LocalShell:  cfg.LocalShell,
		Network:     cfg.Network,
		Variables:   cfg.Variables,
		Macros:      cfg.Macros,
		KeePass:     cfg.KeePass,
		Keybindings: cfg.Keybindings,
	}
}

// SaveSettings applies the preferences dialog values. Invalid values are rejected
// with a readable error and the previous settings are kept.
func (cs *ConfigService) SaveSettings(s Settings) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cfg := cs.store.Config()
	backup := cs.GetSettings()
	applySettings(cfg, s)
	if res := config.Validate(cfg); res.HasErrors() {
		applySettings(cfg, backup)
		return res.Error()
	}

	cs.store.MarkDirty()
	return nil
}

func applySettings(cfg *config.Config, s Settings) {
	cfg.General = s.General
	cfg.Appearance = s.Appearance
	cfg.Prompts = s.Prompts
	cfg.Terminal = s.Terminal
	cfg.SFTPBrowser = s.SFTPBrowser
	cfg.LocalShell = s.LocalShell
	cfg.Network = s.Network
	cfg.Variables = s.Variables
	cfg.Macros = s.Macros
	cfg.KeePass = s.KeePass
	cfg.Keybindings = s.Keybindings
}

// GetUIState returns the remembered window/panel layout.
func (cs *ConfigService) GetUIState() config.UIState {
	return cs.store.LoadUIState()
}

// SaveUIState persists the window/panel layout (state/ui.toml).
func (cs *ConfigService) SaveUIState(st config.UIState) error {
	return cs.store.SaveUIState(st)
}

func (cs *ConfigService) ResetSettings() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	def := config.Default()
	cfg := cs.store.Config()
	cfg.General = def.General
	cfg.Appearance = def.Appearance
	cfg.Prompts = def.Prompts
	cfg.Terminal = def.Terminal
	cfg.SFTPBrowser = def.SFTPBrowser
	cfg.LocalShell = def.LocalShell
	cfg.Network = def.Network
	cfg.Variables = def.Variables
	cfg.Macros = def.Macros
	cfg.KeePass = def.KeePass
	cfg.Keybindings = def.Keybindings

	cs.store.MarkDirty()
	return nil
}

func (cs *ConfigService) Save() error {
	return cs.store.Save()
}

func (cs *ConfigService) HasUnsavedChanges() bool {
	return cs.store.HasUnsavedChanges()
}

func (cs *ConfigService) Search(query, field, fromID string, backwards bool) string {
	return cs.treeManager.Search(query, field, fromID, backwards)
}

func (cs *ConfigService) ExportNodes(ids []string, path string, includeSecrets bool, passphrase string, bulk []config.BulkChanges) error {
	return config.ExportNodes(cs.store.Config(), cs.vault, ids, path, includeSecrets, passphrase, bulk)
}

func (cs *ConfigService) ImportNodes(parent, path, passphrase string) (config.ImportReport, error) {
	report, err := config.ImportNodes(cs.store.Config(), cs.vault, parent, path, passphrase)
	if err != nil {
		return config.ImportReport{}, err
	}
	cs.store.MarkDirty()
	return *report, nil
}

func (cs *ConfigService) ExportFull(path string, debug bool, includeSecrets bool, passphrase string) error {
	return config.ExportFull(cs.store.Config(), cs.vault, path, debug, includeSecrets, passphrase)
}

func (cs *ConfigService) GetVaultStatus() string {
	if cs.vault == nil {
		return "keyring"
	}
	return string(cs.vault.Status())
}

func (cs *ConfigService) SetMasterPassword(oldPwd, newPwd string) error {
	if cs.vault == nil {
		return errors.New("vault not initialized")
	}
	err := cs.vault.SetMasterPassword(oldPwd, newPwd)
	if err == nil {
		cs.store.Config().General.ProtectWithPassword = true
		cs.store.MarkDirty()
	}
	return err
}

func (cs *ConfigService) DisableMasterPassword(currentPwd string) error {
	if cs.vault == nil {
		return errors.New("vault not initialized")
	}
	err := cs.vault.DisableMasterPassword(currentPwd)
	if err == nil {
		cs.store.Config().General.ProtectWithPassword = false
		cs.store.MarkDirty()
	}
	return err
}

func (cs *ConfigService) ApplyBulkEdit(connIDs []string, rules []config.BulkEditRule) (config.BulkEditResult, error) {
	if cs.store == nil {
		return config.BulkEditResult{}, errors.New("store not initialized")
	}
	return cs.store.ApplyBulkEdit(connIDs, rules)
}
