//go:build android || ios

package frontend

import "sync"

// mobilePicker is deliberately a bridge boundary. Android/iOS native hosts
// select a document and call Shell.OpenExternalDocument. The shared Go layer
// must not import desktop dialog packages or assume a content URI is a path.
type mobilePicker struct{}

type NativePickerHost interface {
	RequestDocument(firmware bool)
}

var nativePickerBridge struct {
	sync.RWMutex
	host NativePickerHost
}

func SetNativePickerHost(host NativePickerHost) {
	nativePickerBridge.Lock()
	nativePickerBridge.host = host
	nativePickerBridge.Unlock()
}

func requestNativeDocument(firmware bool) error {
	nativePickerBridge.RLock()
	host := nativePickerBridge.host
	nativePickerBridge.RUnlock()
	if host == nil {
		return ErrPickerUnavailable
	}
	host.RequestDocument(firmware)
	return ErrPickerDeferred
}

func NewPlatformPicker() Picker {
	return mobilePicker{}
}

func (mobilePicker) OpenFile() (string, error) {
	return "", requestNativeDocument(false)
}

func (mobilePicker) OpenFirmwareDirectory(string) (string, error) {
	return "", requestNativeDocument(true)
}

func (mobilePicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}
