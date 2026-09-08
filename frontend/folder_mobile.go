//go:build android || ios

package frontend

func openPlatformFolder(string) error {
	return ErrFolderBrowserUnavailable
}
