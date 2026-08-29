//go:build !windows

package frontend

// lowerCurrentThreadPriority is a no-op on platforms without a simple
// per-thread priority knob; the guest worker runs at normal priority there.
// The Windows build lowers it so the guest yields CPU to the interface.
func lowerCurrentThreadPriority() {}
