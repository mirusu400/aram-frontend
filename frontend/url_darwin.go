//go:build darwin && !ios

package frontend

import "os/exec"

func openPlatformURL(rawURL string) error {
	command := exec.Command("open", rawURL)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
