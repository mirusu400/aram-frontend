package frontend

import (
	"errors"
	"sync"
)

var ErrNativeURLUnavailable = errors.New("the native host cannot open links")

// NativeURLHost is implemented by a mobile application layer that can hand an
// already validated HTTP(S) URL to the operating system browser.
type NativeURLHost interface {
	OpenURL(rawURL string) error
}

var nativeURLBridge struct {
	sync.RWMutex
	host NativeURLHost
}

func SetNativeURLHost(host NativeURLHost) {
	nativeURLBridge.Lock()
	nativeURLBridge.host = host
	nativeURLBridge.Unlock()
}

func openNativeURL(rawURL string) error {
	nativeURLBridge.RLock()
	host := nativeURLBridge.host
	nativeURLBridge.RUnlock()
	if host == nil {
		return ErrNativeURLUnavailable
	}
	return host.OpenURL(rawURL)
}
