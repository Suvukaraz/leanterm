package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/dlclark/regexp2"
	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	toml "github.com/pelletier/go-toml/v2"
)

const CurrentExportVersion = 1

type BulkChanges struct {
	Field        string `json:"field"`
	NewValue     string `json:"newValue"`
	UseRegex     bool   `json:"useRegex"`
	RegexPattern string `json:"regexPattern"`
}

type ExportDocument struct {
	ExportVersion int                 `toml:"export_version"`
	ExportedAt    string              `toml:"exported_at"`
	IsFullConfig  bool                `toml:"is_full_config,omitempty"`
	General       *GeneralConfig      `toml:"general,omitempty"`
	Appearance    *AppearanceConfig   `toml:"appearance,omitempty"`
	Prompts       *PromptsConfig      `toml:"prompts,omitempty"`
	Terminal      *TerminalConfig     `toml:"terminal,omitempty"`
	SFTPBrowser   *SFTPBrowserConfig  `toml:"sftp_browser,omitempty"`
	LocalShell    *LocalShellConfig   `toml:"local_shell,omitempty"`
	Network       *NetworkConfig      `toml:"network,omitempty"`
	Variables     []VariableConfig    `toml:"variables,omitempty"`
	Macros        *MacrosConfig       `toml:"macros,omitempty"`
	AutoClusters  []AutoClusterConfig `toml:"auto_clusters,omitempty"`
	KeePass       *KeePassConfig      `toml:"keepass,omitempty"`
	Keybindings   *KeybindingsConfig  `toml:"keybindings,omitempty"`
	Groups        []Group             `toml:"groups,omitempty"`
	Connections   []Connection        `toml:"connections,omitempty"`
	SecretsArmor  string              `toml:"secrets,omitempty"`
}

type ImportReport struct {
	GroupsImported      int      `json:"groupsImported"`
	ConnectionsImported int      `json:"connectionsImported"`
	Warnings            []string `json:"warnings"`
	R14Commands         []string `json:"r14Commands"`
}

// ExportNodes exports selected groups and connections to a TOML file.
func ExportNodes(cfg *Config, vault *secrets.Vault, ids []string, filePath string, includeSecrets bool, passphrase string, bulk []BulkChanges) error {
	doc := ExportDocument{
		ExportVersion: CurrentExportVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		IsFullConfig:  false,
		Groups:        []Group{},
		Connections:   []Connection{},
	}

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	for _, g := range cfg.Groups {
		if idMap[g.ID] {
			doc.Groups = append(doc.Groups, g)
		}
	}

	var exportedConns []Connection
	for _, c := range cfg.Connections {
		if idMap[c.ID] {
			cCopy := c
			// Apply bulk changes if specified
			for _, bc := range bulk {
				applyBulkChange(&cCopy, bc)
			}
			exportedConns = append(exportedConns, cCopy)
		}
	}
	doc.Connections = exportedConns

	// If passwords should be included, create an armored age block
	if includeSecrets && passphrase != "" && vault != nil {
		secretsDoc := secrets.VaultDocument{
			SchemaVersion: 1,
			Connections:   make(map[string]map[string]string),
		}

		for _, c := range exportedConns {
			connSecrets := make(map[string]string)
			if pw := vault.GetConnection(c.ID, "password"); pw != "" {
				connSecrets["password"] = pw
			}
			if kp := vault.GetConnection(c.ID, "key_passphrase"); kp != "" {
				connSecrets["key_passphrase"] = kp
			}
			if sp := vault.GetConnection(c.ID, "socks_password"); sp != "" {
				connSecrets["socks_password"] = sp
			}
			if jp := vault.GetConnection(c.ID, "jump_password"); jp != "" {
				connSecrets["jump_password"] = jp
			}
			for _, exp := range c.Expect {
				if exp.Hidden {
					if val := vault.GetConnection(c.ID, "expect:"+exp.ID); val != "" {
						connSecrets["expect:"+exp.ID] = val
					}
				}
			}
			for _, v := range c.Variables {
				if v.Hidden {
					if val := vault.GetConnection(c.ID, "variable:"+v.ID); val != "" {
						connSecrets["variable:"+v.ID] = val
					}
				}
			}
			if len(connSecrets) > 0 {
				secretsDoc.Connections[c.ID] = connSecrets
			}
		}

		armored, err := encryptToArmor(secretsDoc, passphrase)
		if err != nil {
			return fmt.Errorf("failed to encrypt exported secrets: %w", err)
		}
		doc.SecretsArmor = armored
	}

	data, err := toml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal export document: %w", err)
	}

	return os.WriteFile(filePath, data, 0600)
}

