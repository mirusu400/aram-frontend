//go:build (windows || linux || darwin) && !android && !ios

package frontend

import (
	"errors"

	"github.com/ncruces/zenity"
)

type platformPicker struct{}

func NewPlatformPicker() Picker {
	return platformPicker{}
}

func (platformPicker) OpenFile() (string, error) {
	path, err := zenity.SelectFile(
		zenity.Title("Open WIPI package or firmware"),
		zenity.FileFilters{
			{Name: "Supported inputs", Patterns: []string{"*.dat", "*.wbin", "*.wbt", "*.bin", "*.rom", "*.img", "*.mbn", "*.jar"}},
			{Name: "WIPI packages", Patterns: []string{"*.dat", "*.jar"}},
			{Name: "Firmware images", Patterns: []string{"*.wbin", "*.wbt", "*.bin", "*.rom", "*.img", "*.mbn"}},
			{Name: "All files", Patterns: []string{"*"}},
		},
	)
	return path, normalizePickerError(err)
}

func (platformPicker) OpenFirmwareDirectory(previous string) (string, error) {
	options := []zenity.Option{
		zenity.Title("Select firmware directory"),
		zenity.Directory(),
	}
	if previous != "" {
		options = append(options, zenity.Filename(previous))
	}
	path, err := zenity.SelectFile(options...)
	return path, normalizePickerError(err)
}

func (platformPicker) ChooseRecent(recent []string) (string, error) {
	labels := make([]string, len(recent))
	for index, path := range recent {
		labels[index] = recentEntryLabel(path, 110)
	}
	selected, err := zenity.List(
		"Choose a recent input",
		labels,
		zenity.Title("ARAM recent files"),
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
