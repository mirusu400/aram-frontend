package frontend

func (s *Shell) toggleFullscreen() {
	s.setStatus(s.tr(togglePlatformFullscreen()))
}

func (s *Shell) toggleIntegerScaling() {
	profile := s.displayProfile()
	profile.IntegerScaling = !profile.IntegerScaling
	s.saveDisplayProfile(profile, s.trf(
		"Integer scaling: %s",
		s.tr(onOff(profile.IntegerScaling)),
	))
}

// toggleUIPriority flips the persisted UI-priority preference. The frame worker
// reads it per batch, so the change takes effect on the next scheduled frame.
func (s *Shell) toggleUIPriority() {
	s.settings.UIPriority = !s.settings.UIPriority
	_ = s.settings.save()
	s.setStatus(s.trf("UI priority: %s", s.tr(onOff(s.settings.UIPriority))))
}

func (s *Shell) toggleAspectRatio() {
	profile := s.displayProfile()
	profile.PreserveAspect = !profile.PreserveAspect
	s.saveDisplayProfile(profile, s.trf(
		"Preserve aspect ratio: %s",
		s.tr(onOff(profile.PreserveAspect)),
	))
}

func (s *Shell) fitWindow() {
	s.setStatus(s.tr(fitPlatformWindow()))
}

func (s *Shell) cycleRotation() {
	profile := s.displayProfile()
	profile.Rotation = (profile.Rotation + 90) % 360
	s.saveDisplayProfile(profile, s.trf("Rotation: %d°", profile.Rotation))
}

func (s *Shell) cycleScreenLayout() {
	profile := s.displayProfile()
	if profile.ScreenLayout == "center" {
		profile.ScreenLayout = "stretch"
	} else {
		profile.ScreenLayout = "center"
	}
	s.saveDisplayProfile(profile, s.trf(
		"Screen layout: %s",
		s.tr(settingValueLabel(profile.ScreenLayout)),
	))
}

func (s *Shell) cycleFilter() {
	profile := s.displayProfile()
	if profile.Filter == "nearest" {
		profile.Filter = "linear"
	} else {
		profile.Filter = "nearest"
	}
	s.saveDisplayProfile(profile, s.trf(
		"Filter: %s",
		s.tr(settingValueLabel(profile.Filter)),
	))
}

func (s *Shell) cycleDisplayEffect() {
	profile := s.displayProfile()
	choices := displayEffectChoices()
	current := 0
	for index, effect := range choices {
		if effect == profile.DisplayEffect {
			current = index
			break
		}
	}
	profile.DisplayEffect = choices[(current+1)%len(choices)]
	s.saveDisplayProfile(profile, s.trf(
		"Display Preset: %s",
		s.tr(displayEffectValueLabel(profile.DisplayEffect)),
	))
}

func (s *Shell) setDisplayEffectStrength(strength int) {
	profile := s.displayProfile()
	profile.DisplayEffectStrength = clampInt(
		strength,
		displayEffectStrengthMin,
		displayEffectStrengthMax,
	)
	s.saveDisplayProfile(profile, s.trf(
		"Filter strength: %d%%",
		profile.DisplayEffectStrength,
	))
}

func (s *Shell) displayProfile() DisplayProfile {
	global := s.settings.globalDisplayProfile()
	key := titleSettingsKey(s.input)
	if key == "" {
		return global
	}
	profile, ok := s.settings.TitleDisplays[key]
	if !ok {
		return global
	}
	profile.normalize()
	return profile
}

func (s *Shell) saveDisplayProfile(profile DisplayProfile, status string) {
	profile.normalize()
	if key := titleSettingsKey(s.input); key != "" {
		if s.settings.TitleDisplays == nil {
			s.settings.TitleDisplays = make(map[string]DisplayProfile)
		}
		s.settings.TitleDisplays[key] = profile
	} else {
		s.settings.setGlobalDisplayProfile(profile)
	}
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Display settings: ") + err.Error())
		return
	}
	s.setStatus(status)
}

func (s *Shell) displayProfileScopeLabel() string {
	if titleSettingsKey(s.input) != "" {
		return "This title"
	}
	return "Global"
}

func displayEffectSupportsStrength(effect string) bool {
	switch effect {
	case displayEffectFeaturePhoneTFT,
		displayEffectFeaturePhoneSTN,
		displayEffectSmoothPixel,
		displayEffectCRTTV:
		return true
	default:
		return false
	}
}

func displayEffectValueLabel(effect string) string {
	switch effect {
	case displayEffectCrispFit:
		return "Crisp Fit"
	case displayEffectFeaturePhoneTFT:
		return "Feature Phone TFT"
	case displayEffectFeaturePhoneSTN:
		return "Feature Phone STN"
	case displayEffectSmoothPixel:
		return "Smooth Pixel"
	case displayEffectCRTTV:
		return "CRT TV"
	default:
		return "Original"
	}
}

func (s *Shell) displayPresentationValueLabel() string {
	profile := s.displayProfile()
	if profile.DisplayEffect == displayEffectOff {
		return settingValueLabel(profile.Filter)
	}
	return displayEffectValueLabel(profile.DisplayEffect)
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
