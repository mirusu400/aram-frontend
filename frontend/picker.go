package frontend

import "errors"

var (
	ErrPickerCanceled    = errors.New("file selection canceled")
	ErrPickerUnavailable = errors.New("native document picker unavailable")
	ErrPickerDeferred    = errors.New("selection delegated to native host")
)

type Picker interface {
	OpenFile() (string, error)
	OpenFontFile() (string, error)
	OpenFirmwareDirectory(previous string) (string, error)
	OpenSaveBackupFile() (string, error)
	ChooseRecent([]string) (string, error)
}

type languageAwarePicker interface {
	SetLanguage(Language)
}

func supportedInputPatterns() []string {
	return append(wipiPackagePatterns(), firmwareImagePatterns()...)
}

func wipiPackagePatterns() []string {
	return []string{"*.dat", "*.jar", "*.zip", "*.ZIP"}
}

func firmwareImagePatterns() []string {
	return []string{"*.wbin", "*.wbt", "*.bin", "*.rom", "*.img", "*.mbn"}
}

func fontFilePatterns() []string {
	return []string{"*.bdf", "*.ttf", "*.otf", "*.ttc"}
}

func saveBackupPatterns() []string {
	return []string{"*.aramsave"}
}
