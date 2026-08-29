//go:build windows

package frontend

import (
	"syscall"
	"unsafe"
)

type windowsReportPrompter struct{}

func init() { platformReportPrompter = windowsReportPrompter{} }

const (
	messageBoxYesNo         = 0x00000004
	messageBoxIconError     = 0x00000010
	messageBoxSystemModal   = 0x00001000
	messageBoxSetForeground = 0x00010000
	messageBoxResultYes     = 6
)

// confirmReport shows a native Yes/No dialog. It is a top-level system-modal
// box with no owner window: the crash caller has already lost the ebiten window
// to a panic, and the fault caller runs it off the update goroutine so the
// still-live window keeps painting behind it.
func (windowsReportPrompter) confirmReport(title, message string) (bool, bool) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	caption, err := syscall.UTF16PtrFromString(title)
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
		uintptr(unsafe.Pointer(caption)),
		uintptr(messageBoxYesNo|messageBoxIconError|
			messageBoxSystemModal|messageBoxSetForeground),
	)
	return int(result) == messageBoxResultYes, true
}
