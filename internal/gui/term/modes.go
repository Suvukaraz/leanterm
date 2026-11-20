package term

import (
	"fmt"
	"strconv"
	"strings"
)

// MouseTracking is what the program wants reported (DECSET 9, 1000, 1002,
// 1003).
type MouseTracking uint8

const (
	MouseNone MouseTracking = iota
	MouseX10
	MouseNormal // presses and releases
	MouseButton // and motion while a button is down
	MouseAny    // and all motion
)

// Modes are the terminal modes the GUI needs: mouse reporting and whether
// the alternate screen is shown.
type Modes struct {
	Mouse MouseTracking
	// SGR is the SGR mouse encoding (DECSET 1006).
	SGR bool
	// AltScreen is set while the alternate screen is active (DECSET 47,
	// 1047, 1049).
	AltScreen bool
}

var trackingModes = map[int]MouseTracking{9: MouseX10, 1000: MouseNormal, 1002: MouseButton, 1003: MouseAny}

// modeTracker follows the mode changes in the output of the program
// (frontend/src/lib/terminal/mouseReports.ts). Sequences split across Feed
// calls are carried over. Both engines use it, so the GUI sees the same
// modes with either.
type modeTracker struct {
	Modes
	state   uint8 // 0 ground, 1 escape, 2 CSI
	private bool
	params  []byte
}

func (t *modeTracker) feed(p []byte) {
	for _, b := range p {
		switch t.state {
		case 0:
			if b == 0x1b {
				t.state = 1
			}
		case 1:
			switch {
			case b == '[':
				t.state, t.private, t.params = 2, false, t.params[:0]
			case b == 'c': // RIS
				t.Modes = Modes{}
				t.state = 0
			case b == 0x1b:
			default:
				t.state = 0
			}
		case 2:
			switch {
			case b == '?' && len(t.params) == 0 && !t.private:
				t.private = true
			case b >= '0' && b <= '9' || b == ';':
				if len(t.params) < 64 {
					t.params = append(t.params, b)
				}
			case b >= 0x40 && b <= 0x7e:
				if t.private && (b == 'h' || b == 'l') {
					t.set(b == 'h')
				}
				t.state = 0
			case b == 0x1b:
				t.state = 1
			default:
				t.state = 0
			}
		}
	}
}

func (t *modeTracker) set(on bool) {
	for _, p := range strings.Split(string(t.params), ";") {
		mode, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		switch {
		case mode == 1006:
			t.SGR = on
		case mode == 47 || mode == 1047 || mode == 1049:
			t.AltScreen = on
		default:
			if m, ok := trackingModes[mode]; ok {
				if on {
					t.Mouse = m
				} else if t.Mouse == m {
					t.Mouse = MouseNone
				}
			}
		}
	}
}

// MouseKind is the kind of a mouse report.
type MouseKind uint8

const (
	MousePress MouseKind = iota
	MouseRelease
	MouseMotion
	MouseWheelUp
	MouseWheelDown
)

// MouseReport is a mouse event for the program.
type MouseReport struct {
	Kind MouseKind
	// Button: 0 left, 1 middle, 2 right; -1 for motion without a button.
	Button int
	// Col and Row are the zero-based cell.
	Col, Row int
	Mods     Mods
}

// EncodeMouse encodes a report for the modes; nil when nothing is to be
// sent (frontend/src/lib/terminal/mouseReports.ts, and the wheel reports of
// ghosttyEngine.ts).
func EncodeMouse(m Modes, r MouseReport) []byte {
	t := m.Mouse
	if t == MouseNone {
		return nil
	}
	wheel := r.Kind == MouseWheelUp || r.Kind == MouseWheelDown
	if t == MouseX10 && r.Kind != MousePress {
		return nil
	}
	if r.Kind == MouseMotion && t != MouseAny && !(t == MouseButton && r.Button >= 0) {
		return nil
	}
	if !wheel && r.Button > 2 {
		return nil
	}
	code := r.Button
	switch {
	case wheel:
		code = 64
		if r.Kind == MouseWheelDown {
			code = 65
		}
	case r.Button < 0:
		code = 3
	}
	if r.Kind == MouseRelease && !m.SGR {
		code = 3
	}
	if r.Kind == MouseMotion {
		code += 32
	}
	if t != MouseX10 {
		if r.Mods&ModShift != 0 {
			code += 4
		}
		if r.Mods&ModAlt != 0 {
			code += 8
		}
		if r.Mods&ModCtrl != 0 {
			code += 16
		}
	}
	x, y := r.Col+1, r.Row+1
	if m.SGR {
		final := 'M'
		if r.Kind == MouseRelease {
			final = 'm'
		}
		return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", code, x, y, final))
	}
	// Legacy encoding: one byte per value; coordinates beyond 223 cannot be
	// expressed.
	return []byte{0x1b, '[', 'M', byte(32 + code), byte(32 + min(x, 223)), byte(32 + min(y, 223))}
}
