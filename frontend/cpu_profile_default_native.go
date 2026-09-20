//go:build windows || darwin

package frontend

// CPU profiling is enabled by default on native Windows and macOS builds so a
// debug bundle can capture host CPU samples without asking the user to first
// reproduce the problem with a diagnostic switch enabled.
func defaultCPUProfilingEnabled() bool { return true }
