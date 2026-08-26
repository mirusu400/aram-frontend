//go:build js && wasm

package frontend

import "github.com/hajimehoshi/ebiten/v2"

// This file is the web/wasm counterpart of window_desktop.go. Ebitengine runs
// the shell against a browser <canvas>; there is no OS window, so title and
// fullscreen route through the same ebiten calls (which act on the document /
// canvas) and the sizing helpers stay best-effort. The browser has no readable
// filesystem path for a user selection, so Run wires the web picker's byte sink
// to Shell.OpenExternalBytes before starting the game loop.

func setPlatformWindowTitle(title string) {
	ebiten.SetWindowTitle(title)
}

func togglePlatformFullscreen() string {
	ebiten.SetFullscreen(!ebiten.IsFullscreen())
	if ebiten.IsFullscreen() {
		return "Fullscreen enabled"
	}
	return "Fullscreen disabled"
}

func fitPlatformWindow() string {
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	return "Window restored to 960x720"
}

func platformUsesTouchLayout() bool { return false }

// Run boots the shell on the web. initialPath is unused on wasm (the browser
// hands input bytes through the picker), but the signature matches the desktop
// Run so a web cmd wires the backend the same way.
func Run(backend Backend, initialPath string) error {
	ebiten.SetWindowTitle("ARAM - Archived Runtime for ARM Mobiles")
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	shell := NewShell(backend, NewPlatformPicker(), "")
	setWebInputSink(shell.OpenExternalBytes)
	defer shell.closeAudio()
	return ebiten.RunGame(shell)
}