// ExportFull exports the entire configuration (and optionally secrets) to a TOML file.
func ExportFull(cfg *Config, vault *secrets.Vault, filePath string, debug bool, includeSecrets bool, passphrase string) error {
	doc := ExportDocument{
		ExportVersion: CurrentExportVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		IsFullConfig:  true,
		General:       &cfg.General,
		Appearance:    &cfg.Appearance,
		Prompts:       &cfg.Prompts,
		Terminal:      &cfg.Terminal,
		SFTPBrowser:   &cfg.SFTPBrowser,
		LocalShell:    &cfg.LocalShell,
		Network:       &cfg.Network,
		Variables:     cfg.Variables,
		Macros:        &cfg.Macros,
		AutoClusters:  cfg.AutoClusters,
		KeePass:       &cfg.KeePass,
		Keybindings:   &cfg.Keybindings,
		Groups:        cfg.Groups,
		Connections:   cfg.Connections,
	}

	if debug {
		// Strip personal/network identifiers
		doc.General.HelpURL = ""
		doc.Network = &NetworkConfig{Mode: "direct"}
		for i := range doc.Connections {
			doc.Connections[i].Host = "redacted"
			doc.Connections[i].Auth.User = "redacted"
			doc.Connections[i].Auth.KeyFile = ""
			doc.Connections[i].Network = ConnectionNetwork{Mode: "direct"}
		}
	} else if includeSecrets && passphrase != "" && vault != nil {
		secretsDoc := secrets.VaultDocument{
			SchemaVersion: 1,
			Global:        make(map[string]string),
			Connections:   make(map[string]map[string]string),
		}

		for _, c := range cfg.Connections {
			connSecrets := make(map[string]string)
			if pw := vault.GetConnection(c.ID, "password"); pw != "" {
				connSecrets["password"] = pw
			}
			if kp := vault.GetConnection(c.ID, "key_passphrase"); kp != "" {
				connSecrets["key_passphrase"] = kp
			}
			if len(connSecrets) > 0 {
				secretsDoc.Connections[c.ID] = connSecrets
			}
		}

		armored, err := encryptToArmor(secretsDoc, passphrase)
		if err != nil {
			return err
		}
		doc.SecretsArmor = armored
	}

	data, err := toml.Marshal(doc)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

// ImportNodes imports groups and connections from a LeanTerm export TOML file.
func ImportNodes(targetCfg *Config, vault *secrets.Vault, parentGroupID, filePath string, passphrase string) (*ImportReport, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read import file: %w", err)
	}

	var doc ExportDocument
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse import file: %w", err)
	}

	if doc.ExportVersion == 0 {
		return nil, errors.New("invalid or unrecognized export file (only LeanTerm format is supported)")
	}

	report := &ImportReport{
		Warnings:    []string{},
		R14Commands: []string{},
	}

	// Decrypt secrets if present and passphrase provided
	var importedSecrets map[string]map[string]string
	if doc.SecretsArmor != "" && passphrase != "" {
		decDoc, err := decryptFromArmor(doc.SecretsArmor, passphrase)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to decrypt secrets (wrong passphrase): %v. Imported without passwords.", err))
		} else {
			importedSecrets = decDoc.Connections
		}
	} else if doc.SecretsArmor != "" && passphrase == "" {
		report.Warnings = append(report.Warnings, "Import file contains encrypted secrets, but no passphrase was provided. Imported without passwords.")
	}

	// Build map of existing IDs to prevent collisions
	existingIDs := make(map[string]bool)
	for _, g := range targetCfg.Groups {
		existingIDs[g.ID] = true
	}
	for _, c := range targetCfg.Connections {
		existingIDs[c.ID] = true
	}

	idRemap := make(map[string]string)

	// Map top-level parent IDs to parentGroupID
	rootGroupIDs := make(map[string]bool)
	for _, g := range doc.Groups {
		if g.Parent == "" {
			rootGroupIDs[g.ID] = true
		}
	}

	// 1. Import groups
	for _, g := range doc.Groups {
		oldID := g.ID
		newID := oldID
		if existingIDs[oldID] || newID == "" {
			newID = uuid.NewString()
			idRemap[oldID] = newID
		}

		gCopy := g
		gCopy.ID = newID

		if gCopy.Parent == "" {
			gCopy.Parent = parentGroupID
		} else if mapped, ok := idRemap[gCopy.Parent]; ok {
			gCopy.Parent = mapped
		}

		existingIDs[newID] = true
		targetCfg.Groups = append(targetCfg.Groups, gCopy)
		report.GroupsImported++
	}

	// 2. Import connections
	for _, c := range doc.Connections {
		oldID := c.ID
		newID := oldID
		if existingIDs[oldID] || newID == "" {
			newID = uuid.NewString()
			idRemap[oldID] = newID
		}

		cCopy := c
		cCopy.ID = newID

		if cCopy.Parent == "" || rootGroupIDs[cCopy.Parent] {
			if cCopy.Parent == "" {
				cCopy.Parent = parentGroupID
			} else if mapped, ok := idRemap[cCopy.Parent]; ok {
				cCopy.Parent = mapped
			}
		} else if mapped, ok := idRemap[cCopy.Parent]; ok {
			cCopy.Parent = mapped
		}

		// Remap secrets if decrypted
		if importedSecrets != nil && vault != nil {
			if secMap, ok := importedSecrets[oldID]; ok {
				for k, v := range secMap {
					vault.SetConnection(newID, k, v)
				}
			}
		}

		// Scan for R14 dangerous commands (<CMD:...>, pre_exec, post_exec)
		r14Found := checkR14Commands(&cCopy)
		if len(r14Found) > 0 {
			report.R14Commands = append(report.R14Commands, fmt.Sprintf("Connection '%s': %s", cCopy.Name, strings.Join(r14Found, ", ")))
		}

		existingIDs[newID] = true
		targetCfg.Connections = append(targetCfg.Connections, cCopy)
		report.ConnectionsImported++
	}

	return report, nil
}

