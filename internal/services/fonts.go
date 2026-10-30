package services

import (
	"sort"
	"strings"
)

// FontFamily is an installed font family as listed by fontconfig.
type FontFamily struct {
	Name      string   `json:"name"`
	Styles    []string `json:"styles"`
	Monospace bool     `json:"monospace"`
}

// ListFonts returns the installed font families, sorted by name: from fc-list on
// Unix, from the font registry and the font files on Windows. An empty list
// means no font source is available; the font dialog then uses a built-in list.
func (ss *SystemService) ListFonts() ([]FontFamily, error) {
	faces, err := listFontFaces()
	if err != nil {
		return nil, err
	}
	return groupFaces(faces), nil
}

// groupFaces merges faces into families with their distinct styles.
func groupFaces(faces []fontFace) []FontFamily {
	byName := map[string]*FontFamily{}
	seenStyle := map[string]map[string]bool{}
	for _, face := range faces {
		name := strings.TrimSpace(face.Family)
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		style := strings.TrimSpace(face.Style)
		if style == "" {
			style = "Regular"
		}
		f := byName[name]
		if f == nil {
			f = &FontFamily{Name: name, Styles: []string{}}
			byName[name] = f
			seenStyle[name] = map[string]bool{}
		}
		// A family counts as monospace when its faces are (some list no spacing for
		// single faces, e.g. emoji fallbacks inside a mono family).
		f.Monospace = f.Monospace || face.Mono
		if !seenStyle[name][style] {
			seenStyle[name][style] = true
			f.Styles = append(f.Styles, style)
		}
	}

	families := make([]FontFamily, 0, len(byName))
	for _, f := range byName {
		families = append(families, *f)
	}
	sort.Slice(families, func(i, j int) bool {
		a, b := strings.ToLower(families[i].Name), strings.ToLower(families[j].Name)
		if a != b {
			return a < b
		}
		return families[i].Name < families[j].Name
	})
	return families
}
