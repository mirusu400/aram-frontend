//go:build !js || !wasm

package frontend

// systemThemeMode is the default UI theme on hosts that do not detect an OS or
// browser color scheme. Desktop and mobile keep the historical "light" default;
// the web build overrides this to read the browser's prefers-color-scheme
// (theme_web.go), so a first visit inherits the viewer's dark/light preference.
func systemThemeMode() string { return "light" }
