//go:build !js || !wasm

package frontend

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

func customGamepadMappingsPath() (string, error) {
	path, err := settingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "gamecontrollerdb.txt"), nil
}

func loadCustomGamepadMappings() (bool, error) {
	path, err := customGamepadMappingsPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, errors.New("gamecontrollerdb.txt is empty")
	}
	return ebiten.UpdateStandardGamepadLayoutMappings(string(data))
}
