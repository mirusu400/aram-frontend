//go:build js && wasm

package frontend

import (
	"errors"
	"fmt"
	"syscall/js"
)

func openPlatformURL(rawURL string) (err error) {
	var opened js.Value
	defer func() {
		if recovered := recover(); recovered != nil {
			closeWebWindow(opened)
			err = fmt.Errorf("the browser rejected the external link: %v", recovered)
		}
	}()

	// Supplying "noopener" as a window feature makes some browsers return
	// null even when the tab opened, which is indistinguishable from a popup
	// block. Open a same-origin blank tab first, sever its opener immediately,
	// then navigate it so we retain reliable failure detection without exposing
	// the ARAM page to the guest-controlled destination.
	opened = js.Global().Call("open", "", "_blank")
	if opened.IsNull() || opened.IsUndefined() {
		return errors.New("the browser blocked the external link popup")
	}
	opened.Set("opener", js.Null())
	opened.Get("location").Call("replace", rawURL)
	return nil
}

func closeWebWindow(opened js.Value) {
	if opened.IsNull() || opened.IsUndefined() {
		return
	}
	// A browser that rejected navigation may also reject close(). This is only
	// cleanup for the blank tab, so never let a second JavaScript exception mask
	// the useful error returned to the confirmation panel.
	defer func() { _ = recover() }()
	opened.Call("close")
}
