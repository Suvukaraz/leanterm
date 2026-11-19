package term

import (
	"image/color"
	"net/url"
	"strings"
)

// Colors is a terminal color scheme.
type Colors struct {
	Fg, Bg, Cursor color.NRGBA
	// Palette holds the 256 indexed colors; the first 16 are the ANSI colors.
	Palette *[256]color.NRGBA
}

// DefaultColors is LeanTerm's default scheme: light grey on black with the
// xterm palette.
func DefaultColors() Colors {
	p := XtermPalette()
	return Colors{
		Fg:      color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff},
		Bg:      color.NRGBA{A: 0xff},
		Cursor:  color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff},
		Palette: &p,
	}
}

// XtermPalette returns xterm's 256-color palette.
func XtermPalette() [256]color.NRGBA {
	var p [256]color.NRGBA
	ansi := [16][3]uint8{
		{0x00, 0x00, 0x00}, {0xcd, 0x00, 0x00}, {0x00, 0xcd, 0x00}, {0xcd, 0xcd, 0x00},
		{0x00, 0x00, 0xee}, {0xcd, 0x00, 0xcd}, {0x00, 0xcd, 0xcd}, {0xe5, 0xe5, 0xe5},
		{0x7f, 0x7f, 0x7f}, {0xff, 0x00, 0x00}, {0x00, 0xff, 0x00}, {0xff, 0xff, 0x00},
		{0x5c, 0x5c, 0xff}, {0xff, 0x00, 0xff}, {0x00, 0xff, 0xff}, {0xff, 0xff, 0xff},
	}
	for i, c := range ansi {
		p[i] = color.NRGBA{R: c[0], G: c[1], B: c[2], A: 0xff}
	}
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for i := 0; i < 216; i++ {
		p[16+i] = color.NRGBA{R: levels[i/36], G: levels[i/6%6], B: levels[i%6], A: 0xff}
	}
	for i := 0; i < 24; i++ {
		v := uint8(8 + 10*i)
		p[232+i] = color.NRGBA{R: v, G: v, B: v, A: 0xff}
	}
	return p
}

func toNRGBA(c color.Color) color.NRGBA {
	if n, ok := c.(color.NRGBA); ok {
		return n
	}
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// dirFromOSC7 turns the value of OSC 7 (file://host/path) into a path; other
// values are returned as they are.
func dirFromOSC7(v string) string {
	if !strings.HasPrefix(v, "file://") {
		return v
	}
	u, err := url.Parse(v)
	if err != nil || u.Path == "" {
		return v
	}
	p := u.Path
	// file:///C:/Users -> C:/Users
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return p
}
