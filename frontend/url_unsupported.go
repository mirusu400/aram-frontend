//go:build !windows && !linux && !darwin && !android && !ios

package frontend

import "errors"

func openPlatformURL(string) error {
	return errors.New("opening links is unavailable on this platform")
}
