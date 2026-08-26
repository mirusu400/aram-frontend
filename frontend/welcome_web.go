//go:build js && wasm

package frontend

// platformSupportsWelcome is false on the web/wasm build. The browser runtime
// cannot install or self-update the integrated product, so the Nightly/Stable
// channel picker has nothing to configure; the shell skips Welcome and opens
// the file picker directly.
func platformSupportsWelcome() bool { return false }
