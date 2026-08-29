package frontend

import (
	"context"
	"fmt"
)

type updateChannel string

const (
	updateChannelStable  updateChannel = "stable"
	updateChannelNightly updateChannel = "nightly"
)

type updateComponent string

const (
	updateComponentProduct  updateComponent = "aram-emu"
	updateComponentCore     updateComponent = "aram-core"
	updateComponentFrontend updateComponent = "aram-frontend"
)

type updateComponentInfo struct {
	Repository  string
	DisplayName string
	AssetPrefix string
}

type updateDownload struct {
	Component updateComponent
	Channel   updateChannel
	Version   string
	Path      string
}

type updateResult struct {
	component updateComponent
	download  updateDownload
	err       error
}

type updateProgress struct {
	Busy    bool
	Message string
	Path    string
	Version string
}

type updateDownloader interface {
	Download(context.Context, updateComponent, updateChannel) (updateDownload, error)
}

// updateChecker fetches the newest published version for a component without
// downloading it. The real gitHubUpdater satisfies both interfaces; the
// startup check type-asserts for this one so test doubles that only download
// stay valid.
type updateChecker interface {
	CheckLatest(context.Context, updateComponent, updateChannel) (string, error)
}

// updateCheckResult carries a background version check back to the shell loop.
type updateCheckResult struct {
	component updateComponent
	channel   updateChannel
	version   string
	err       error
}

func updateInfo(component updateComponent) (updateComponentInfo, bool) {
	switch component {
	case updateComponentProduct:
		return updateComponentInfo{
			Repository:  "aram-emu",
			DisplayName: "ARAM product",
			AssetPrefix: "aram",
		}, true
	case updateComponentCore:
		return updateComponentInfo{
			Repository:  "aram-core",
			DisplayName: "aram-core developer tools",
			AssetPrefix: "aram-core",
		}, true
	case updateComponentFrontend:
		return updateComponentInfo{
			Repository:  "aram-frontend",
			DisplayName: "aram-frontend",
			AssetPrefix: "aram-frontend",
		}, true
	default:
		return updateComponentInfo{}, false
	}
}

func normalizeUpdateChannel(value string) updateChannel {
	if updateChannel(value) == updateChannelNightly {
		return updateChannelNightly
	}
	return updateChannelStable
}

func updateChannelLabel(channel updateChannel) string {
	if channel == updateChannelNightly {
		return "Nightly"
	}
	return "Stable"
}

func updateAssetName(
	component updateComponent,
	goos string,
	goarch string,
) (string, error) {
	info, ok := updateInfo(component)
	if !ok {
		return "", fmt.Errorf("unknown update component %q", component)
	}
	var platform string
	switch {
	case goos == "windows" && goarch == "amd64":
		platform = "windows-amd64.zip"
	case goos == "linux" && goarch == "amd64":
		platform = "linux-amd64.tar.gz"
	case goos == "darwin" && goarch == "arm64":
		platform = "macos-arm64.tar.gz"
	case goos == "android" && component == updateComponentProduct:
		// The product Nightly ships one APK carrying both arm64-v8a and
		// x86_64 native libraries; the developer archives have no Android
		// build.
		platform = "android-universal.apk"
	default:
		return "", fmt.Errorf(
			"%s updates are not built for %s/%s",
			info.DisplayName,
			goos,
			goarch,
		)
	}
	return info.AssetPrefix + "-" + platform, nil
}
