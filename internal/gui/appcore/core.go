// Package appcore builds LeanTerm's backend for a GUI: configuration,
// secrets, the event bus, sessions, clusters and KeePass, wired as the
// application needs them. It has no GUI of its own; the GUI subscribes to
// Bus and answers prompts through session.Prompter.
package appcore

import (
	"fmt"
	"path/filepath"

	"github.com/Suvukaraz/leanterm/internal/cluster"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/events"
	"github.com/Suvukaraz/leanterm/internal/keepass"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/Suvukaraz/leanterm/internal/session"
)

// Options select the configuration (the command line flags of LeanTerm).
type Options struct {
	// ConfigDir is the configuration folder ("" = the default).
	ConfigDir string
	// MasterPassword unlocks secrets protected with a password.
	MasterPassword string
	ReadOnly       bool
	NoBackup       bool
}

// Core is the running backend.
type Core struct {
	Store    *config.Store
	Vault    *secrets.Vault
	Tree     *config.TreeManager
	Bus      *events.Bus
	Sessions *session.SessionManager
	Clusters *cluster.Manager
	KeePass  *keepass.Client
}

// Open loads the configuration and the secrets and starts the managers.
func Open(opts Options) (*Core, error) {
	store, err := config.NewStore(config.StoreOptions{ConfigDir: opts.ConfigDir, NoBackup: opts.NoBackup, ReadOnly: opts.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("configuration: %w", err)
	}
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("loading the configuration: %w", err)
	}
	vault := secrets.NewVault(secrets.VaultOptions{
		ConfigDir:      filepath.Dir(store.ConfigPath()),
		ProtectWithPwd: store.Config().General.ProtectWithPassword,
		MasterPassword: opts.MasterPassword,
		ReadOnly:       opts.ReadOnly,
		NoBackup:       opts.NoBackup,
	})
	// As in the web GUI: without readable secrets connections still open and
	// ask for passwords.
	_ = vault.Load()

	c := &Core{Store: store, Vault: vault, Tree: config.NewTreeManager(store, vault), Bus: events.New()}
	c.Clusters = cluster.NewManager(store, c.Bus)
	c.Sessions = session.NewSessionManager(store, vault, c.Bus)
	c.Sessions.SetClusterManager(c.Clusters)
	c.KeePass = keepass.NewClient(keepass.Options{
		Config:         store.Config().KeePass,
		MasterPassword: vault.GetGlobal("keepass_master_password"),
	})
	c.Sessions.SetKeePassClient(c.KeePass)
	store.OnReload = func() { c.Bus.Emit(events.SettingsChanged, nil) }
	return c, nil
}

// FindConnection looks a saved connection up by id or name.
func (c *Core) FindConnection(idOrName string) (config.Connection, bool) {
	for _, conn := range c.Store.Config().Connections {
		if conn.ID == idOrName || conn.Name == idOrName || conn.Title == idOrName {
			return conn, true
		}
	}
	return config.Connection{}, false
}

// Close ends all sessions.
func (c *Core) Close() {
	for _, s := range c.Sessions.List() {
		_ = c.Sessions.Close(s.TmpID)
	}
}
