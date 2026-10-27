package config

import "testing"

func TestMigrateV1toV2AppliesNewDefaults(t *testing.T) {
	cfg := Default()
	cfg.SchemaVersion = 1
	cfg.General.Theme = "default"
	cfg.General.ColorScheme = ""
	cfg.Terminal.Look.Font = FontConfig{Family: "Monospace", Size: 9}

	if err := MigrateUpgrades(cfg); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if cfg.SchemaVersion != 2 {
		t.Errorf("schema version = %d, want 2", cfg.SchemaVersion)
	}
	if cfg.General.Theme != "asbru-dark" || cfg.General.ColorScheme != "dark" {
		t.Errorf("theme = %q/%q, want asbru-dark/dark", cfg.General.Theme, cfg.General.ColorScheme)
	}
	if cfg.Terminal.Look.Font.Size != DefaultTerminalFontSize {
		t.Errorf("font size = %d, want %d", cfg.Terminal.Look.Font.Size, DefaultTerminalFontSize)
	}
}

func TestMigrateV1toV2KeepsUserChoices(t *testing.T) {
	cfg := Default()
	cfg.SchemaVersion = 1
	cfg.General.Theme = "asbru-color"
	cfg.General.ColorScheme = "light"
	cfg.Terminal.Look.Font = FontConfig{Family: "Fira Code", Size: 9}

	if err := MigrateUpgrades(cfg); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if cfg.General.Theme != "asbru-color" || cfg.General.ColorScheme != "light" {
		t.Errorf("user theme overwritten: %q/%q", cfg.General.Theme, cfg.General.ColorScheme)
	}
	if cfg.Terminal.Look.Font.Size != 9 {
		t.Errorf("custom font size overwritten: %d", cfg.Terminal.Look.Font.Size)
	}
}
