package frontend

import (
	"errors"
	"sync"
)

var (
	// ErrFolderBrowserUnavailable reports that the platform cannot reveal a
	// directory to the user. A handset is the case that matters: there is no
	// file manager to point at an app-private folder, so the shell hands the
	// file itself to another app instead.
	ErrFolderBrowserUnavailable = errors.New("the native host does not expose a folder browser")
	// ErrShareUnavailable reports that no native host is attached to hand a
	// file to another app. Desktop builds always report this; they reveal the
	// containing folder instead.
	ErrShareUnavailable = errors.New("the native host cannot share a file")
)

// saveBackupMIMEType is what a host advertises when it offers an .aramsave
// file to another app. The format is ARAM's own, so the generic binary type is
// the honest answer; it keeps every target app (cloud drive, messenger, the
// system file manager) in the chooser.
const saveBackupMIMEType = "application/octet-stream"

// NativeShareHost is implemented by a mobile application layer that can hand a
// file below the app's private storage to another app - Android's share sheet,
// iOS's activity view. It is the only way a save backup written into private
// storage can reach a place that survives uninstalling the app.
type NativeShareHost interface {
	ShareFile(path, mimeType, title string) error
}

var nativeShareBridge struct {
	sync.RWMutex
	host NativeShareHost
}

// SetNativeShareHost connects the running native Activity. Passing nil detaches
// it, which is what a host does as it goes away.
func SetNativeShareHost(host NativeShareHost) {
	nativeShareBridge.Lock()
	nativeShareBridge.host = host
	nativeShareBridge.Unlock()
}

func currentNativeShareHost() NativeShareHost {
	nativeShareBridge.RLock()
	defer nativeShareBridge.RUnlock()
	return nativeShareBridge.host
}

// shareNativeFile offers one file to another app. It reports
// ErrShareUnavailable when the platform has no such host, so a caller can fall
// back to a desktop behaviour without treating the absence as a failure.
var shareNativeFile = func(path, mimeType, title string) error {
	host := currentNativeShareHost()
	if host == nil {
		return ErrShareUnavailable
	}
	return host.ShareFile(path, mimeType, title)
}
