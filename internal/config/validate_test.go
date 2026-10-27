package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := Default()
	res := Validate(cfg)
	assert.False(t, res.HasErrors(), "default configuration must be valid: %v", res.Error())
}

func TestValidateEnumsAndPorts(t *testing.T) {
	cfg := Default()
	cfg.General.Layout = "invalid-layout"
	cfg.Appearance.ProtectedColor = "not-a-color"
	cfg.Network.Socks.Host = "127.0.0.1"
	cfg.Network.Socks.Port = 70000 // invalid port

	res := Validate(cfg)
	assert.True(t, res.HasErrors())
	assert.Len(t, res.Errors, 3)
}

func TestValidateTreeOrphansAndCycles(t *testing.T) {
	cfg := Default()
	cfg.Groups = []Group{
		{ID: "g1", Parent: "g2", Name: "Group 1"},
		{ID: "g2", Parent: "g1", Name: "Group 2"}, // Cycle!
		{ID: "g3", Parent: "non-existent", Name: "Group 3"}, // Orphan!
	}
	conn := DefaultConnection("c1", "non-existent-group", "Conn 1")
	cfg.Connections = []Connection{conn}

	res := Validate(cfg)
	assert.False(t, res.HasErrors(), "orphans and cycles should be resolved with warnings")
	assert.NotEmpty(t, res.Warnings)

	// Verify cycle broken and orphans reattached to root
	assert.Equal(t, "", cfg.Groups[2].Parent, "orphan group reattached to root")
	assert.Equal(t, "", cfg.Connections[0].Parent, "orphan connection reattached to root")
}

func TestValidateSSHForwardDuplicates(t *testing.T) {
	cfg := Default()
	conn := DefaultConnection("c1", "", "web01")
	conn.SSH.LocalForwards = []LocalForward{
		{BindAddress: "127.0.0.1", Port: 8080, Host: "localhost", HostPort: 80},
		{BindAddress: "127.0.0.1", Port: 8080, Host: "remote", HostPort: 8080}, // duplicate local port!
	}
	conn.SSH.RemoteForwards = []LocalForward{
		{BindAddress: "", Port: 9000, Host: "localhost", HostPort: 9000},
		{BindAddress: "", Port: 9000, Host: "localhost", HostPort: 9001}, // duplicate remote port warning
	}
	cfg.Connections = append(cfg.Connections, conn)

	res := Validate(cfg)
	assert.True(t, res.HasErrors())
	assert.NotEmpty(t, res.Warnings)
}
