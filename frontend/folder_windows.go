//go:build windows && !android && !ios

package frontend

import "os/exec"

func openPlatformFolder(path string) error {
	command := exec.Command("explorer.exe", path)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
