package frontend

func (s *Shell) cycleThemeMode() {
	if s.settings.ThemeMode == "light" {
		s.settings.ThemeMode = "dark"
	} else {
		s.settings.ThemeMode = "light"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Appearance: %s",
		s.tr(settingValueLabel(s.settings.ThemeMode)),
	))
}

// setThemeFamily switches the shell skin. The design system itself is rebuilt
// by syncDesignSystem on the next update once the setting no longer matches.
func (s *Shell) setThemeFamily(family string) {
	if family == s.settings.ThemeFamily {
		return
	}
	if family != themeFamilyModern && !isRetroFamily(family) {
		return
	}
	previous := s.settings.ThemeFamily
	s.settings.ThemeFamily = family
	if err := s.settings.save(); err != nil {
		s.settings.ThemeFamily = previous
		s.setStatus(s.tr("Appearance: ") + err.Error())
		return
	}
	s.setStatus(s.trf("UI skin: %s", s.tr(themeFamilyLabel(family))))
}

func (s *Shell) cycleLanguage() {
	previous := s.settings.Language
	if normalizeLanguage(previous) == LanguageKorean {
		s.settings.Language = string(LanguageEnglish)
	} else {
		s.settings.Language = string(LanguageKorean)
	}
	if err := s.settings.save(); err != nil {
		s.settings.Language = previous
		s.setStatus(s.tr("Language settings: ") + err.Error())
		return
	}
	s.setStatus(s.trf(
		"Language: %s",
		languageLabel(s.language(), s.language()),
	))
	if s.panel != nil {
		s.panel.Title = s.tr("Configure ARAM")
	}
	if localized, ok := s.picker.(languageAwarePicker); ok {
		localized.SetLanguage(s.language())
	}
	if s.input == nil {
		setPlatformWindowTitle(s.tr("ARAM - Archived Runtime for ARM Mobiles"))
	}
	s.interfaceUI = newShellUI(s, s.design)
}
