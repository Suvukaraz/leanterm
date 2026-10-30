package services

import (
	"os"
	"testing"
)

// Uses font files of the system when they exist (Debian: fonts-dejavu, fonts-noto-cjk).
func TestParseFontFile(t *testing.T) {
	cases := []struct {
		path   string
		family string
		style  string
		mono   bool
	}{
		{"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "DejaVu Sans Mono", "Book", true},
		{"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", "DejaVu Sans", "Bold", false},
		{`C:\Windows\Fonts\consola.ttf`, "Consolas", "Regular", true},
		{`C:\Windows\Fonts\arialbd.ttf`, "Arial", "Bold", false},
	}
	tested := 0
	for _, c := range cases {
		f, err := os.Open(c.path)
		if err != nil {
			continue
		}
		faces, err := parseFontFile(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		tested++
		if len(faces) != 1 || faces[0].Family != c.family || faces[0].Style != c.style || faces[0].Mono != c.mono {
			t.Errorf("%s: got %+v, want %s / %s / mono=%v", c.path, faces, c.family, c.style, c.mono)
		}
	}
	if f, err := os.Open("/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc"); err == nil {
		faces, err := parseFontFile(f)
		f.Close()
		if err != nil || len(faces) < 2 {
			t.Errorf("collection: %v, %d faces", err, len(faces))
		}
		tested++
	}
	if tested == 0 {
		t.Skip("no known font files on this system")
	}
}

func TestParseFontFileRejectsOtherFiles(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "x.ttf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("this is not a font file at all")
	defer f.Close()
	if _, err := parseFontFile(f); err == nil {
		t.Fatal("a text file was accepted as font")
	}
}
