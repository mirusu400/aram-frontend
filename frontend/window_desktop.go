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
	return RunWithOptions(backend, initialPath, false)
}

// RunWithOptions lets the integrated desktop bootstrap reopen the native file
// picker after it installs and relaunches a selected update channel.
func RunWithOptions(
	backend Backend,
	initialPath string,
	openOnStart bool,
) error {
	ebiten.SetWindowTitle("ARAM - Archived Runtime for ARM Mobiles")
	ebiten.SetWindowIcon(appIcons())
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	ebiten.SetWindowSizeLimits(720, 540, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	shell := NewShell(backend, NewPlatformPicker(), initialPath)
	if openOnStart && initialPath == "" {
		shell.DispatchExternalCommand("file.open")
	}
	defer shell.closeAudio()
	return runGameWithCrashReporting(shell, func() error {
		return ebiten.RunGame(shell)
	})
}
