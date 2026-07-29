//go:build windows && !android && !ios

package frontend

import "os/exec"

func openPlatformURL(rawURL string) error {
	command := exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
