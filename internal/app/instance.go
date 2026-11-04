package app

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// LeanEditInstanceID is the single instance id of LeanEdit. It includes the
// configuration folder, so LeanEdit started with another --config-dir is a
// separate instance. D-Bus needs dot-separated elements that start with a
// letter, hence the "c" before the hash.
func LeanEditInstanceID(appID, configDir string) string {
	if abs, err := filepath.Abs(configDir); err == nil {
		configDir = abs
	}
	sum := sha256.Sum256([]byte(filepath.Clean(configDir)))
	return appID + ".Instance.c" + hex.EncodeToString(sum[:8])
}
