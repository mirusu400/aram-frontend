package frontend

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
	channel = normalizeUpdateChannel(string(channel))
	previousChannel := s.settings.UpdateChannel
	s.settings.UpdateChannel = string(channel)
	s.settings.WelcomeCompleted = true
	if err := s.settings.save(); err != nil {
		s.settings.UpdateChannel = previousChannel
		s.settings.WelcomeCompleted = false
		s.setStatus(s.tr("Welcome settings: ") + err.Error())
		return
	}
	s.panel = nil
	s.setStatus(s.trf(
		"Setup complete - %s integrated product updates selected",
		s.tr(updateChannelLabel(channel)),
	))
}

func (s *Shell) dismissWelcome() {
	s.panel = nil
	s.setStatus(s.tr("Welcome deferred - update channel setup will return next launch"))
}
