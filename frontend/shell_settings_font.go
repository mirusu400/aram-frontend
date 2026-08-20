package frontend

import (
	"errors"
	"os"
	"path/filepath"
)

// handsetFontChoices lists the selectable built-in fallback fonts in dropdown
// order. The values are the identifiers the backend and core understand.
var handsetFontChoices = []string{"galmuri9", "mulmaru", "neodgm"}

// fontDropdownChoices returns the font identifiers offered in the dropdown,
// appending the loaded custom font when one is configured.
func (s *Shell) fontDropdownChoices() []string {
	choices := append([]string(nil), handsetFontChoices...)
	if s.settings.CustomFontPath != "" {
		choices = append(choices, "custom")
	}
	return choices
}

func fontChoiceBuiltinLabel(name string) string {
	switch name {
	case "neodgm":
		return "Dunggeunmo (soft)"
	case "mulmaru":
		return "Mulmaru (bold)"
	default:
		return "Galmuri (crisp)"
	}
}

// fontChoiceLabel returns the localized display label for a font identifier; the
// custom entry shows the chosen file's name.
func (s *Shell) fontChoiceLabel(name string) string {
	if name == "custom" {
		base := filepath.Base(s.settings.CustomFontPath)
		if base == "." || base == "" || base == string(filepath.Separator) {
			base = s.tr("custom font")
		}
		return s.trf("Custom: %s", base)
	}
	return s.tr(fontChoiceBuiltinLabel(name))
}

func fontChoiceIndex(choices []string, name string) int {
	for i, choice := range choices {
		if choice == name {
			return i
		}
	}
	return 0
}

func (s *Shell) currentFontSettings() FontSettings {
	settings := FontSettings{Name: s.settings.FontChoice}
	if s.settings.FontChoice == "custom" {
		settings.Data = s.customFontData
	}
	return settings
}

// setFont selects a font by identifier and applies it.
func (s *Shell) setFont(name string) {
	s.settings.FontChoice = name
	s.applyFontSettings()
}

// loadCustomFont prompts for a BDF or TrueType/OpenType font file, reads it, and
// switches the handset fallback to it.
func (s *Shell) loadCustomFont() {
	path, err := s.picker.OpenFontFile()
	if err != nil {
		if !errors.Is(err, ErrPickerCanceled) {
			s.setStatus(s.tr("Handset font: ") + err.Error())
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.setStatus(s.tr("Handset font: ") + err.Error())
		return
	}
	s.customFontData = data
	s.settings.CustomFontPath = path
	s.settings.FontChoice = "custom"
	s.applyFontSettings()
}

// loadCustomFontAtStartup re-reads a previously chosen custom font so it can be
// pushed to the backend when the shell starts. A missing file falls back to the
// crisp default.
func (s *Shell) loadCustomFontAtStartup() {
	if s.settings.FontChoice != "custom" || s.settings.CustomFontPath == "" {
		return
	}
	data, err := os.ReadFile(s.settings.CustomFontPath)
	if err != nil {
		s.settings.FontChoice = "galmuri9"
		s.appendLog(s.tr("Handset font: ") + err.Error())
		return
	}
	s.customFontData = data
}

// applyFontSettings persists the selection and pushes it to the backend. The
// new font takes effect the next time a title is opened.
func (s *Shell) applyFontSettings() {
	_ = s.settings.save()
	if backend, ok := s.backend.(FontBackend); ok {
		if err := backend.ConfigureFont(s.currentFontSettings()); err != nil {
			s.setStatus(s.tr("Handset font: ") + err.Error())
			return
		}
	}
	s.setStatus(s.trf(
		"Handset font: %s (restart title to apply)",
		s.fontChoiceLabel(s.settings.FontChoice),
	))
}
