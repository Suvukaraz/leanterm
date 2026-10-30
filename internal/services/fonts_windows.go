//go:build windows

package services

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const fontsKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`

// listFontFaces reads the fonts registered for all users (HKLM, files in
// %WINDIR%\Fonts) and for the current user (HKCU, usually full paths in
// %LOCALAPPDATA%\Microsoft\Windows\Fonts) and takes the names from the files.
func listFontFaces() ([]fontFace, error) {
	fontDir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	seen := map[string]bool{}
	var faces []fontFace
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, file := range registeredFontFiles(root) {
			if !filepath.IsAbs(file) {
				file = filepath.Join(fontDir, file)
			}
			key := strings.ToLower(file)
			if seen[key] {
				continue
			}
			seen[key] = true
			switch strings.ToLower(filepath.Ext(file)) {
			case ".ttf", ".otf", ".ttc", ".otc":
			default:
				continue // bitmap (.fon) and vector fonts cannot be used by the web view
			}
			f, err := os.Open(file)
			if err != nil {
				continue
			}
			if fs, err := parseFontFile(f); err == nil {
				faces = append(faces, fs...)
			}
			f.Close()
		}
	}
	return faces, nil
}

func registeredFontFiles(root registry.Key) []string {
	k, err := registry.OpenKey(root, fontsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	files := make([]string, 0, len(names))
	for _, n := range names {
		if v, _, err := k.GetStringValue(n); err == nil && v != "" {
			files = append(files, v)
		}
	}
	return files
}