func encryptToArmor(secretsDoc secrets.VaultDocument, passphrase string) (string, error) {
	plain, err := toml.Marshal(secretsDoc)
	if err != nil {
		return "", err
	}

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return "", err
	}
	recipient.SetWorkFactor(secrets.DefaultWorkFactor)

	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)
	w, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return "", err
	}

	if _, err := w.Write(plain); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	if err := armorWriter.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func decryptFromArmor(armored, passphrase string) (*secrets.VaultDocument, error) {
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}

	armorReader := armor.NewReader(strings.NewReader(armored))
	r, err := age.Decrypt(armorReader, identity)
	if err != nil {
		return nil, err
	}

	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var doc secrets.VaultDocument
	if err := toml.Unmarshal(plain, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func applyBulkChange(c *Connection, bc BulkChanges) {
	applyField := func(val *string) {
		if !bc.UseRegex {
			*val = bc.NewValue
			return
		}
		if re, err := regexp2.Compile(bc.RegexPattern, 0); err == nil {
			if res, err := re.Replace(*val, bc.NewValue, -1, -1); err == nil {
				*val = res
			}
		}
	}

	switch strings.ToLower(bc.Field) {
	case "host":
		applyField(&c.Host)
	case "user", "username":
		applyField(&c.Auth.User)
	case "title":
		applyField(&c.Title)
	case "description":
		applyField(&c.Description)
	}
}

func checkR14Commands(c *Connection) []string {
	var cmds []string

	scanField := func(field, val string) {
		if strings.Contains(val, "<CMD:") {
			cmds = append(cmds, fmt.Sprintf("<CMD:...> in %s", field))
		}
	}

	scanField("Host", c.Host)
	scanField("User", c.Auth.User)
	scanField("Title", c.Title)
	scanField("Description", c.Description)

	for _, pre := range c.PreExec {
		if pre.Command != "" {
			cmds = append(cmds, fmt.Sprintf("PreExec: '%s'", pre.Command))
		}
	}
	for _, post := range c.PostExec {
		if post.Command != "" {
			cmds = append(cmds, fmt.Sprintf("PostExec: '%s'", post.Command))
		}
	}

	return cmds
}
