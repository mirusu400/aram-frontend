//go:build !windows && !linux && !darwin && !android && !ios && !js

package frontend

func setPlatformWindowTitle(string) {}

func togglePlatformFullscreen() string {
	return "Fullscreen is unavailable on this platform"
}

func fitPlatformWindow() string {
	return "Window sizing is unavailable on this platform"
}

func platformUsesTouchLayout() bool { return false }
