package frontend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	gitHubAPIBase       = "https://api.github.com"
	gitHubOwner         = "mirusu400"
	maxReleaseBodyBytes = 4 << 20
	maxUpdateBytes      = int64(2 << 30)
)

type gitHubUpdater struct {
	client       *http.Client
	apiBase      string
	owner        string
	downloadRoot func() (string, error)
	goos         string
	goarch       string
}

type gitHubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Prerelease  bool                 `json:"prerelease"`
	PublishedAt time.Time            `json:"published_at"`
	Assets      []gitHubReleaseAsset `json:"assets"`
}

type gitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type releaseNotFoundError struct {
	repository string
	channel    updateChannel
}

func (err *releaseNotFoundError) Error() string {
	if err.channel == updateChannelNightly {
		return fmt.Sprintf(
			"%s has no published Nightly release yet; push its "+
				"main-branch Nightly workflow to publish one",
			err.repository,
		)
	}
	return fmt.Sprintf(
		"no %s release is published for %s",
		updateChannelLabel(err.channel),
		err.repository,
	)
}

func newGitHubUpdater() *gitHubUpdater {
	return &gitHubUpdater{
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
		apiBase:      gitHubAPIBase,
		owner:        gitHubOwner,
		downloadRoot: defaultUpdateDownloadRoot,
		goos:         runtime.GOOS,
		goarch:       runtime.GOARCH,
	}
}

func (updater *gitHubUpdater) Download(
	ctx context.Context,
	component updateComponent,
	channel updateChannel,
) (updateDownload, error) {
	info, ok := updateInfo(component)
	if !ok {
		return updateDownload{}, fmt.Errorf("unknown update component %q", component)
	}
	channel = normalizeUpdateChannel(string(channel))
	assetName, err := updateAssetName(component, updater.goos, updater.goarch)
	if err != nil {
		return updateDownload{}, err
	}
	release, err := updater.fetchRelease(ctx, info.Repository, channel)
	if err != nil {
		return updateDownload{}, err
	}
	asset, found := findReleaseAsset(release.Assets, assetName)
	if !found {
		return updateDownload{}, fmt.Errorf(
			"%s %s does not include %s",
			info.DisplayName,
			updateChannelLabel(channel),
			assetName,
		)
	}
	version := releaseVersion(release, channel)
	path, err := updater.downloadAsset(
		ctx,
		info.Repository,
		channel,
		version,
		asset,
	)
	if err != nil {
		return updateDownload{}, err
	}
	return updateDownload{
		Component: component,
		Channel:   channel,
		Version:   version,
		Path:      path,
	}, nil
}

// CheckLatest reports the version string of the newest published release for
// the component on the channel without downloading anything. It is the
// metadata half of Download, used by the background startup update check.
func (updater *gitHubUpdater) CheckLatest(
	ctx context.Context,
	component updateComponent,
	channel updateChannel,
) (string, error) {
	info, ok := updateInfo(component)
	if !ok {
		return "", fmt.Errorf("unknown update component %q", component)
	}
	channel = normalizeUpdateChannel(string(channel))
	release, err := updater.fetchRelease(ctx, info.Repository, channel)
	if err != nil {
		return "", err
	}
	return releaseVersion(release, channel), nil
}

// releaseVersion picks the human version a release advertises: the Nightly
// release rewrites its rolling tag on every build, so its Name carries the
// identity; a Stable release is identified by its tag.
func releaseVersion(release gitHubRelease, channel updateChannel) string {
	version := strings.TrimSpace(release.TagName)
	if channel == updateChannelNightly && strings.TrimSpace(release.Name) != "" {
		version = strings.TrimSpace(release.Name)
	}
	if version == "" {
		version = updateChannelLabel(channel)
	}
	return version
}

