package frontend

import "fmt"

func isPreviewBackend(backend Backend) bool {
	switch backend.(type) {
	case NullBackend, *NullBackend:
		return true
	default:
		return false
	}
}

func (s *Shell) shouldOpenWelcome() bool {
	return !s.settings.WelcomeCompleted && !isPreviewBackend(s.backend)
}

func (s *Shell) openWelcome() {
	s.panel = &Panel{
		Kind:  "welcome",
		Title: "Welcome to ARAM",
	}
}

func (s *Shell) completeWelcome(channel updateChannel) {
	if s.welcomeInstalling {
		return
	}
	channel = normalizeUpdateChannel(string(channel))
	previousChannel := s.settings.UpdateChannel
	previousCompleted := s.settings.WelcomeCompleted
	s.settings.UpdateChannel = string(channel)

	_, canInstall := s.backend.(ProductUpdateInstaller)
	if canInstall {
		s.settings.WelcomeCompleted = false
		if err := s.settings.save(); err != nil {
			s.settings.UpdateChannel = previousChannel
			s.settings.WelcomeCompleted = previousCompleted
			s.setStatus(s.tr("Welcome settings: ") + err.Error())
			return
		}
		s.welcomeInstalling = true
		if !s.downloadUpdate(updateComponentProduct) {
			s.welcomeInstalling = false
			return
		}
		s.setStatus(s.trf(
			"Downloading the latest integrated %s build...",
			s.tr(updateChannelLabel(channel)),
		))
		return
	}

	s.settings.WelcomeCompleted = true
	if err := s.settings.save(); err != nil {
		s.settings.UpdateChannel = previousChannel
		s.settings.WelcomeCompleted = previousCompleted
		s.setStatus(s.tr("Welcome settings: ") + err.Error())
		return
	}
	s.panel = nil
	s.setStatus(s.trf(
		"Setup complete - %s integrated product updates selected",
		s.tr(updateChannelLabel(channel)),
	))
}

func (s *Shell) completeWelcomeWithBundledStable() {
	s.settings.WelcomeCompleted = true
	if err := s.settings.save(); err != nil {
		s.settings.WelcomeCompleted = false
		s.failProductInstall(
			fmt.Errorf("save Welcome settings: %w", err),
			true,
		)
		return
	}
	s.welcomeInstalling = false
	s.panel = nil
	s.setStatus(s.tr(
		"No Stable release is published yet; continuing with the bundled build",
	))
	s.DispatchExternalCommand("file.open")
}

func (s *Shell) dismissWelcome() {
	if s.welcomeInstalling {
		return
	}
	s.panel = nil
	s.setStatus(s.tr("Welcome deferred - update channel setup will return next launch"))
}
