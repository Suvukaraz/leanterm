// Package fonts loads the faces LeanTerm's GUI shapes text with: the system's
// UI and monospace fonts when they can be found, else Gio's Go fonts.
package fonts

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"golang.org/x/image/font/gofont/gomono"
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

// fontconfigCandidates asks fontconfig for the faces the web view would use
// (system-ui, monospace), so both GUIs draw with the same fonts.
func fontconfigCandidates() []candidate {
	if runtime.GOOS != "linux" {
		return nil
	}
	fcMatch, err := exec.LookPath("fc-match")
	if err != nil {
		return nil
	}
	var out []candidate
	seen := map[string]bool{}
	for _, tf := range []struct {
		typeface font.Typeface
		pattern  string
	}{{UI, "system-ui"}, {Monospace, "monospace"}} {
		for _, v := range []struct {
			suffix string
			style  font.Style
			weight font.Weight
		}{
			{"", font.Regular, font.Normal},
			{":medium", font.Regular, font.Medium},
			{":semibold", font.Regular, font.SemiBold},
			{":bold", font.Regular, font.Bold},
			{":italic", font.Italic, font.Normal},
			{":bold:italic", font.Italic, font.Bold},
		} {
			b, err := exec.Command(fcMatch, "-f", "%{file}", tf.pattern+v.suffix).Output()
			path := strings.TrimSpace(string(b))
			// A weight the family lacks resolves to another face: skip it, the
			// shaper picks the nearest one.
			if err != nil || path == "" || seen[path] {
				continue
			}
			seen[path] = true
			out = append(out, candidate{tf.typeface, v.style, v.weight, []string{path}})
		}
	}
	return out
}

func candidates() []candidate {
	if c := fontconfigCandidates(); len(c) > 0 {
		return c
	}
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
			{UI, font.Regular, font.SemiBold, f("seguisb.ttf")},
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

// monoData is the file of the regular monospace face.
var monoData []byte

// MonospaceData returns the font file of the regular monospace face (Go
// Mono without a system one), e.g. to draw terminal screenshots.
func MonospaceData() []byte {
	Collection()
	if monoData != nil {
		return monoData
	}
	return gomono.TTF
}

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
				if c.typeface == Monospace && c.style == font.Regular && c.weight == font.Normal {
					monoData = data
				}
				break
			}
		}
		// System fonts for scripts and emoji the UI and monospace fonts
		// lack (CJK, color emoji): the shaper falls back to them.
		collection = append(collection, fallbackFaces()...)
		// Nerd Font symbols: icons, and the fallback for Powerline and other
		// Nerd Font glyphs in prompts.
		if face, err := opentype.Parse(symbolsTTF); err == nil {
			collection = append(collection, font.FontFace{Font: font.Font{Typeface: Symbols}, Face: face})
		}
		// Go fonts under our names, so a missing system face still resolves.
		// Only where there is no system face: with two faces of the same
		// name, weight and style Gio may pick the Go one.
		have := map[font.Font]bool{}
		for _, ff := range collection {
			have[ff.Font] = true
		}
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
			collection = append(collection, ff)
			if !have[alias.Font] {
				collection = append(collection, alias)
			}
		}
	})
	return collection
}

// Fallback is the name prefix of the system faces for characters the UI
// and monospace fonts do not have ("LeanTerm Fallback 1", ...): each face
// needs its own name, as Gio keeps one face per name, weight and style.
const Fallback font.Typeface = "LeanTerm Fallback"

// fallbackFaces loads system fonts for CJK and emoji: fontconfig's choice
// on Linux, the fonts Windows ships elsewhere.
func fallbackFaces() []font.FontFace {
	type src struct {
		path  string
		index int
	}
	var srcs []src
	seen := map[src]bool{}
	add := func(s src) {
		if s.path != "" && !seen[s] {
			seen[s] = true
			srcs = append(srcs, s)
		}
	}
	if runtime.GOOS == "linux" {
		if fc, err := exec.LookPath("fc-match"); err == nil {
			for _, pattern := range []string{"monospace:lang=ja", "monospace:lang=zh-cn", "monospace:lang=ko", "emoji"} {
				out, err := exec.Command(fc, "-f", "%{file}\n%{index}", pattern).Output()
				if err != nil {
					continue
				}
				parts := strings.SplitN(string(out), "\n", 2)
				idx := 0
				if len(parts) == 2 {
					idx, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
				add(src{strings.TrimSpace(parts[0]), idx})
			}
		}
	}
	if runtime.GOOS == "windows" {
		dir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
		for _, n := range []string{"msgothic.ttc", "msyh.ttc", "malgun.ttf", "seguiemj.ttf"} {
			add(src{filepath.Join(dir, n), 0})
		}
	}
	var out []font.FontFace
	for _, s := range srcs {
		data, err := os.ReadFile(s.path)
		if err != nil {
			continue
		}
		faces, err := opentype.ParseCollection(data)
		if err != nil || len(faces) == 0 {
			continue
		}
		f := faces[min(s.index, len(faces)-1)]
		f.Font = font.Font{Typeface: font.Typeface(fmt.Sprintf("%s %d", Fallback, len(out)+1))}
		out = append(out, f)
	}
	return out
}
