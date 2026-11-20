package term

import (
	"bytes"
	"testing"
)

// Ported from frontend/src/lib/terminal/mouseReports.test.ts.

func press(f func(*MouseReport)) MouseReport {
	r := MouseReport{Kind: MousePress, Button: 0, Col: 4, Row: 1}
	if f != nil {
		f(&r)
	}
	return r
}

func TestModeTrackerSplitAcrossChunks(t *testing.T) {
	var m modeTracker
	m.feed([]byte("hello \x1b[?100"))
	if m.Mouse != MouseNone {
		t.Fatal("tracking before the sequence ended")
	}
	m.feed([]byte("2;1006h"))
	if m.Mouse != MouseButton || !m.SGR {
		t.Fatalf("after 1002;1006h: %+v", m.Modes)
	}
	m.feed([]byte("\x1b[?1000l"))
	if m.Mouse != MouseButton {
		t.Fatal("resetting another mode turned tracking off")
	}
	m.feed([]byte("\x1b[?1002l\x1b[?1006l"))
	if m.Mouse != MouseNone || m.SGR {
		t.Fatalf("after reset: %+v", m.Modes)
	}
}

func TestModeTrackerPrivateOnlyAndRIS(t *testing.T) {
	var m modeTracker
	m.feed([]byte("\x1b[1000h"))
	if m.Mouse != MouseNone {
		t.Fatal("non-private mode counted")
	}
	m.feed([]byte("\x1b[?1003h\x1b[?1006h\x1b[?1049h"))
	if m.Mouse != MouseAny || !m.SGR || !m.AltScreen {
		t.Fatalf("%+v", m.Modes)
	}
	m.feed([]byte("\x1bc"))
	if m.Modes != (Modes{}) {
		t.Fatalf("after RIS: %+v", m.Modes)
	}
}

func TestEncodeMouse(t *testing.T) {
	if EncodeMouse(Modes{}, press(nil)) != nil {
		t.Fatal("report without tracking")
	}
	var m modeTracker
	m.feed([]byte("\x1b[?1002h\x1b[?1006h"))
	for _, tc := range []struct {
		r    MouseReport
		want string
	}{
		{press(nil), "\x1b[<0;5;2M"},
		{press(func(r *MouseReport) { r.Kind, r.Button = MouseRelease, 2 }), "\x1b[<2;5;2m"},
		{press(func(r *MouseReport) { r.Kind, r.Mods = MouseMotion, ModCtrl }), "\x1b[<48;5;2M"},
		{press(func(r *MouseReport) { r.Kind = MouseWheelUp }), "\x1b[<64;5;2M"},
		{press(func(r *MouseReport) { r.Kind, r.Mods = MouseWheelDown, ModShift }), "\x1b[<69;5;2M"},
	} {
		if got := string(EncodeMouse(m.Modes, tc.r)); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.r, got, tc.want)
		}
	}
	if EncodeMouse(m.Modes, press(func(r *MouseReport) { r.Kind, r.Button = MouseMotion, -1 })) != nil {
		t.Error("motion without a button reported in button mode")
	}

	var legacy modeTracker
	legacy.feed([]byte("\x1b[?1000h"))
	if got := EncodeMouse(legacy.Modes, press(func(r *MouseReport) { r.Col, r.Row = 250, 0 })); !bytes.Equal(got, []byte{0x1b, '[', 'M', 32, 255, 33}) {
		t.Errorf("legacy clamp: %v", got)
	}
	if got := string(EncodeMouse(legacy.Modes, press(func(r *MouseReport) { r.Kind, r.Button = MouseRelease, 1 }))); got != "\x1b[M#%\"" {
		t.Errorf("legacy release: %q", got)
	}
	if EncodeMouse(legacy.Modes, press(func(r *MouseReport) { r.Kind = MouseMotion })) != nil {
		t.Error("motion reported in normal mode")
	}

	var x10 modeTracker
	x10.feed([]byte("\x1b[?9h"))
	if got := string(EncodeMouse(x10.Modes, press(func(r *MouseReport) { r.Mods = ModShift }))); got != "\x1b[M %\"" {
		t.Errorf("x10: %q", got)
	}
	if EncodeMouse(x10.Modes, press(func(r *MouseReport) { r.Kind = MouseRelease })) != nil {
		t.Error("x10 release reported")
	}
}
