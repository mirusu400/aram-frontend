//go:build android || ios

package frontend

import (
	"errors"
	"sync"
)

// Mobile hosts have no user-visible Downloads folder that Go code may write
// to, so the native host names an app-private update folder before the shell
// starts. The same host later reads packages back from that folder when it
// hands them to the platform installer.
var updateDownloadBridge struct {
	sync.RWMutex
	root string
}

// SetUpdateDownloadRoot records the app-private folder that receives verified
// update packages. An empty root disables downloads again.
func SetUpdateDownloadRoot(root string) {
	updateDownloadBridge.Lock()
	updateDownloadBridge.root = root
	updateDownloadBridge.Unlock()
}

func defaultUpdateDownloadRoot() (string, error) {
	updateDownloadBridge.RLock()
	root := updateDownloadBridge.root
	updateDownloadBridge.RUnlock()
	if root == "" {
		return "", errors.New("the native host has not configured an update folder")
	}
	return root, nil
}

// productInstallActionLabel names the ARAM product row action when the host
// can install the downloaded build. Mobile hosts hand the package to the
// platform installer, which finishes without restarting the app itself.
func productInstallActionLabel() string { return "Install" }

// platformInstallsProductOnWelcome reports whether choosing a channel on the
// first-run Welcome should download and install that channel immediately. A
// mobile package is the complete product rather than a bootstrap, so Welcome
// only records the channel and the Updates settings row installs on demand.
func platformInstallsProductOnWelcome() bool { return false }
