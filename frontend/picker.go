package frontend

import (
	"errors"
	"slices"
)

var (
	ErrPickerCanceled    = errors.New("file selection canceled")
	ErrPickerUnavailable = errors.New("native document picker unavailable")
	ErrPickerDeferred    = errors.New("selection delegated to native host")
)

type Picker interface {
	OpenFile() (string, error)
	OpenFontFile() (string, error)
	OpenFirmwareDirectory(previous string) (string, error)
	OpenGameDirectory(previous string) (string, error)
	OpenSaveBackupFile() (string, error)
	ChooseRecent([]string) (string, error)
}

type languageAwarePicker interface {
	SetLanguage(Language)
}

func supportedInputPatterns() []string {
	patterns := wipiPackagePatterns()
	// ZIP and JAR are shared containers, not evidence of a platform. The core
	// inspects the selected bytes and chooses the application implementation.
	for _, pattern := range append(j2mePackagePatterns(), gvmInputPatterns()...) {
		if !slices.Contains(patterns, pattern) {
			patterns = append(patterns, pattern)
		}
	}
	return append(patterns, firmwareImagePatterns()...)
}

func wipiPackagePatterns() []string {
	return []string{"*.dat", "*.jar", "*.zip", "*.ZIP"}
}

func j2mePackagePatterns() []string {
	// Standalone JAD files are not supported: companion document access must
	// be explicit, and descriptor URLs must never cause automatic downloads.
	return []string{"*.jar", "*.JAR", "*.zip", "*.ZIP"}
}

func gvmInputPatterns() []string {
	// These inputs have a validated recognition path, not an execution backend.
	return []string{"*.sgs", "*.SGS", "*.zip", "*.ZIP"}
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
