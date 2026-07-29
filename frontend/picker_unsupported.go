//go:build !windows && !linux && !darwin && !android && !ios

package frontend

type unsupportedPicker struct{}

func NewPlatformPicker() Picker { return unsupportedPicker{} }
func (unsupportedPicker) OpenFile() (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) OpenFirmwareDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}
func (unsupportedPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}
