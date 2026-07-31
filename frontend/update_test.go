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
	if _, err := updateAssetName(
		updateComponentProduct,
		"darwin",
		"amd64",
	); err == nil {
		t.Fatal("unsupported platform unexpectedly has an update asset")
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
