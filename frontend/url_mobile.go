//go:build android || ios

package frontend

func openPlatformURL(rawURL string) error {
	return openNativeURL(rawURL)
}
