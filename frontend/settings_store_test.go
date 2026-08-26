package frontend

import "testing"

// TestSettingsPersistRoundTripThroughStore guards the storage seam added for
// the web build: on a filesystem host, values written by save() must load back
// unchanged through the readSettingsBlob/writeSettingsBlob functions. The
// web/wasm build swaps those two functions for localStorage, keeping this same
// load/save logic.
func TestSettingsPersistRoundTripThroughStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)         // Windows UserConfigDir root
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux/macOS UserConfigDir root

	saved := defaultSettings()
	saved.ThemeMode = "dark"
	saved.Language = "ko"
	saved.Speed = 2
	saved.RecentFiles = []string{"probe-marker.dat"}
	if err := saved.save(); err != nil {
		t.Fatal(err)
	}

	loaded := loadSettings()
	if loaded.ThemeMode != "dark" || loaded.Language != "ko" || loaded.Speed != 2 {
		t.Fatalf("round-tripped settings = %+v", loaded)
	}
	if len(loaded.RecentFiles) != 1 || loaded.RecentFiles[0] != "probe-marker.dat" {
		t.Fatalf("round-tripped recent files = %v", loaded.RecentFiles)
	}
}
