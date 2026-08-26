//go:build js && wasm

package frontend

import (
	"errors"
	"syscall/js"
)

// Web build: persist settings in the browser's localStorage instead of a file.
// localStorage is origin-scoped and survives reloads, so theme, language, CPU
// core, speed, and the recent list stick across sessions like the desktop
// settings.json. It can be absent or throw (private mode, disabled site data,
// quota), so a read miss falls back to defaults and a write error is non-fatal -
// matching how the file store's os errors are treated by the callers.
const webSettingsKey = "aram.settings"

var errNoLocalStorage = errors.New("localStorage is unavailable")

func localStorage() (js.Value, error) {
	global := js.Global()
	if !global.Truthy() {
		return js.Value{}, errNoLocalStorage
	}
	store := global.Get("localStorage")
	if !store.Truthy() {
		return js.Value{}, errNoLocalStorage
	}
	return store, nil
}

func readSettingsBlob() (data []byte, err error) {
	// getItem can throw in locked-down contexts; recover so first launch falls
	// back to defaults rather than crashing the game loop.
	defer func() {
		if recover() != nil {
			data, err = nil, errNoLocalStorage
		}
	}()
	store, err := localStorage()
	if err != nil {
		return nil, err
	}
	value := store.Call("getItem", webSettingsKey)
	if !value.Truthy() {
		return nil, errNoLocalStorage
	}
	return []byte(value.String()), nil
}

func writeSettingsBlob(data []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errNoLocalStorage
		}
	}()
	store, err := localStorage()
	if err != nil {
		return err
	}
	store.Call("setItem", webSettingsKey, string(data))
	return nil
}
