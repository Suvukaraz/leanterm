package termview

import (
	"fmt"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// The terminal look ([terminal.look], per connection overridable), as
// frontend/src/lib/terminal/look.ts maps it for the web GUI's engines.

// MergeLook lays a connection's personal look over the global one: what the
// personal one leaves empty is inherited.
func MergeLook(global config.TerminalLookConfig, personal *config.TerminalLookConfig) config.TerminalLookConfig {
	out := global
	if personal == nil {
		return out
	}
	p := personal
	// Style and features belong to the family they were chosen for.
	if p.Font.Family != "" {
		out.Font.Family, out.Font.Style, out.Font.Features = p.Font.Family, p.Font.Style, p.Font.Features
	}
	if p.Font.Size > 0 {
		out.Font.Size = p.Font.Size
	}
	str := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	str(&out.Foreground, p.Foreground)
	str(&out.Background, p.Background)
	str(&out.Bold, p.Bold)
	str(&out.CursorShape, p.CursorShape)
	str(&out.WordChars, p.WordChars)
	str(&out.Encoding, p.Encoding)
	str(&out.Backspace, p.Backspace)
	out.BoldLikeText, out.BoldIsBright, out.Transparency = p.BoldLikeText, p.BoldIsBright, p.Transparency
	if p.ScrollbackLines != -2 { // -2: inherit
		out.ScrollbackLines = p.ScrollbackLines
	}
	if len(p.Palette) == 16 {
		out.Palette = p.Palette
	}
	return out
}

// lookPalette is look.ts's DEFAULT_PALETTE (the Tango colors).
var lookPalette = []string{
	"#000000", "#cc0000", "#4e9a06", "#c4a000", "#3465a4", "#75507b", "#06989a", "#d3d7cf",
	"#555753", "#ef2929", "#8ae234", "#fce94f", "#729fcf", "#ad7fa8", "#34e2e2", "#eeeeec",
}

// ParseHex parses "#rrggbb"; ok is false for anything else.
func ParseHex(s string) (color.NRGBA, bool) {
	var r, g, b uint8
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, false
	}
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}, true
}

func hexOr(s, def string) color.NRGBA {
	if c, ok := ParseHex(s); ok {
		return c
	}
	c, _ := ParseHex(def)
	return c
}

// LookColors are the terminal colors of a look: foreground (#cccccc),
// background (#000000), the cursor in the foreground color and the 16
// palette colors over xterm's 256-color palette (look.ts's buildTheme).
func LookColors(l config.TerminalLookConfig) term.Colors {
	p := term.XtermPalette()
	pal := lookPalette
	if len(l.Palette) == 16 {
		pal = l.Palette
	}
	for i := range 16 {
		p[i] = hexOr(pal[i], lookPalette[i])
	}
	fg := hexOr(l.Foreground, "#cccccc")
	return term.Colors{Fg: fg, Bg: hexOr(l.Background, "#000000"), Cursor: fg, Palette: &p}
}

// LookCursor maps the cursor shape setting (ibeam = bar).
func LookCursor(l config.TerminalLookConfig) term.CursorShape {
	switch l.CursorShape {
	case "ibeam":
		return term.CursorBar
	case "underline":
		return term.CursorUnderline
	}
	return term.CursorBlock
}

// LookScrollback is the number of history lines (-1 = "unlimited" =
// 100000; no setting = 5000; 0 = none).
func LookScrollback(l config.TerminalLookConfig) int {
	switch {
	case l.ScrollbackLines < 0:
		return 100000
	case l.ScrollbackLines == 0:
		return 0
	}
	return l.ScrollbackLines
}

// LookFontPt is the font size in points (11 without a setting).
func LookFontPt(l config.TerminalLookConfig) int {
	if l.Font.Size > 0 {
		return l.Font.Size
	}
	return DefaultFontPt
}

// LookFamily is the font family to load ("" for the system monospace font:
// no setting or a generic name like "Monospace").
func LookFamily(l config.TerminalLookConfig) string {
	f := strings.TrimSpace(l.Font.Family)
	switch strings.ToLower(f) {
	case "", "mono", "monospace":
		return ""
	}
	return f
}

// ApplyLook sets the view's font, cursor and word characters (the colors
// and the history are the engine's, set when it is created).
func (v *View) ApplyLook(l config.TerminalLookConfig) {
	v.FontSize = unit.Sp(float32(LookFontPt(l)) * 96 / 72)
	v.Typeface = font.Typeface(LookFamily(l))
	v.CursorShape = LookCursor(l)
	v.WordChars = DefaultWordChars
	if l.WordChars != "" {
		v.WordChars = l.WordChars
	}
}
