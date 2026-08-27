package frontend

func (s *Shell) toggleFullscreen() {
	s.setStatus(s.tr(togglePlatformFullscreen()))
}

func (s *Shell) toggleIntegerScaling() {
	s.settings.IntegerScaling = !s.settings.IntegerScaling
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Integer scaling: %s",
		s.tr(onOff(s.settings.IntegerScaling)),
	))
}

func (s *Shell) toggleAspectRatio() {
	s.settings.PreserveAspect = !s.settings.PreserveAspect
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Preserve aspect ratio: %s",
		s.tr(onOff(s.settings.PreserveAspect)),
	))
}

func (s *Shell) fitWindow() {
	s.setStatus(s.tr(fitPlatformWindow()))
}

func (s *Shell) cycleRotation() {
	s.settings.Rotation = (s.settings.Rotation + 90) % 360
	_ = s.settings.save()
	s.setStatus(s.trf("Rotation: %d°", s.settings.Rotation))
}

func (s *Shell) cycleScreenLayout() {
	if s.settings.ScreenLayout == "center" {
		s.settings.ScreenLayout = "stretch"
	} else {
		s.settings.ScreenLayout = "center"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Screen layout: %s",
		s.tr(settingValueLabel(s.settings.ScreenLayout)),
	))
}

func (s *Shell) cycleFilter() {
	if s.settings.Filter == "nearest" {
		s.settings.Filter = "linear"
	} else {
		s.settings.Filter = "nearest"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Filter: %s",
		s.tr(settingValueLabel(s.settings.Filter)),
	))
}

func (s *Shell) cycleStateSlot() {
	s.setStateSlot((s.settings.StateSlot + 1) % 10)
}

func (s *Shell) setStateSlot(slot int) {
	if slot < 0 {
		slot = 0
	} else if slot > 9 {
		slot = 9
	}
	s.settings.StateSlot = slot
	_ = s.settings.save()
	s.setStatus(s.trf("State slot: %d", s.settings.StateSlot))
}

func (s *Shell) cycleSpeed() {
	s.setSpeed(speedPresets[(speedPresetIndex(s.settings.Speed)+1)%len(speedPresets)])
}

func (s *Shell) setSpeed(speed float64) {
	if speed != s.settings.Speed {
		s.flushAudioDiscontinuity()
		s.resetFramePacing()
	}
	s.settings.Speed = speed
	_ = s.settings.save()
	s.setStatus(s.trf("Emulation speed: %gx", s.settings.Speed))
}

func (s *Shell) showAbout() {
	s.panel = &Panel{
		Kind:  "about",
		Title: "About ARAM",
		Lines: []string{
			"ARAM - Archived Runtime for ARM Mobiles",
			"",
			"Cross-platform frontend for Korean feature-phone emulation.",
			s.trf("Version: %s", currentApplicationVersion()),
			s.trf(
				"Frontend state: %s",
				s.tr(stateValueLabel(string(s.state))),
			),
			s.trf("Backend: %s", s.backendName()),
		},
	}
}

func (s *Shell) openDocumentation() {
	if err := openPlatformURL("https://github.com/mirusu400/aram-emu/tree/main/docs"); err != nil {
		s.setStatus(s.tr("Documentation: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Opened ARAM documentation"))
}
