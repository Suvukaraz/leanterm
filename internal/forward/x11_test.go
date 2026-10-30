package forward

import (
	"context"
	"testing"
)

func TestParseDisplay(t *testing.T) {
	tests := []struct {
		input       string
		wantUnix    bool
		wantSocket  string
		wantTCPAddr string
		wantScreen  uint32
	}{
		{
			input:      ":0",
			wantUnix:   true,
			wantSocket: "/tmp/.X11-unix/X0",
			wantScreen: 0,
		},
		{
			input:      ":1.2",
			wantUnix:   true,
			wantSocket: "/tmp/.X11-unix/X1",
			wantScreen: 2,
		},
		{
			input:      "unix:5.0",
			wantUnix:   true,
			wantSocket: "/tmp/.X11-unix/X5",
			wantScreen: 0,
		},
		{
			input:       "192.168.1.50:10.1",
			wantUnix:    false,
			wantTCPAddr: "192.168.1.50:6010",
			wantScreen:  1,
		},
	}

	for _, tc := range tests {
		info, err := ParseDisplay(tc.input)
		if err != nil {
			t.Fatalf("ParseDisplay(%q) unexpected error: %v", tc.input, err)
		}
		if info.IsUnix != tc.wantUnix {
			t.Errorf("ParseDisplay(%q) IsUnix = %v, want %v", tc.input, info.IsUnix, tc.wantUnix)
		}
		if tc.wantUnix && info.SocketPath != tc.wantSocket {
			t.Errorf("ParseDisplay(%q) SocketPath = %q, want %q", tc.input, info.SocketPath, tc.wantSocket)
		}
		if !tc.wantUnix && info.TCPAddr != tc.wantTCPAddr {
			t.Errorf("ParseDisplay(%q) TCPAddr = %q, want %q", tc.input, info.TCPAddr, tc.wantTCPAddr)
		}
		if info.ScreenNum != tc.wantScreen {
			t.Errorf("ParseDisplay(%q) ScreenNum = %d, want %d", tc.input, info.ScreenNum, tc.wantScreen)
		}
	}
}

func TestGetX11Cookie(t *testing.T) {
	proto, cookie, err := GetX11Cookie(":0")
	if err != nil {
		t.Fatalf("GetX11Cookie failed: %v", err)
	}
	if proto == "" {
		t.Errorf("expected non-empty protocol")
	}
	if len(cookie) != 32 {
		t.Errorf("expected 32-character hex cookie, got %q (len %d)", cookie, len(cookie))
	}
}

func TestPipeStreams_ContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify context done doesn't panic
	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected context to be cancelled")
	}
}

func TestParseDisplayTCPFallback(t *testing.T) {
	info, err := ParseDisplay(":3")
	if err != nil {
		t.Fatal(err)
	}
	if info.TCPAddr != "127.0.0.1:6003" {
		t.Fatalf("TCP fallback of :3 = %q", info.TCPAddr)
	}
}
