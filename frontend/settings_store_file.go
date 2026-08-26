//go:build !js || !wasm

package frontend

import (
	"os"
	"path/filepath"
)

// This is the filesystem-backed settings store used on every target with a real
// filesystem (desktop, mobile, and other native hosts). The web/wasm build
// replaces it with a browser-localStorage store (settings_store_web.go); the two
// build constraints are exact complements, so exactly one compiles per target.

func readSettingsBlob() ([]byte, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func writeSettingsBlob(data []byte) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
