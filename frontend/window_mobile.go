//go:build android || ios

package frontend

func setPlatformWindowTitle(string) {}

func togglePlatformFullscreen() string {
	return "Fullscreen is managed by the mobile host"
}

func fitPlatformWindow() string {
	return "Window sizing is managed by the mobile host"
}

func platformUsesTouchLayout() bool { return true }
