package localfs

import (
	"runtime"
	"testing"
)

func TestIsRoot(t *testing.T) {
	roots := []string{"/"}
	others := []string{"/home", "/tmp/x"}
	if runtime.GOOS == "windows" {
		roots = []string{`C:\`, `C:/`, `D:\`, `\\server\share\`}
		others = []string{`C:\Users`, `D:\data\x`, `\\server\share\dir`}
	}
	for _, p := range roots {
		if !isRoot(p) {
			t.Errorf("isRoot(%q) = false", p)
		}
	}
	for _, p := range others {
		if isRoot(p) {
			t.Errorf("isRoot(%q) = true", p)
		}
	}
}
