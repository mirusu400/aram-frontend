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
	version := strings.TrimSpace(release.TagName)
	if channel == updateChannelNightly && strings.TrimSpace(release.Name) != "" {
		version = strings.TrimSpace(release.Name)
	}
	if version == "" {
		version = updateChannelLabel(channel)
	}
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

func defaultUpdateDownloadRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads", "ARAM"), nil
}

func (s *Shell) cycleUpdateChannel() {
	channel := normalizeUpdateChannel(s.settings.UpdateChannel)
	if channel == updateChannelStable {
		channel = updateChannelNightly
	} else {
		channel = updateChannelStable
	}
	s.settings.UpdateChannel = string(channel)
	_ = s.settings.save()
	s.invalidateSettingsPanel()
	s.setStatus(s.trf(
		"Update channel: %s",
		s.tr(updateChannelLabel(channel)),
	))
}

func (s *Shell) downloadUpdate(component updateComponent) bool {
	if _, err := updateAssetName(component, runtime.GOOS, runtime.GOARCH); err != nil {
		s.setStatus(s.tr("Update: ") + err.Error())
		return false
	}
	progress := s.updateProgress[component]
	if progress.Busy {
		s.setStatus(s.tr("Update download is already in progress"))
		return false
	}
	channel := normalizeUpdateChannel(s.settings.UpdateChannel)
	progress.Busy = true
	progress.Message = s.trf(
		"Looking up the latest %s build...",
		s.tr(updateChannelLabel(channel)),
	)
	s.updateProgress[component] = progress
	s.invalidateSettingsPanel()
	info, _ := updateInfo(component)
	s.setStatus(s.trf(
		"%s: checking %s...",
		s.tr(info.DisplayName),
		s.tr(updateChannelLabel(channel)),
	))
	downloader := s.updater
	if downloader == nil {
		downloader = newGitHubUpdater()
		s.updater = downloader
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		download, err := downloader.Download(ctx, component, channel)
		s.updateResults <- updateResult{
			component: component,
			download:  download,
			err:       err,
		}
	}()
	return true
}

func (s *Shell) consumeUpdateResult(result updateResult) {
	component := result.component
	progress := s.updateProgress[component]
	progress.Busy = false
	info, _ := updateInfo(component)
	if result.err != nil {
		if component == updateComponentProduct && s.welcomeInstalling {
			var notFound *releaseNotFoundError
			if normalizeUpdateChannel(s.settings.UpdateChannel) ==
				updateChannelStable &&
				errors.As(result.err, &notFound) {
				s.completeWelcomeWithBundledStable()
				return
			}
			s.welcomeInstalling = false
		}
		progress.Message = shorten(result.err.Error(), 100)
		s.updateProgress[component] = progress
		s.invalidateSettingsPanel()
		s.appendLog(s.trf(
			"%s update: %s",
			s.tr(info.DisplayName),
			result.err.Error(),
		))
		s.setStatus(s.trf(
			"%s: %s",
			s.tr(info.DisplayName),
			result.err.Error(),
		))
		return
	}
	if component == updateComponentProduct {
		if _, ok := s.backend.(ProductUpdateInstaller); ok {
			s.installProductUpdate(result.download, s.welcomeInstalling)
			return
		}
	}
	progress.Message = s.trf(
		"%s saved successfully",
		result.download.Version,
	)
	progress.Path = result.download.Path
	progress.Version = result.download.Version
	s.updateProgress[component] = progress
	s.invalidateSettingsPanel()
	s.appendLog(s.trf(
		"%s update saved: %s",
		s.tr(info.DisplayName),
		result.download.Path,
	))
	s.setStatus(s.trf(
		"%s saved: %s",
		s.tr(info.DisplayName),
		result.download.Path,
	))
}

