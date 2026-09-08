//go:build android || ios

package frontend

func NewPlatformPicker() Picker {
	return nativeHostPicker{}
}
