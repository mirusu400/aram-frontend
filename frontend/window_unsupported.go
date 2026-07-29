//go:build !windows && !linux && !darwin && !android && !ios

package frontend

func setPlatformWindowTitle(string) {}

func togglePlatformFullscreen() string {
	return "Fullscreen is unavailable on this platform"
}
