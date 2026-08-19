package frontend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitHubUpdaterDownloadsAndVerifiesStableAsset(t *testing.T) {
	payload := []byte("checked ARAM update")
	digest := sha256.Sum256(payload)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/repos/mirusu400/aram-core/releases/latest":
			_ = json.NewEncoder(writer).Encode(gitHubRelease{
				TagName: "v1.2.3",
				Name:    "ARAM Core 1.2.3",
				Assets: []gitHubReleaseAsset{{
					Name:               "aram-core-windows-amd64.zip",
					BrowserDownloadURL: server.URL + "/assets/core.zip",
					Size:               int64(len(payload)),
					Digest:             "sha256:" + hex.EncodeToString(digest[:]),
				}},
			})
		case "/assets/core.zip":
			_, _ = writer.Write(payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	updater := &gitHubUpdater{
		client:       server.Client(),
		apiBase:      server.URL,
		owner:        "mirusu400",
		downloadRoot: func() (string, error) { return root, nil },
		goos:         "windows",
		goarch:       "amd64",
	}
	download, err := updater.Download(
		context.Background(),
		updateComponentCore,
		updateChannelStable,
	)
	if err != nil {
		t.Fatal(err)
	}
	if download.Version != "v1.2.3" {
		t.Fatalf("download version = %q", download.Version)
	}
	if !strings.Contains(
		filepath.ToSlash(download.Path),
		"/aram-core/stable/v1.2.3/aram-core-windows-amd64.zip",
	) {
		t.Fatalf("download path = %q", download.Path)
	}
	data, err := os.ReadFile(download.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("download payload = %q", data)
	}
}

func TestGitHubUpdaterUsesRollingNightlyRelease(t *testing.T) {
	payload := []byte("nightly")
	var requestedPaths []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requestedPaths = append(requestedPaths, request.URL.Path)
		if request.URL.Path == "/repos/mirusu400/aram-frontend/releases/tags/nightly" {
			_ = json.NewEncoder(writer).Encode(gitHubRelease{
				TagName:    "nightly",
				Name:       "Nightly abc1234",
				Prerelease: true,
				Assets: []gitHubReleaseAsset{{
					Name:               "aram-frontend-linux-amd64.tar.gz",
					BrowserDownloadURL: server.URL + "/frontend.tar.gz",
					Size:               int64(len(payload)),
				}},
			})
			return
		}
		if request.URL.Path == "/frontend.tar.gz" {
			_, _ = writer.Write(payload)
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()

	root := t.TempDir()
	updater := &gitHubUpdater{
		client:       server.Client(),
		apiBase:      server.URL,
		owner:        "mirusu400",
		downloadRoot: func() (string, error) { return root, nil },
		goos:         "linux",
		goarch:       "amd64",
	}
	download, err := updater.Download(
		context.Background(),
		updateComponentFrontend,
		updateChannelNightly,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(requestedPaths) != 2 ||
		requestedPaths[0] != "/repos/mirusu400/aram-frontend/releases/tags/nightly" ||
		requestedPaths[1] != "/frontend.tar.gz" {
		t.Fatalf("requested paths = %v", requestedPaths)
	}
	if download.Version != "Nightly abc1234" {
		t.Fatalf("nightly version = %q", download.Version)
	}
}

func TestGitHubUpdaterExplainsUnpublishedNightlyRelease(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	updater := &gitHubUpdater{
		client:       server.Client(),
		apiBase:      server.URL,
		owner:        "mirusu400",
		downloadRoot: func() (string, error) { return t.TempDir(), nil },
		goos:         "windows",
		goarch:       "amd64",
	}
	_, err := updater.Download(
		context.Background(),
		updateComponentCore,
		updateChannelNightly,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "aram-core") ||
		!strings.Contains(err.Error(), "main-branch Nightly workflow") {
		t.Fatalf("unpublished Nightly error = %v", err)
	}
}

func TestGitHubUpdaterRejectsDigestMismatchAndCleansTemporaryFile(t *testing.T) {
	payload := []byte("tampered")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if strings.HasSuffix(request.URL.Path, "/releases/latest") {
			_ = json.NewEncoder(writer).Encode(gitHubRelease{
				TagName: "v1",
				Assets: []gitHubReleaseAsset{{
					Name:               "aram-windows-amd64.zip",
					BrowserDownloadURL: server.URL + "/aram.zip",
					Size:               int64(len(payload)),
					Digest:             "sha256:" + strings.Repeat("0", 64),
				}},
			})
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	root := t.TempDir()
	updater := &gitHubUpdater{
		client:       server.Client(),
		apiBase:      server.URL,
		owner:        "mirusu400",
		downloadRoot: func() (string, error) { return root, nil },
		goos:         "windows",
		goarch:       "amd64",
	}
	_, err := updater.Download(
		context.Background(),
		updateComponentProduct,
		updateChannelStable,
	)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("digest error = %v", err)
	}
	var files []string
	walkErr := filepath.Walk(root, func(
		path string,
		info os.FileInfo,
		err error,
	) error {
		if err == nil && info != nil && !info.IsDir() {
			files = append(files, path)
		}
		return err
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if len(files) != 0 {
		t.Fatalf("failed download left files: %v", files)
	}
}

func TestUpdateAssetNamesMatchPublishedArchives(t *testing.T) {
	tests := []struct {
		component updateComponent
		goos      string
		goarch    string
		want      string
	}{
		{updateComponentProduct, "windows", "amd64", "aram-windows-amd64.zip"},
		{updateComponentCore, "linux", "amd64", "aram-core-linux-amd64.tar.gz"},
		{updateComponentFrontend, "darwin", "arm64", "aram-frontend-macos-arm64.tar.gz"},
		{updateComponentProduct, "android", "arm64", "aram-android-universal.apk"},
		{updateComponentProduct, "android", "amd64", "aram-android-universal.apk"},
	}
	for _, test := range tests {
		got, err := updateAssetName(test.component, test.goos, test.goarch)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("asset name = %q, want %q", got, test.want)
		}
	}
	unsupported := []struct {
		component updateComponent
		goos      string
		goarch    string
	}{
		{updateComponentProduct, "darwin", "amd64"},
		// Only the integrated product publishes an Android package; the
		// developer archives must stay disabled there.
		{updateComponentCore, "android", "arm64"},
		{updateComponentFrontend, "android", "arm64"},
	}
	for _, test := range unsupported {
		if _, err := updateAssetName(
			test.component,
			test.goos,
			test.goarch,
		); err == nil {
			t.Fatalf(
				"%s on %s/%s unexpectedly has an update asset",
				test.component,
				test.goos,
				test.goarch,
			)
		}
	}
}

func TestDeferredProductInstallKeepsArchiveAndStaysRunning(t *testing.T) {
	isolateSettings(t)
	directory := t.TempDir()
	archive := filepath.Join(directory, "aram-android-universal.apk")
	if err := os.WriteFile(archive, []byte("product package"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
		err:     ErrProductInstallDeferred,
	}
	shell := NewShell(backend, nil, "")

	shell.installProductUpdate(updateDownload{
		Component: updateComponentProduct,
		Channel:   updateChannelNightly,
		Version:   "Nightly 760f53f",
		Path:      archive,
	}, false)

	select {
	case installed := <-backend.updates:
		if installed.ArchivePath != archive {
			t.Fatalf("deferred product = %#v", installed)
		}
	case <-time.After(time.Second):
		t.Fatal("product update was not handed to the installer")
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("the package still needed by the platform installer was removed: %v", err)
	}
	if shell.quitting {
		t.Fatal("a deferred install requested a restart")
	}
	progress := shell.updateProgress[updateComponentProduct]
	want := shell.trf(
		"Finish installing %s in the system installer",
		"Nightly 760f53f",
	)
	if progress.Busy || progress.Path != archive ||
		progress.Version != "Nightly 760f53f" || progress.Message != want {
		t.Fatalf("deferred install progress = %#v", progress)
	}
	if shell.status != want {
		t.Fatalf("deferred install status = %q, want %q", shell.status, want)
	}
}

type fakeUpdateDownloader struct {
	download updateDownload
	err      error
	requests chan updateDownload
}

func (downloader *fakeUpdateDownloader) Download(
	_ context.Context,
	component updateComponent,
	channel updateChannel,
) (updateDownload, error) {
	downloader.requests <- updateDownload{
		Component: component,
		Channel:   channel,
	}
	return downloader.download, downloader.err
}

func TestShellUpdateDownloadKeepsErrorsScopedToOriginatingComponent(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	downloader := &fakeUpdateDownloader{
		err:      errors.New("release unavailable"),
		requests: make(chan updateDownload, 1),
	}
	shell := NewShell(NullBackend{}, nil, "")
	shell.updater = downloader
	shell.settings.UpdateChannel = string(updateChannelNightly)

	shell.downloadUpdate(updateComponentFrontend)
	select {
	case request := <-downloader.requests:
		if request.Component != updateComponentFrontend ||
			request.Channel != updateChannelNightly {
			t.Fatalf("update request = %#v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("update request was not dispatched")
	}
	select {
	case result := <-shell.updateResults:
		shell.consumeUpdateResult(result)
	case <-time.After(time.Second):
		t.Fatal("update result was not delivered")
	}
	progress := shell.updateProgress[updateComponentFrontend]
	if progress.Busy || !strings.Contains(progress.Message, "release unavailable") {
		t.Fatalf("frontend update progress = %#v", progress)
	}
	if core := shell.updateProgress[updateComponentCore]; core.Message != "" {
		t.Fatalf("core progress was changed by frontend failure: %#v", core)
	}
}

func TestManualProductDownloadInstallsRestartsAndReopensCurrentInput(t *testing.T) {
	isolateSettings(t)
	settings := defaultSettings()
	settings.WelcomeCompleted = true
	settings.UpdateChannel = string(updateChannelNightly)
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
	}
	download := updateDownload{
		Component: updateComponentProduct,
		Channel:   updateChannelNightly,
		Version:   "Nightly 760f53f",
		Path:      filepath.Join(t.TempDir(), "aram-windows-amd64.zip"),
	}
	downloader := &fakeUpdateDownloader{
		download: download,
		requests: make(chan updateDownload, 1),
	}
	shell := NewShell(backend, nil, "")
	shell.updater = downloader
	shell.selectedPath = filepath.Join(t.TempDir(), "game.zip")

	shell.downloadUpdate(updateComponentProduct)
	select {
	case request := <-downloader.requests:
		if request.Component != updateComponentProduct {
			t.Fatalf("manual product request = %#v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("manual product download was not dispatched")
	}
	select {
	case result := <-shell.updateResults:
		shell.consumeUpdateResult(result)
	case <-time.After(time.Second):
		t.Fatal("manual product download did not complete")
	}
	select {
	case installed := <-backend.updates:
		if installed.ArchivePath != download.Path ||
			installed.RelaunchPath != shell.selectedPath {
			t.Fatalf("manual installed product = %#v", installed)
		}
	case <-time.After(time.Second):
		t.Fatal("manual product download was not installed")
	}
	if !shell.quitting {
		t.Fatal("manual product install did not request restart")
	}
}

func TestSettingsNormalizeUpdateChannel(t *testing.T) {
	settings := defaultSettings()
	if settings.UpdateChannel != string(updateChannelStable) {
		t.Fatalf("default update channel = %q", settings.UpdateChannel)
	}
	settings.UpdateChannel = "preview"
	settings.normalize()
	if settings.UpdateChannel != string(updateChannelStable) {
		t.Fatalf("normalized update channel = %q", settings.UpdateChannel)
	}
	settings.UpdateChannel = string(updateChannelNightly)
	settings.normalize()
	if settings.UpdateChannel != string(updateChannelNightly) {
		t.Fatalf("nightly update channel = %q", settings.UpdateChannel)
	}
}

func TestInstalledProductArchiveIsRemovedWithItsDownloadFolders(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	directory := filepath.Join(root, "aram-emu", "nightly", "Nightly-760f53f")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "aram-windows-amd64.zip")
	if err := os.WriteFile(archive, []byte("product archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
	}
	shell := NewShell(backend, nil, "")

	shell.installProductUpdate(updateDownload{
		Component: updateComponentProduct,
		Channel:   updateChannelNightly,
		Version:   "Nightly 760f53f",
		Path:      archive,
	}, false)

	select {
	case installed := <-backend.updates:
		if installed.ArchivePath != archive {
			t.Fatalf("installed product = %#v", installed)
		}
	case <-time.After(time.Second):
		t.Fatal("product update was not installed")
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("the installed archive was kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "aram-emu")); !os.IsNotExist(err) {
		t.Fatal("the emptied download folders were kept")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("deletion climbed past the download root: %v", err)
	}
	if progress := shell.updateProgress[updateComponentProduct]; progress.Path != "" {
		t.Fatalf("progress still offers the deleted archive: %#v", progress)
	}
}

func TestDownloadFolderSharedWithAnotherUpdateSurvives(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	directory := filepath.Join(root, "aram-emu", "nightly", "Nightly-760f53f")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "aram-windows-amd64.zip")
	if err := os.WriteFile(archive, []byte("product archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	neighbour := filepath.Join(directory, "aram-linux-amd64.tar.gz")
	if err := os.WriteFile(neighbour, []byte("other asset"), 0o600); err != nil {
		t.Fatal(err)
	}

	removeInstalledArchive(archive)

	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("the installed archive was kept: %v", err)
	}
	if _, err := os.Stat(neighbour); err != nil {
		t.Fatalf("deletion took a folder holding another download: %v", err)
	}
}

func TestFailedProductInstallKeepsTheDownloadedArchive(t *testing.T) {
	isolateSettings(t)
	directory := t.TempDir()
	archive := filepath.Join(directory, "aram-windows-amd64.zip")
	if err := os.WriteFile(archive, []byte("product archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
		err:     errors.New("runtime is busy"),
	}
	shell := NewShell(backend, nil, "")

	shell.installProductUpdate(updateDownload{
		Component: updateComponentProduct,
		Channel:   updateChannelNightly,
		Version:   "Nightly 760f53f",
		Path:      archive,
	}, false)

	select {
	case <-backend.updates:
	case <-time.After(time.Second):
		t.Fatal("product update was not attempted")
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("a failed install discarded the archive: %v", err)
	}
	if shell.quitting {
		t.Fatal("a failed install requested a restart")
	}
}
