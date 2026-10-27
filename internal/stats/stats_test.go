package stats

import (
	"os"
	"testing"
	"time"
)

func TestStatsManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "leanterm-stats-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// 1. Record start for conn-1
	mgr.RecordStart("conn-1", "Server 1", "ssh")

	st := mgr.GetStats("conn-1")
	if st.TotalConnections != 1 {
		t.Fatalf("expected 1 connection, got %d", st.TotalConnections)
	}
	if st.FirstStart.IsZero() || st.LastStart.IsZero() {
		t.Fatal("expected non-zero start times")
	}

	hist := mgr.GetHistory()
	if len(hist) != 1 || hist[0].ID != "conn-1" || hist[0].Name != "Server 1" {
		t.Fatalf("unexpected history: %v", hist)
	}

	// 2. Record stop with duration
	mgr.RecordStop("conn-1", 15*time.Second)
	st = mgr.GetStats("conn-1")
	if st.TotalSeconds != 15 {
		t.Fatalf("expected 15 seconds, got %d", st.TotalSeconds)
	}

	// 3. Test persistence across manager restart
	mgr2 := NewManager(tmpDir)
	st2 := mgr2.GetStats("conn-1")
	if st2.TotalConnections != 1 || st2.TotalSeconds != 15 {
		t.Fatalf("reloaded stats mismatch: %+v", st2)
	}

	hist2 := mgr2.GetHistory()
	if len(hist2) != 1 {
		t.Fatalf("reloaded history mismatch: %+v", hist2)
	}

	// 4. Test ResetStats
	if err := mgr2.ResetStats("conn-1"); err != nil {
		t.Fatalf("reset stats error: %v", err)
	}
	if mgr2.GetStats("conn-1").TotalConnections != 0 {
		t.Fatal("expected 0 stats after reset")
	}
}
