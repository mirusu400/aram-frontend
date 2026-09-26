package frontend

import "sync"

// NativeShortcutHost pins one title to the device launcher. The native host
// owns the launcher API and keeps the title's private path out of shortcut
// intents; the shared frontend only supplies the selected title and icon.
type NativeShortcutHost interface {
	PinGameShortcut(path, title string, iconPNG []byte) error
}

var nativeShortcutBridge struct {
	sync.RWMutex
	host NativeShortcutHost
}

func SetNativeShortcutHost(host NativeShortcutHost) {
	nativeShortcutBridge.Lock()
	nativeShortcutBridge.host = host
	nativeShortcutBridge.Unlock()
}

func currentNativeShortcutHost() NativeShortcutHost {
	nativeShortcutBridge.RLock()
	defer nativeShortcutBridge.RUnlock()
	return nativeShortcutBridge.host
}
