package cluster

import (
	"testing"

	"github.com/Suvukaraz/leanterm/internal/config"
)

type mockEmitter struct {
	events []string
}

func (m *mockEmitter) Emit(event string, data any) {
	m.events = append(m.events, event)
}

func TestClusterManager_Lifecycle(t *testing.T) {
	emitter := &mockEmitter{}
	mgr := NewManager(nil, emitter)

	_ = mgr.Create("Production")
	_ = mgr.Create("Staging")

	mgr.Add("sess-1", "Production")
	mgr.Add("sess-2", "Production")
	mgr.Add("sess-3", "Staging")

	if mgr.GetCluster("sess-1") != "Production" {
		t.Errorf("sess-1 cluster = %s, want Production", mgr.GetCluster("sess-1"))
	}
	if len(mgr.GetMembers("Production")) != 2 {
		t.Errorf("Production members = %d, want 2", len(mgr.GetMembers("Production")))
	}

	// Move sess-1 to Staging
	mgr.Add("sess-1", "Staging")
	if mgr.GetCluster("sess-1") != "Staging" {
		t.Errorf("sess-1 cluster = %s, want Staging", mgr.GetCluster("sess-1"))
	}
	if len(mgr.GetMembers("Production")) != 1 {
		t.Errorf("Production members = %d, want 1", len(mgr.GetMembers("Production")))
	}

	// Rename Staging -> Test
	_ = mgr.Rename("Staging", "Test")
	if mgr.GetCluster("sess-1") != "Test" {
		t.Errorf("sess-1 cluster after rename = %s, want Test", mgr.GetCluster("sess-1"))
	}

	// Remove sess-1
	mgr.Remove("sess-1")
	if mgr.GetCluster("sess-1") != "" {
		t.Errorf("sess-1 should be unclustered, got %s", mgr.GetCluster("sess-1"))
	}

	// Delete Test cluster
	mgr.Delete("Test")
	if len(mgr.List()) != 1 {
		t.Errorf("clusters count = %d, want 1", len(mgr.List()))
	}
}

func TestMatchAutoClusters(t *testing.T) {
	autoRules := []config.AutoClusterConfig{
		{
			Name:       "WebServers",
			MatchTitle: `^web-\d+`,
			MatchHost:  `^10\.0\.`,
		},
		{
			Name:             "DBServers",
			MatchDescription: `Database`,
		},
	}

	conn1 := config.Connection{
		Title: "web-01",
		Host:  "10.0.1.20",
	}
	if m := MatchAutoClusters(conn1, autoRules); m != "WebServers" {
		t.Errorf("conn1 matched %s, want WebServers", m)
	}

	conn2 := config.Connection{
		Title: "web-01",
		Host:  "192.168.1.20", // host doesn't match
	}
	if m := MatchAutoClusters(conn2, autoRules); m != "" {
		t.Errorf("conn2 matched %s, want empty", m)
	}

	conn3 := config.Connection{
		Title:       "srv-db",
		Description: "Main Database cluster node",
	}
	if m := MatchAutoClusters(conn3, autoRules); m != "DBServers" {
		t.Errorf("conn3 matched %s, want DBServers", m)
	}
}
