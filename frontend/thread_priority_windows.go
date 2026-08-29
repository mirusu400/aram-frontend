//go:build windows

package frontend

// kernel32 is declared in battery_windows.go and shared here.
var (
	procGetCurrentThread  = kernel32.NewProc("GetCurrentThread")
	procSetThreadPriority = kernel32.NewProc("SetThreadPriority")
)

// lowerCurrentThreadPriority drops the calling OS thread below the interface
// thread so a heavy guest cannot starve the UI of CPU. It is best-effort: a
// failed call simply leaves the thread at its normal priority.
func lowerCurrentThreadPriority() {
	handle, _, _ := procGetCurrentThread.Call()
	const belowNormal = -1 // THREAD_PRIORITY_BELOW_NORMAL
	priority := belowNormal
	_, _, _ = procSetThreadPriority.Call(handle, uintptr(priority))
}
