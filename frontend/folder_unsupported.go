//go:build !windows && !linux && !darwin && !android && !ios

package frontend

import "errors"

func openPlatformFolder(string) error {
	return errors.New("opening folders is unsupported on this platform")
}
