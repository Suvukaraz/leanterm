package app

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestParseCLI(t *testing.T) {
	// Redirect stderr to capture warnings
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	args := []string{
		"--config-dir=/custom/config",
		"-start-shell",
		"--start-uuid=uuid1:cluster1",
		"--start-uuid=uuid2",
		"--password", "secret123",
		"--readonly",
		"--verbose",
		"--invalid-flag",
	}

	flags := ParseCLI(args)
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	stderrOutput := buf.String()

	assert.Equal(t, "/custom/config", flags.ConfigDir)
	assert.True(t, flags.StartShell)
	assert.Equal(t, []string{"uuid1:cluster1", "uuid2"}, flags.StartUUIDs)
	assert.Equal(t, "secret123", flags.Password)
	assert.True(t, flags.ReadOnly)
	assert.True(t, flags.Verbose)
	assert.Contains(t, flags.UnknownArgs, "--invalid-flag")
	assert.Contains(t, stderrOutput, "INFO: bad start up parameter --invalid-flag")
}

func TestParseCLIHelp(t *testing.T) {
	flags := ParseCLI([]string{"--help"})
	assert.True(t, flags.Help)

	flags = ParseCLI([]string{"-h"})
	assert.True(t, flags.Help)
}

func TestDumpAndListUUIDs(t *testing.T) {
	cfg := &config.Config{
		Groups: []config.Group{
			{ID: "grp-1", Name: "Servers", Parent: ""},
		},
		Connections: []config.Connection{
			{ID: "conn-1", Name: "Web Server", Host: "192.168.1.10", Port: 22, Method: "ssh", Parent: "grp-1"},
		},
	}

	// Test DumpUUID
	err := DumpUUID(cfg, "conn-1")
	assert.NoError(t, err)

	err = DumpUUID(cfg, "non-existent")
	assert.Error(t, err)

	// Test ListUUIDs
	ListUUIDs(cfg)
}

func TestParseEditFiles(t *testing.T) {
	flags := ParseCLI([]string{"--edit=/etc/hosts", "--edit", "notes.txt", "--start-shell"})
	assert.Equal(t, []string{"/etc/hosts", "notes.txt"}, flags.EditFiles)
	assert.True(t, flags.StartShell)
}
