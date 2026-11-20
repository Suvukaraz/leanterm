package fonts

import (
	"strings"
	"testing"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

func TestSymbolsFontHasAllIcons(t *testing.T) {
	face, err := opentype.Parse(symbolsTTF)
	if err != nil {
		t.Fatal(err)
	}
	f := face.Face()
	for _, r := range AllIcons {
		if _, ok := f.NominalGlyph(r); !ok {
			t.Errorf("icon U+%04X missing in the Nerd Fonts symbol font", r)
		}
	}
}

func TestCollectionHasOurTypefaces(t *testing.T) {
	seen := map[string]bool{}
	for _, ff := range Collection() {
		seen[string(ff.Font.Typeface)] = true
	}
	for _, tf := range []string{string(UI), string(Monospace), string(Symbols)} {
		if !seen[tf] {
			t.Errorf("typeface %q missing", tf)
		}
	}
}

func TestFaceLoadsInstalledFamily(t *testing.T) {
	fams := []string{"DejaVu Sans Mono", "Inter", "Consolas", "Segoe UI"}
	for _, fam := range fams {
		if f, ok := Face(fam, "Bold"); ok {
			if f.Face == nil {
				t.Fatalf("%s: no face", fam)
			}
			if _, ok := Face("No Such Font Family 42", "Regular"); ok {
				t.Fatal("found a family that does not exist")
			}
			return
		}
	}
	t.Skip("none of the test families is installed")
}

// Two faces with the same name, weight and style make Gio pick either: the
// Go fallbacks must not shadow the system's UI and monospace fonts.
func TestCollectionHasNoDuplicateFaces(t *testing.T) {
	seen := map[font.Font]bool{}
	for _, ff := range Collection() {
		if seen[ff.Font] {
			t.Errorf("face %+v registered twice", ff.Font)
		}
		seen[ff.Font] = true
	}
}

// Characters the monospace font lacks come from the system's fallback
// faces (CJK here; skipped where none is installed).
func TestMonospaceFallsBackForCJK(t *testing.T) {
	hasFallback := false
	for _, ff := range Collection() {
		hasFallback = hasFallback || strings.HasPrefix(string(ff.Font.Typeface), string(Fallback))
	}
	if !hasFallback {
		t.Skip("no fallback fonts on this system")
	}
	sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection(Collection()))
	sh.LayoutString(text.Parameters{Font: font.Font{Typeface: Monospace}, PxPerEm: fixed.I(16), MaxWidth: 1000}, "日本")
	n := 0
	for g, ok := sh.NextGlyph(); ok; g, ok = sh.NextGlyph() {
		if g.ID&0xffff == 0 {
			t.Fatalf("glyph %d is .notdef (tofu)", n)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("%d glyphs", n)
	}
}
