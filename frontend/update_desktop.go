//go:build !android && !ios

package frontend

import (
	"os"
	"path/filepath"
)

// defaultUpdateDownloadRoot is the user-visible folder that keeps verified
// update archives on desktop hosts.
func defaultUpdateDownloadRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads", "ARAM"), nil
}

// productInstallActionLabel names the ARAM product row action when the host
// can install the downloaded build. Desktop hosts extract the archive and
// relaunch the executable themselves.
func productInstallActionLabel() string { return "Install & Restart" }

// platformInstallsProductOnWelcome reports whether choosing a channel on the
// first-run Welcome should download and install that channel immediately. The
// bundled desktop executable is only a bootstrap for the installed runtime, so
// the first launch fetches the real build.
func platformInstallsProductOnWelcome() bool { return true }
