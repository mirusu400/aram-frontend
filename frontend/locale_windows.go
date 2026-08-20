//go:build windows

package frontend

import (
	"syscall"
	"unsafe"
)

func osLocale() string {
	const localeNameMaxLength = 85
	buffer := make([]uint16, localeNameMaxLength)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getUserDefaultLocaleName := kernel32.NewProc("GetUserDefaultLocaleName")
	length, _, _ := getUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer)
}
