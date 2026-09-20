//go:build !windows && !darwin

package frontend

// Android and WebAssembly default off because continuous profiling competes
// directly with emulation on their constrained execution paths. Linux and any
// future unclassified hosts stay opt-in until chosen explicitly.
func defaultCPUProfilingEnabled() bool { return false }
