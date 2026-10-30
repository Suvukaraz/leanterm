//go:build !windows

package services

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// fcListFormat prints one line per font face: family, style and spacing
// (100 = mono, 90 = dual width, 110 = charcell; empty = proportional).
const fcListFormat = "%{family[0]}\t%{style[0]}\t%{spacing}\n"

func listFontFaces() ([]fontFace, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "fc-list", "--format", fcListFormat).Output()
	if err != nil {
		if _, lookErr := exec.LookPath("fc-list"); lookErr != nil {
			return nil, nil
		}
		return nil, err
	}
	return parseFcListFaces(string(out)), nil
}

func parseFcList(out string) []FontFamily {
	return groupFaces(parseFcListFaces(out))
}

func parseFcListFaces(out string) []fontFace {
	var faces []fontFace
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		face := fontFace{Family: parts[0]}
		if len(parts) > 1 {
			face.Style = parts[1]
		}
		face.Mono = len(parts) > 2 && isMonoSpacing(strings.TrimSpace(parts[2]))
		faces = append(faces, face)
	}
	return faces
}

func isMonoSpacing(spacing string) bool {
	switch spacing {
	case "90", "100", "110":
		return true
	}
	return false
}
