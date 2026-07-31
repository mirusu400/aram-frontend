//go:build darwin && !ios

package frontend

import "os/exec"

func openPlatformFolder(path string) error {
	command := exec.Command("open", path)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
