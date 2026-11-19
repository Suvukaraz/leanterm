// Package fonts loads the faces LeanTerm's GUI shapes text with: the system's
// UI and monospace fonts when they can be found, else Gio's Go fonts.
package fonts

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
)

// Typefaces used by the GUI.
const (
	UI        font.Typeface = "LeanTerm UI"
	Monospace font.Typeface = "LeanTerm Mono"
	// Symbols is the Nerd Fonts symbol font, for icons (see icons.go).
	Symbols font.Typeface = "Symbols Nerd Font"
)

//go:embed nerd/SymbolsNerdFont-Regular.ttf
var symbolsTTF []byte

type candidate struct {
	typeface font.Typeface
	style    font.Style
	weight   font.Weight
	paths    []string
}

func candidates() []candidate {
	if runtime.GOOS == "windows" {
		dir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
		f := func(names ...string) []string {
			out := make([]string, len(names))
			for i, n := range names {
				out[i] = filepath.Join(dir, n)
			}
			return out
		}
		return []candidate{
			{UI, font.Regular, font.Normal, f("segoeui.ttf")},
			{UI, font.Regular, font.Bold, f("segoeuib.ttf")},
			{UI, font.Italic, font.Normal, f("segoeuii.ttf")},
			{Monospace, font.Regular, font.Normal, f("CascadiaMono.ttf", "consola.ttf")},
			{Monospace, font.Regular, font.Bold, f("consolab.ttf")},
			{Monospace, font.Italic, font.Normal, f("consolai.ttf")},
			{Monospace, font.Italic, font.Bold, f("consolaz.ttf")},
		}
	}
	dirs := []string{"/usr/share/fonts", "/usr/local/share/fonts"}
	f := func(rel ...string) []string {
		var out []string
		for _, d := range dirs {
			for _, r := range rel {
				out = append(out, filepath.Join(d, r))
			}
		}
		return out
	}
	return []candidate{
		{UI, font.Regular, font.Normal, f("truetype/noto/NotoSans-Regular.ttf", "noto/NotoSans-Regular.ttf", "google-noto/NotoSans-Regular.ttf", "truetype/dejavu/DejaVuSans.ttf", "dejavu/DejaVuSans.ttf", "TTF/DejaVuSans.ttf")},
		{UI, font.Regular, font.Bold, f("truetype/noto/NotoSans-Bold.ttf", "noto/NotoSans-Bold.ttf", "google-noto/NotoSans-Bold.ttf", "truetype/dejavu/DejaVuSans-Bold.ttf", "dejavu/DejaVuSans-Bold.ttf", "TTF/DejaVuSans-Bold.ttf")},
		{Monospace, font.Regular, font.Normal, f("truetype/hack/Hack-Regular.ttf", "truetype/dejavu/DejaVuSansMono.ttf", "dejavu/DejaVuSansMono.ttf", "TTF/DejaVuSansMono.ttf", "truetype/noto/NotoSansMono-Regular.ttf")},
		{Monospace, font.Regular, font.Bold, f("truetype/hack/Hack-Bold.ttf", "truetype/dejavu/DejaVuSansMono-Bold.ttf", "dejavu/DejaVuSansMono-Bold.ttf", "TTF/DejaVuSansMono-Bold.ttf", "truetype/noto/NotoSansMono-Bold.ttf")},
		{Monospace, font.Italic, font.Normal, f("truetype/hack/Hack-Italic.ttf", "truetype/dejavu/DejaVuSansMono-Oblique.ttf", "dejavu/DejaVuSansMono-Oblique.ttf", "TTF/DejaVuSansMono-Oblique.ttf")},
		{Monospace, font.Italic, font.Bold, f("truetype/hack/Hack-BoldItalic.ttf", "truetype/dejavu/DejaVuSansMono-BoldOblique.ttf", "dejavu/DejaVuSansMono-BoldOblique.ttf", "TTF/DejaVuSansMono-BoldOblique.ttf")},
	}
}

var (
	once       sync.Once
	collection []font.FontFace
)

// Collection returns the system faces found (registered under [UI] and
// [Monospace]) followed by the Go fonts as fallback, also for glyphs the
// system faces lack.
func Collection() []font.FontFace {
	once.Do(func() {
		for _, c := range candidates() {
			for _, p := range c.paths {
				data, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				face, err := opentype.Parse(data)
				if err != nil {
					continue
				}
				collection = append(collection, font.FontFace{
					Font: font.Font{Typeface: c.typeface, Style: c.style, Weight: c.weight},
					Face: face,
				})
				break
			}
		}
		// Nerd Font symbols: icons, and the fallback for Powerline and other
		// Nerd Font glyphs in prompts.
		if face, err := opentype.Parse(symbolsTTF); err == nil {
			collection = append(collection, font.FontFace{Font: font.Font{Typeface: Symbols}, Face: face})
		}
		// Go fonts under our names, so a missing system face still resolves.
		for _, ff := range gofont.Collection() {
			alias := ff
			switch ff.Font.Typeface {
			case "Go Mono":
				alias.Font.Typeface = Monospace
			case "Go":
				alias.Font.Typeface = UI
			default: // Go Smallcaps, Go Medium etc. are not aliased
				collection = append(collection, ff)
				continue
			}
			collection = append(collection, ff, alias)
		}
	})
	return collection
}
