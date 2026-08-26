package frontend

import (
	"os"
	"path/filepath"
)

// settingsPath is the on-disk location of the ARAM settings directory/file. It
// is shared by the filesystem settings store and the sibling files that live
// next to settings.json (e.g. the custom gamepad database). The web/wasm build
// persists settings in localStorage and does not use this path for settings,
// but keeps it defined so filesystem-adjacent loaders compile and degrade
// gracefully: on wasm os.UserConfigDir returns an error, which those callers
// already treat as "nothing to load".
func settingsPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ARAM", "settings.json"), nil
}
