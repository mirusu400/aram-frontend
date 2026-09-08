package frontend

import "sync"

// Document kinds the shared layer asks a native host to present. The host maps
// each kind to its own picker configuration and answers through the matching
// Shell entry point: OpenExternalDocument for an input or firmware image,
// ImportExternalSaveBackup for a save backup.
const (
	DocumentKindInput      = "input"
	DocumentKindFirmware   = "firmware"
	DocumentKindSaveBackup = "save-backup"
)

// NativePickerHost is implemented by an Android/iOS application layer. It owns
// SAF/UIDocumentPicker presentation and answers asynchronously, so every
// request here returns ErrPickerDeferred rather than a path.
type NativePickerHost interface {
	RequestDocument(kind string)
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

func requestNativeDocument(kind string) error {
	nativePickerBridge.RLock()
	host := nativePickerBridge.host
	nativePickerBridge.RUnlock()
	if host == nil {
		return ErrPickerUnavailable
	}
	host.RequestDocument(kind)
	return ErrPickerDeferred
}

// nativeHostPicker is deliberately a bridge boundary. Android/iOS native hosts
// select a document and call back into the Shell. The shared Go layer must not
// import desktop dialog packages or assume a content URI is a path.
//
// It is compiled on every platform so the kind each menu entry asks for stays
// under test; only NewPlatformPicker is per-platform.
type nativeHostPicker struct{}

func (nativeHostPicker) OpenFile() (string, error) {
	return "", requestNativeDocument(DocumentKindInput)
}

func (nativeHostPicker) OpenFontFile() (string, error) {
	return "", ErrPickerUnavailable
}

// OpenSaveBackupFile routes through the same native picker as an input. A
// backup is an ordinary file the user keeps outside the app, so restoring one
// must not depend on a folder browser the platform does not offer.
func (nativeHostPicker) OpenSaveBackupFile() (string, error) {
	return "", requestNativeDocument(DocumentKindSaveBackup)
}

func (nativeHostPicker) OpenFirmwareDirectory(string) (string, error) {
	return "", requestNativeDocument(DocumentKindFirmware)
}

func (nativeHostPicker) OpenGameDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}

func (nativeHostPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}
