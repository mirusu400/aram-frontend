//go:build js && wasm

package frontend

import "errors"

func customGamepadMappingsPath() (string, error) {
	return "", errors.New("custom controller databases are unavailable in a browser")
}

// Browsers do not expose a neighboring gamecontrollerdb.txt file. Ebitengine's
// built-in mappings remain available, so startup should quietly skip the
// desktop-only custom database instead of consulting os.UserConfigDir.
func loadCustomGamepadMappings() (bool, error) {
	return false, nil
}
