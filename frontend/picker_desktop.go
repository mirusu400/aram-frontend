//go:build (windows || linux || darwin) && !android && !ios

package frontend

import (
	"errors"

	"github.com/ncruces/zenity"
)

type platformPicker struct {
	language Language
}

func NewPlatformPicker() Picker {
	return &platformPicker{language: systemLanguage()}
}

func (p *platformPicker) SetLanguage(language Language) {
	p.language = normalizeLanguage(string(language))
}

func (p *platformPicker) OpenFile() (string, error) {
	path, err := zenity.SelectFile(
		zenity.Title(translate(p.language, "Open WIPI package or firmware")),
		zenity.FileFilters{
			{Name: translate(p.language, "Supported inputs"), Patterns: supportedInputPatterns()},
			{Name: translate(p.language, "WIPI packages"), Patterns: wipiPackagePatterns()},
			{Name: translate(p.language, "Firmware images"), Patterns: firmwareImagePatterns()},
			{Name: translate(p.language, "All files"), Patterns: []string{"*"}},
		},
	)
	return path, normalizePickerError(err)
}

func (p *platformPicker) OpenFontFile() (string, error) {
	path, err := zenity.SelectFile(
		zenity.Title(translate(p.language, "Choose a handset font")),
		zenity.FileFilters{
			{Name: translate(p.language, "Bitmap and outline fonts"), Patterns: fontFilePatterns()},
			{Name: translate(p.language, "All files"), Patterns: []string{"*"}},
		},
	)
	return path, normalizePickerError(err)
}

func (p *platformPicker) OpenSaveBackupFile() (string, error) {
	path, err := zenity.SelectFile(
		zenity.Title(translate(p.language, "Choose a save backup to restore")),
		zenity.FileFilters{
			{Name: translate(p.language, "ARAM save backups"), Patterns: saveBackupPatterns()},
			{Name: translate(p.language, "All files"), Patterns: []string{"*"}},
		},
	)
	return path, normalizePickerError(err)
}

func (p *platformPicker) OpenFirmwareDirectory(previous string) (string, error) {
	options := []zenity.Option{
		zenity.Title(translate(p.language, "Select firmware directory")),
		zenity.Directory(),
	}
	if previous != "" {
		options = append(options, zenity.Filename(previous))
	}
	path, err := zenity.SelectFile(options...)
	return path, normalizePickerError(err)
}

func (p *platformPicker) OpenGameDirectory(previous string) (string, error) {
	options := []zenity.Option{
		zenity.Title(translate(p.language, "Select game library folder")),
		zenity.Directory(),
	}
	if previous != "" {
		options = append(options, zenity.Filename(previous))
	}
	path, err := zenity.SelectFile(options...)
	return path, normalizePickerError(err)
}

func (p *platformPicker) ChooseRecent(recent []string) (string, error) {
	labels := make([]string, len(recent))
	for index, path := range recent {
		labels[index] = recentEntryLabel("", path, 110)
	}
	selected, err := zenity.List(
		translate(p.language, "Choose a recent input"),
		labels,
		zenity.Title(translate(p.language, "ARAM recent files")),
		zenity.Width(840),
		zenity.Height(420),
	)
	if err != nil {
		return "", normalizePickerError(err)
	}
	for index, label := range labels {
		if label == selected {
			return recent[index], nil
		}
	}
	return "", ErrPickerCanceled
}

func normalizePickerError(err error) error {
	if errors.Is(err, zenity.ErrCanceled) {
		return ErrPickerCanceled
	}
	return err
}
