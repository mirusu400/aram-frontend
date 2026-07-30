//go:build (windows || linux || darwin) && !android && !ios

package frontend

import "github.com/hajimehoshi/ebiten/v2"

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

func Run(backend Backend, initialPath string) error {
	ebiten.SetWindowTitle("ARAM - Archived Runtime for ARM Mobiles")
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	ebiten.SetWindowSizeLimits(720, 540, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	shell := NewShell(backend, NewPlatformPicker(), initialPath)
	defer shell.closeAudio()
	return ebiten.RunGame(shell)
}
