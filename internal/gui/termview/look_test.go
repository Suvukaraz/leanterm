package termview

import (
	"image/color"
	"testing"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/gui/term"
)

// Ported from frontend/src/lib/terminal/look.test.ts.

func TestMergeLook(t *testing.T) {
	g := config.TerminalLookConfig{Font: config.FontConfig{Family: "Monospace", Size: 11}, Foreground: "#cccccc", ScrollbackLines: 5000}
	m := MergeLook(g, &config.TerminalLookConfig{Font: config.FontConfig{Size: 14}, ScrollbackLines: -2})
	if m.Font != (config.FontConfig{Family: "Monospace", Size: 14}) || m.Foreground != "#cccccc" || m.ScrollbackLines != 5000 {
		t.Fatalf("merged %+v", m)
	}
	// Style and features stay with the family they were chosen for.
	g = config.TerminalLookConfig{Font: config.FontConfig{Family: "Hack", Size: 11, Style: "Bold", Features: "liga"}}
	if m := MergeLook(g, &config.TerminalLookConfig{Font: config.FontConfig{Size: 14}, ScrollbackLines: -2}); m.Font != (config.FontConfig{Family: "Hack", Size: 14, Style: "Bold", Features: "liga"}) {
		t.Fatalf("size only: %+v", m.Font)
	}
	if m := MergeLook(g, &config.TerminalLookConfig{Font: config.FontConfig{Family: "Fira Code"}, ScrollbackLines: -2}); m.Font != (config.FontConfig{Family: "Fira Code", Size: 11}) {
		t.Fatalf("own family: %+v", m.Font)
	}
	if m := MergeLook(g, nil); m.Font != g.Font {
		t.Fatal("no personal look changed the look")
	}
}

func TestLookMapping(t *testing.T) {
	if LookCursor(config.TerminalLookConfig{CursorShape: "ibeam"}) != term.CursorBar ||
		LookCursor(config.TerminalLookConfig{CursorShape: "underline"}) != term.CursorUnderline ||
		LookCursor(config.TerminalLookConfig{}) != term.CursorBlock {
		t.Error("cursor shapes")
	}
	if LookScrollback(config.TerminalLookConfig{ScrollbackLines: -1}) != 100000 || LookScrollback(config.TerminalLookConfig{ScrollbackLines: 300}) != 300 {
		t.Error("scrollback")
	}
	if LookFontPt(config.TerminalLookConfig{}) != 11 || LookFontPt(config.TerminalLookConfig{Font: config.FontConfig{Size: 12}}) != 12 {
		t.Error("font size")
	}
	if LookFamily(config.TerminalLookConfig{Font: config.FontConfig{Family: "Monospace"}}) != "" || LookFamily(config.TerminalLookConfig{Font: config.FontConfig{Family: "Fira Code"}}) != "Fira Code" {
		t.Error("family")
	}
	c := LookColors(config.TerminalLookConfig{Background: "#101010", Palette: []string{"#010203"}})
	if c.Bg != (color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}) || c.Fg != (color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}) || c.Cursor != c.Fg {
		t.Errorf("colors %+v", c)
	}
	// An incomplete palette is ignored: the Tango colors apply.
	if c.Palette[1] != (color.NRGBA{R: 0xcc, A: 0xff}) || c.Palette[200] != term.XtermPalette()[200] {
		t.Errorf("palette %v %v", c.Palette[1], c.Palette[200])
	}
}
