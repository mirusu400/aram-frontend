//go:build android || ios

package frontend

import "errors"

func openPlatformURL(string) error {
	return errors.New("links are opened by the native mobile host")
}
