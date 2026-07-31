//go:build linux && !android

package frontend

import "os/exec"

func openPlatformFolder(path string) error {
	command := exec.Command("xdg-open", path)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
