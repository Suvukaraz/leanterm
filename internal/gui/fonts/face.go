package fonts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

var (
	faceMu    sync.Mutex
	faceCache = map[string]*font.FontFace{}
	scanned   []font.FontFace
	scanOnce  sync.Once
)

// Face loads the face of an installed font family in a style ("Bold",
// "Italic", ...), e.g. for previews in the font chooser. On Linux fontconfig
// finds the file; elsewhere the font folders are read once.
func Face(family, style string) (font.FontFace, bool) {
	key := family + "\x00" + style
	faceMu.Lock()
	if f, ok := faceCache[key]; ok {
		faceMu.Unlock()
		if f == nil {
			return font.FontFace{}, false
		}
		return *f, true
	}
	faceMu.Unlock()

	f, ok := findFace(family, style)
	faceMu.Lock()
	if ok {
		faceCache[key] = &f
	} else {
		faceCache[key] = nil
	}
	faceMu.Unlock()
	return f, ok
}

func findFace(family, style string) (font.FontFace, bool) {
	if runtime.GOOS == "linux" {
		if fc, err := exec.LookPath("fc-match"); err == nil {
			pattern := family
			if style != "" {
				pattern += ":style=" + style
			}
			out, err := exec.Command(fc, "-f", "%{family}\n%{file}", pattern).Output()
			lines := strings.SplitN(string(out), "\n", 2)
			// fontconfig substitutes another family for an unknown one.
			if err == nil && len(lines) == 2 && familyMatches(lines[0], family) {
				if data, err := os.ReadFile(lines[1]); err == nil {
					if faces, err := opentype.ParseCollection(data); err == nil && len(faces) > 0 {
						return pick(faces, style), true
					}
				}
			}
			return font.FontFace{}, false
		}
	}
	scanOnce.Do(scanFolders)
	var matches []font.FontFace
	for _, f := range scanned {
		if strings.EqualFold(string(f.Font.Typeface), family) {
			matches = append(matches, f)
		}
	}
	if len(matches) == 0 {
		return font.FontFace{}, false
	}
	return pick(matches, style), true
}

// familyMatches compares a family with fontconfig's list of names for it.
func familyMatches(names, family string) bool {
	for _, n := range strings.Split(names, ",") {
		if strings.EqualFold(strings.TrimSpace(n), family) {
			return true
		}
	}
	return false
}

// pick chooses the face for a style name by weight and slant.
func pick(faces []font.FontFace, style string) font.FontFace {
	s := strings.ToLower(style)
	want := font.Font{Weight: font.Normal}
	switch {
	case strings.Contains(s, "black") || strings.Contains(s, "heavy"):
		want.Weight = font.Black
	case strings.Contains(s, "extrabold") || strings.Contains(s, "ultrabold"):
		want.Weight = font.ExtraBold
	case strings.Contains(s, "semibold") || strings.Contains(s, "demibold"):
		want.Weight = font.SemiBold
	case strings.Contains(s, "bold"):
		want.Weight = font.Bold
	case strings.Contains(s, "medium"):
		want.Weight = font.Medium
	case strings.Contains(s, "light"):
		want.Weight = font.Light
	case strings.Contains(s, "thin"):
		want.Weight = font.Thin
	}
	if strings.Contains(s, "italic") || strings.Contains(s, "oblique") {
		want.Style = font.Italic
	}
	best, bestScore := faces[0], -1
	for _, f := range faces {
		score := 0
		if f.Font.Style == want.Style {
			score += 1000
		}
		d := int(f.Font.Weight - want.Weight)
		if d < 0 {
			d = -d
		}
		score -= d
		if bestScore < 0 || score > bestScore {
			best, bestScore = f, score
		}
	}
	return best
}

func scanFolders() {
	var dirs []string
	if runtime.GOOS == "windows" {
		dirs = append(dirs, filepath.Join(os.Getenv("WINDIR"), "Fonts"))
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			dirs = append(dirs, filepath.Join(la, "Microsoft", "Windows", "Fonts"))
		}
	} else {
		home, _ := os.UserHomeDir()
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts"))
	}
	for _, d := range dirs {
		_ = filepath.WalkDir(d, func(path string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".otc":
			default:
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if faces, err := opentype.ParseCollection(data); err == nil {
				scanned = append(scanned, faces...)
			}
			return nil
		})
	}
}