func (s *Shell) installProductUpdate(
	download updateDownload,
	firstRun bool,
) {
	progress := s.updateProgress[updateComponentProduct]
	progress.Busy = true
	progress.Message = s.tr("Installing the integrated ARAM build...")
	s.updateProgress[updateComponentProduct] = progress
	s.invalidateSettingsPanel()

	installer, ok := s.backend.(ProductUpdateInstaller)
	if !ok {
		s.failProductInstall(errors.New(
			"the integrated product installer is not available",
		), firstRun)
		return
	}
	previousCompleted := s.settings.WelcomeCompleted
	if firstRun {
		s.settings.WelcomeCompleted = true
		if err := s.settings.save(); err != nil {
			s.settings.WelcomeCompleted = previousCompleted
			s.failProductInstall(
				fmt.Errorf("save Welcome settings: %w", err),
				firstRun,
			)
			return
		}
	}
	err := installer.InstallProductUpdate(ProductUpdate{
		Channel:      string(download.Channel),
		Version:      download.Version,
		ArchivePath:  download.Path,
		RelaunchPath: s.selectedPath,
	})
	if err != nil {
		if firstRun {
			s.settings.WelcomeCompleted = false
			_ = s.settings.save()
		}
		s.failProductInstall(err, firstRun)
		return
	}

	removeInstalledArchive(download.Path)

	progress.Busy = false
	progress.Message = s.tr("Installed; restarting ARAM...")
	progress.Path = ""
	progress.Version = download.Version
	s.updateProgress[updateComponentProduct] = progress
	if firstRun {
		s.welcomeInstalling = false
		s.panel = nil
	}
	s.quitting = true
	s.setStatus(s.tr("Installed; restarting ARAM..."))
}

// removeInstalledArchive deletes a product archive the installer has already
// extracted into its own runtime directory, along with the empty folders the
// download left around it. Keeping it would cost a copy of the product on disk
// for every update, and the next download of the same version would be saved
// beside it rather than replacing it. An archive left by a failed install stays
// put so it can still be installed by hand.
func removeInstalledArchive(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil {
		return
	}
	// Only the repository, channel, and version folders belong to this
	// download. os.Remove refuses a folder that still holds another download.
	directory := filepath.Dir(path)
	for depth := 0; depth < 3; depth++ {
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

func (s *Shell) failProductInstall(err error, firstRun bool) {
	progress := s.updateProgress[updateComponentProduct]
	progress.Busy = false
	progress.Message = shorten(err.Error(), 100)
	s.updateProgress[updateComponentProduct] = progress
	prefix := s.tr("Product update: ")
	if firstRun {
		s.welcomeInstalling = false
		prefix = s.tr("First-run update: ")
	}
	s.appendLog(prefix + err.Error())
	s.setStatus(prefix + err.Error())
}

func (s *Shell) updateProgressSignature() string {
	var parts []string
	for _, component := range []updateComponent{
		updateComponentProduct,
		updateComponentCore,
		updateComponentFrontend,
	} {
		progress := s.updateProgress[component]
		parts = append(parts, fmt.Sprintf(
			"%s:%t:%s:%s:%s",
			component,
			progress.Busy,
			progress.Message,
			progress.Path,
			progress.Version,
		))
	}
	return strings.Join(parts, "|")
}

func (s *Shell) updateRowDescription(
	component updateComponent,
	fallback string,
) string {
	progress := s.updateProgress[component]
	if progress.Message != "" {
		return progress.Message
	}
	return fallback
}

func (s *Shell) updateActionLabel(component updateComponent) string {
	if _, err := updateAssetName(component, runtime.GOOS, runtime.GOARCH); err != nil {
		return "Unavailable"
	}
	if s.updateProgress[component].Busy {
		return "Downloading..."
	}
	if component == updateComponentProduct {
		if _, ok := s.backend.(ProductUpdateInstaller); ok {
			return "Install & Restart"
		}
	}
	return "Download"
}

func (s *Shell) updateActionAvailable(component updateComponent) bool {
	if s.updateProgress[component].Busy {
		return false
	}
	_, err := updateAssetName(component, runtime.GOOS, runtime.GOARCH)
	return err == nil
}
