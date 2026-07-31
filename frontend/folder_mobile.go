//go:build android || ios

package frontend

import "errors"

func openPlatformFolder(string) error {
	return errors.New("the native host does not expose a folder browser")
}