func (updater *gitHubUpdater) fetchRelease(
	ctx context.Context,
	repository string,
	channel updateChannel,
) (gitHubRelease, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s/releases/latest",
		strings.TrimRight(updater.apiBase, "/"),
		updater.owner,
		repository,
	)
	if channel == updateChannelNightly {
		endpoint = fmt.Sprintf(
			"%s/repos/%s/%s/releases/tags/nightly",
			strings.TrimRight(updater.apiBase, "/"),
			updater.owner,
			repository,
		)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return gitHubRelease{}, err
	}
	setGitHubHeaders(request)
	response, err := updater.client.Do(request)
	if err != nil {
		return gitHubRelease{}, fmt.Errorf(
			"%s release lookup failed: %w",
			updateChannelLabel(channel),
			err,
		)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return gitHubRelease{}, &releaseNotFoundError{
			repository: repository,
			channel:    channel,
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return gitHubRelease{}, gitHubStatusError("release lookup", response)
	}
	var release gitHubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReleaseBodyBytes))
	if err := decoder.Decode(&release); err != nil {
		return gitHubRelease{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	return release, nil
}

func (updater *gitHubUpdater) downloadAsset(
	ctx context.Context,
	repository string,
	channel updateChannel,
	version string,
	asset gitHubReleaseAsset,
) (string, error) {
	if err := updater.validateAssetURL(asset.BrowserDownloadURL); err != nil {
		return "", err
	}
	if asset.Size > maxUpdateBytes {
		return "", fmt.Errorf("%s is larger than the 2 GiB download limit", asset.Name)
	}
	root, err := updater.downloadRoot()
	if err != nil {
		return "", fmt.Errorf("locate Downloads folder: %w", err)
	}
	directory := filepath.Join(
		root,
		repository,
		string(channel),
		safePathSegment(version),
	)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create update folder: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		asset.BrowserDownloadURL,
		nil,
	)
	if err != nil {
		return "", err
	}
	setGitHubHeaders(request)
	response, err := updater.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", gitHubStatusError("asset download", response)
	}

	temporary, err := os.CreateTemp(directory, "."+asset.Name+".*.part")
	if err != nil {
		return "", fmt.Errorf("create temporary update: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		_ = temporary.Close()
		if !keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	hasher := sha256.New()
	written, copyErr := io.Copy(
		io.MultiWriter(temporary, hasher),
		io.LimitReader(response.Body, maxUpdateBytes+1),
	)
	if copyErr != nil {
		return "", fmt.Errorf("write %s: %w", asset.Name, copyErr)
	}
	if written > maxUpdateBytes {
		return "", fmt.Errorf("%s exceeded the 2 GiB download limit", asset.Name)
	}
	if asset.Size > 0 && written != asset.Size {
		return "", fmt.Errorf(
			"%s size mismatch: received %d bytes, expected %d",
			asset.Name,
			written,
			asset.Size,
		)
	}
	if err := verifyReleaseDigest(asset.Digest, hasher.Sum(nil)); err != nil {
		return "", fmt.Errorf("%s: %w", asset.Name, err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("flush %s: %w", asset.Name, err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", asset.Name, err)
	}

	finalPath := availableDownloadPath(filepath.Join(directory, asset.Name))
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", fmt.Errorf("finish %s: %w", asset.Name, err)
	}
	keepTemporary = true
	return finalPath, nil
}

func (updater *gitHubUpdater) validateAssetURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid release asset URL: %w", err)
	}
	apiURL, _ := url.Parse(updater.apiBase)
	if apiURL != nil && apiURL.Host != "" && parsed.Host == apiURL.Host {
		return nil
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") {
		return errors.New("GitHub release returned an untrusted asset URL")
	}
	return nil
}

func findReleaseAsset(
	assets []gitHubReleaseAsset,
	name string,
) (gitHubReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return gitHubReleaseAsset{}, false
}

func verifyReleaseDigest(digest string, actual []byte) error {
	digest = strings.TrimSpace(strings.ToLower(digest))
	if digest == "" {
		return nil
	}
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return fmt.Errorf("unsupported release digest %q", digest)
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(digest, prefix))
	if err != nil || len(expected) != sha256.Size {
		return errors.New("release SHA-256 digest is malformed")
	}
	if !equalBytes(expected, actual) {
		return errors.New("SHA-256 verification failed")
	}
	return nil
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func gitHubStatusError(operation string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		detail = response.Status
	}
	return fmt.Errorf("GitHub %s failed (%s): %s", operation, response.Status, detail)
}

func setGitHubHeaders(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ARAM-Updater")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '.',
			character == '-',
			character == '_':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
		if builder.Len() == 80 {
			break
		}
	}
	result := strings.Trim(builder.String(), "._")
	if result == "" {
		return "update"
	}
	return result
}

func availableDownloadPath(path string) string {
	if _, err := os.Stat(path); err != nil {
		return path
	}
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(path, extension)
	if strings.HasSuffix(strings.ToLower(path), ".tar.gz") {
		extension = ".tar.gz"
		base = strings.TrimSuffix(path, extension)
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, index, extension)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}
