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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeFileAtomically(dir, path, data)
}

// writeFileAtomically writes data to a temp file in dir and renames it over
// path. settings.save runs on nearly every setting change and every title
// opened, so a process torn down mid-write - a crash, a forced quit, a power
// loss - is not a rare timing to hit; a direct WriteFile leaves a truncated
// file that fails to parse, and loadSettings silently resets every setting -
// the recent titles list included - back to defaults on the next launch. The
// rename leaves the previous complete file in place instead.
func writeFileAtomically(dir, path string, data []byte) (err error) {
	temp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if err != nil {
			os.Remove(tempPath)
		}
	}()
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
