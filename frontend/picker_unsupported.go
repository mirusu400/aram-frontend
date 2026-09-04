//go:build !windows && !linux && !darwin && !android && !ios && !js

package frontend

type unsupportedPicker struct{}

func NewPlatformPicker() Picker { return unsupportedPicker{} }
func (unsupportedPicker) OpenFile() (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) OpenFontFile() (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) OpenSaveBackupFile() (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) OpenFirmwareDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) OpenGameDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}
