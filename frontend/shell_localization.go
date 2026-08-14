package frontend

func (s *Shell) language() Language {
	if s == nil {
		return LanguageEnglish
	}
	return normalizeLanguage(s.settings.Language)
}

func (s *Shell) tr(message string) string {
	return translate(s.language(), message)
}

func (s *Shell) trf(message string, args ...any) string {
	return translatef(s.language(), message, args...)
}

func (s *Shell) trLines(lines []string) []string {
	localized := make([]string, len(lines))
	for index, line := range lines {
		localized[index] = s.tr(line)
	}
	return localized
}
