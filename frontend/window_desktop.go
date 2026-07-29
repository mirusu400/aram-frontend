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

func Run(backend Backend, initialPath string) error {
	ebiten.SetWindowTitle("ARAM — Archived Runtime for ARM Mobiles")
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(NewShell(backend, NewPlatformPicker(), initialPath))
}
