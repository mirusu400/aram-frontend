//go:build android

package frontend

import "errors"

func writeClipboardText(string) error {
	return errors.New("clipboard shortcuts are unavailable on Android")
}
