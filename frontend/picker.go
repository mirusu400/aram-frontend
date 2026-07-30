package frontend

import "errors"

var (
	ErrPickerCanceled    = errors.New("file selection canceled")
	ErrPickerUnavailable = errors.New("native document picker unavailable")
	ErrPickerDeferred    = errors.New("selection delegated to native host")
)

type Picker interface {
	OpenFile() (string, error)
	OpenFirmwareDirectory(previous string) (string, error)
	ChooseRecent([]string) (string, error)
}
