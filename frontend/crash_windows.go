//go:build windows

package frontend

import (
	"syscall"
	"unsafe"
)

type windowsCrashPrompter struct{}

func init() { platformCrashPrompter = windowsCrashPrompter{} }

const (
	messageBoxYesNo         = 0x00000004
	messageBoxIconError     = 0x00000010
	messageBoxSystemModal   = 0x00001000
	messageBoxSetForeground = 0x00010000
	messageBoxResultYes     = 6
)

// confirmCrashReport shows a native Yes/No dialog. The ebiten window is already
// gone after a main-loop panic, so this is a top-level system-modal box.
func (windowsCrashPrompter) confirmCrashReport(message string) (bool, bool) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	title, err := syscall.UTF16PtrFromString("ARAM crashed")
	if err != nil {
		return false, false
	}
	body, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return false, false
	}
	result, _, _ := messageBox.Call(
		0,
		uintptr(unsafe.Pointer(body)),
		uintptr(unsafe.Pointer(title)),
		uintptr(messageBoxYesNo|messageBoxIconError|
			messageBoxSystemModal|messageBoxSetForeground),
	)
	return int(result) == messageBoxResultYes, true
}
