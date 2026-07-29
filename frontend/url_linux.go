//go:build linux && !android

package frontend

import "os/exec"

func openPlatformURL(rawURL string) error {
	command := exec.Command("xdg-open", rawURL)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
