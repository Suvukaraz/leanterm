package fonts

import (
	"testing"

	"gioui.org/font/opentype"
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
