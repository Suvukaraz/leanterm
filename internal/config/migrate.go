package config

import (
	"fmt"
	"strings"
)

// MigrationStep defines a single version upgrade step.
type MigrationStep struct {
	FromVersion int
	ToVersion   int
	Migrate     func(cfg *Config) error
}

// migrations contains ordered upgrade steps for LeanTerm's configuration.
var migrations = []MigrationStep{
	{FromVersion: 1, ToVersion: 2, Migrate: migrateV1toV2},
}

// migrateV1toV2 switches configs that still carry the old v1 defaults to the
// new defaults (dark UI, readable terminal font). Values the user changed are kept.
func migrateV1toV2(cfg *Config) error {
	if cfg.General.Theme == "default" {
		cfg.General.Theme = "asbru-dark"
	}
	if cfg.General.ColorScheme == "" {
		cfg.General.ColorScheme = "dark"
	}
	if cfg.Terminal.Look.Font.Size <= 9 && strings.EqualFold(cfg.Terminal.Look.Font.Family, "Monospace") {
		cfg.Terminal.Look.Font.Size = DefaultTerminalFontSize
	}
	return nil
}

// MigrateUpgrades applies all migrations from cfg.SchemaVersion up to CurrentSchemaVersion.
func MigrateUpgrades(cfg *Config) error {
	if cfg.SchemaVersion > CurrentSchemaVersion {
		return fmt.Errorf("configuration schema_version %d is newer than supported version %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}

	for _, step := range migrations {
		if cfg.SchemaVersion == step.FromVersion {
			if err := step.Migrate(cfg); err != nil {
				return fmt.Errorf("failed to migrate configuration from v%d to v%d: %w", step.FromVersion, step.ToVersion, err)
			}
			cfg.SchemaVersion = step.ToVersion
		}
	}

	return nil
}
