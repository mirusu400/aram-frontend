//go:build android || ios

package frontend

func setPlatformWindowTitle(string) {}

func togglePlatformFullscreen() string {
	return "Fullscreen is managed by the mobile host"
}
