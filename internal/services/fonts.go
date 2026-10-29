package services

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// FontFamily is an installed font family as listed by fontconfig.
type FontFamily struct {
	Name      string   `json:"name"`
	Styles    []string `json:"styles"`
	Monospace bool     `json:"monospace"`
}

// fcListFormat prints one line per font face: family, style and spacing
// (100 = mono, 90 = dual width, 110 = charcell; empty = proportional).
const fcListFormat = "%{family[0]}\t%{style[0]}\t%{spacing}\n"

// ListFonts returns the installed font families (via fc-list), sorted by name.
// An empty list means fontconfig is not available; the font dialog then falls
// back to a built-in list.
func (ss *SystemService) ListFonts() ([]FontFamily, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "fc-list", "--format", fcListFormat).Output()
	if err != nil {
		if _, lookErr := exec.LookPath("fc-list"); lookErr != nil {
			return []FontFamily{}, nil
		}
		return nil, err
	}
	return parseFcList(string(out)), nil
}

func parseFcList(out string) []FontFamily {
	byName := map[string]*FontFamily{}
	seenStyle := map[string]map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		style := ""
		if len(parts) > 1 {
			style = strings.TrimSpace(parts[1])
		}
		if style == "" {
			style = "Regular"
		}
		mono := len(parts) > 2 && isMonoSpacing(strings.TrimSpace(parts[2]))

		f := byName[name]
		if f == nil {
			f = &FontFamily{Name: name, Styles: []string{}}
			byName[name] = f
			seenStyle[name] = map[string]bool{}
		}
		// A family counts as monospace when its faces are (some list no spacing for
		// single faces, e.g. emoji fallbacks inside a mono family).
		f.Monospace = f.Monospace || mono
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

func isMonoSpacing(spacing string) bool {
	switch spacing {
	case "90", "100", "110":
		return true
	}
	return false
}
