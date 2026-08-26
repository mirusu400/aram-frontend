//go:build js && wasm

package frontend

import "syscall/js"

// systemThemeMode reads the browser's prefers-color-scheme so a first visit
// starts ARAM in the viewer's OS/browser theme. It only sets the default used
// when no theme has been saved yet; once the user picks a theme, that saved
// choice wins on later loads. Falls back to "light" if matchMedia is missing or
// throws (older or locked-down contexts).
func systemThemeMode() (mode string) {
	mode = "light"
	defer func() { _ = recover() }()
	global := js.Global()
	if !global.Truthy() {
		return
	}
	mql := global.Call("matchMedia", "(prefers-color-scheme: dark)")
	if mql.Truthy() && mql.Get("matches").Bool() {
		mode = "dark"
	}
	return
}
