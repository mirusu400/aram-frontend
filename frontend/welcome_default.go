//go:build !js || !wasm

package frontend

// platformSupportsWelcome reports whether the first-run Welcome channel picker
// applies to this platform. Desktop hosts bootstrap the installed runtime and
// mobile hosts record the channel for on-demand installs, so both present it.
func platformSupportsWelcome() bool { return true }
