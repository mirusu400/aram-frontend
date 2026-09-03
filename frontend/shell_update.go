package frontend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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

// startUpdateCheck asks GitHub, in the background, whether a newer build of
// the running channel exists. It runs only for published Stable/Nightly
// builds; a development build has no meaningful version to compare and stays
// silent. The result flows back through updateCheckResults like every other
// async result so the shell state is only ever touched on the UI goroutine.
func (s *Shell) startUpdateCheck() {
	if selfUpdateDisabled() {
		return
	}
	channel, ok := runningReleaseChannel()
	if !ok {
		return
	}
	checker, ok := s.updater.(updateChecker)
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		version, err := checker.CheckLatest(ctx, updateComponentProduct, channel)
		s.updateCheckResults <- updateCheckResult{
			component: updateComponentProduct,
			channel:   channel,
			version:   version,
			err:       err,
		}
	}()
}

// consumeUpdateCheckResult records a background check. A newer build raises
// the menu-bar notice; a failure is logged quietly because an update check
// the user did not ask for should never interrupt them with a status message.
func (s *Shell) consumeUpdateCheckResult(result updateCheckResult) {
	if result.err != nil {
		s.appendLog(s.trf("Update check: %s", result.err.Error()))
		return
	}
	if !updateIsNewer(currentApplicationVersion(), result.version, result.channel) {
		return
	}
	s.updateNoticeReady = true
	s.updateNoticeVersion = result.version
	s.updateNoticeChannel = result.channel
	s.appendLog(s.updateNoticeTooltip())
}

// updateNoticeTooltip is the hover text on the menu-bar update badge.
func (s *Shell) updateNoticeTooltip() string {
	return s.trf(
		"New %s version available: %s",
		s.tr(updateChannelLabel(s.updateNoticeChannel)),
		s.updateNoticeVersion,
	)
}

func (s *Shell) downloadUpdate(component updateComponent) bool {
	if selfUpdateDisabled() {
		s.setStatus(s.tr("Updates are managed by the app store"))
		return false
	}
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
	if errors.Is(err, ErrProductInstallDeferred) {
		s.deferProductInstall(download, firstRun)
		return
	}
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

// deferProductInstall records that the platform installer now owns the
// downloaded package. The package stays on disk because that installer reads
// it after the host returns, and the shell keeps running because the platform
// replaces the app only once the user confirms the installation there.
func (s *Shell) deferProductInstall(download updateDownload, firstRun bool) {
	progress := s.updateProgress[updateComponentProduct]
	progress.Busy = false
	progress.Message = s.trf(
		"Finish installing %s in the system installer",
		download.Version,
	)
	progress.Path = download.Path
	progress.Version = download.Version
	s.updateProgress[updateComponentProduct] = progress
	if firstRun {
		s.welcomeInstalling = false
		s.panel = nil
	}
	s.invalidateSettingsPanel()
	s.appendLog(s.trf(
		"Product update handed to the system installer: %s",
		download.Path,
	))
	s.setStatus(progress.Message)
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
			return productInstallActionLabel()
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
