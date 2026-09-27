package frontend

import (
	"strings"
	"testing"
)

// This isolates shared UI bookkeeping with a synthetic backend. It is not a
// product playback, GPU draw, browser, or mobile performance measurement.
func BenchmarkShellUISteadyState(b *testing.B) {
	for _, mode := range []string{"sync", "sync-update", "file-menu-sync-update"} {
		b.Run(mode, func(b *testing.B) {
			temporary := b.TempDir()
			b.Setenv("APPDATA", temporary)
			b.Setenv("XDG_CONFIG_HOME", temporary)
			shell := NewShell(&videoBackend{}, nil, "")
			// Keep the native sampler out of this isolated timing. An externally
			// requested go test CPU profile is unaffected when the shell could
			// not acquire its own profiler.
			shell.setCPUProfiling(false)
			shell.input = &InputInfo{
				DisplayName: "synthetic.dat",
				SHA256:      strings.Repeat("0", 64),
			}
			shell.panel = nil
			shell.settings.Language = string(LanguageEnglish)
			shell.settings.Speed = 1
			shell.measuredSpeed = 1
			shell.Layout(893, 749)
			if mode == "file-menu-sync-update" {
				shell.activeMenu = 0
			}
			view := shell.interfaceUI
			view.sync(shell)
			view.ui.Update()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view.sync(shell)
				if mode != "sync" {
					view.ui.Update()
				}
			}
			b.StopTimer()
			if shell.input.DisplayName != "synthetic.dat" || shell.settings.Speed != 1 {
				b.Fatal("UI bookkeeping changed guest input identity or speed")
			}
		})
	}
}
