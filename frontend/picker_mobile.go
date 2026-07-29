//go:build android || ios

package frontend

// mobilePicker is deliberately a bridge boundary. Android/iOS native hosts
// select a document and call Shell.OpenExternalDocument. The shared Go layer
// must not import desktop dialog packages or assume a content URI is a path.
type mobilePicker struct{}

func NewPlatformPicker() Picker {
	return mobilePicker{}
}

func (mobilePicker) OpenFile() (string, error) {
	return "", ErrPickerUnavailable
}

func (mobilePicker) OpenFirmwareDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}

func (mobilePicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}
