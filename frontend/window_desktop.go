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

// RunWithOptions is the integrated desktop bootstrap entry point. The trailing
// bool once reopened the native file picker after the bootstrap installed and
// relaunched a selected update channel; that unsolicited popup was removed, so
// the flag is now accepted for the aram-emu call sites but ignored. The app
// relaunches to its normal idle state and the user opens a file when they want.
func RunWithOptions(
	backend Backend,
	initialPath string,
	_ bool,
) error {
	ebiten.SetWindowTitle("ARAM - Archived Runtime for ARM Mobiles")
	ebiten.SetWindowIcon(appIcons())
	ebiten.SetWindowSize(logicalWidth, logicalHeight)
	ebiten.SetWindowSizeLimits(720, 540, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	shell := NewShell(backend, NewPlatformPicker(), initialPath)
	// Enable the native fault dialog on the real desktop run only. Tests build
	// their shells with NewShell directly and must never raise a real dialog.
	shell.faultPrompter = platformReportPrompter
	defer shell.closeAudio()
	return runGameWithCrashReporting(shell, func() error {
		return ebiten.RunGame(shell)
	})
}
