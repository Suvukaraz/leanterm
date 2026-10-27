package config

import (
	"testing"
)

func TestApplyBulkEdit_DirectAndRegex(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(StoreOptions{ConfigDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Add test connections
	store.cfg.Connections = []Connection{
		{
			ID:    "c1",
			Title: "web-prod-01",
			Host:  "10.0.1.10",
			Port:  22,
			Auth:  ConnectionAuth{User: "root"},
		},
		{
			ID:    "c2",
			Title: "web-prod-02",
			Host:  "10.0.1.11",
			Port:  22,
			Auth:  ConnectionAuth{User: "root"},
		},
		{
			ID:        "c3",
			Title:     "db-prod-01",
			Host:      "10.0.2.10",
			Port:      22,
			Protected: true, // protected, must NOT be modified
		},
	}

	// Rule 1: direct replace user -> "admin"
	// Rule 2: regex replace host "10.0.1.(\d+)" -> "192.168.1.$1"
	rules := []BulkEditRule{
		{
			Field:    "user",
			NewValue: "admin",
			UseRegex: false,
		},
		{
			Field:      "host",
			NewValue:   "192.168.1.$1",
			UseRegex:   true,
			MatchRegex: `10\.0\.1\.(\d+)`,
		},
	}

	res, err := store.ApplyBulkEdit([]string{"c1", "c2", "c3"}, rules)
	if err != nil {
		t.Fatalf("ApplyBulkEdit error: %v", err)
	}

	if res.ModifiedCount != 2 {
		t.Errorf("expected 2 modified connections, got %d", res.ModifiedCount)
	}

	// Check c1
	c1 := store.cfg.Connections[0]
	if c1.Auth.User != "admin" {
		t.Errorf("c1 user = %s, want admin", c1.Auth.User)
	}
	if c1.Host != "192.168.1.10" {
		t.Errorf("c1 host = %s, want 192.168.1.10", c1.Host)
	}

	// Check c3 (protected)
	c3 := store.cfg.Connections[2]
	if c3.Host != "10.0.2.10" {
		t.Errorf("protected c3 host was modified to %s", c3.Host)
	}
}
