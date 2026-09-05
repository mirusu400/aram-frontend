package frontend

import (
	"encoding/json"
	"testing"
)

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
	saved.RecentFiles = []RecentEntry{{Path: "probe-marker.dat", Name: "Probe Marker"}}
	if err := saved.save(); err != nil {
		t.Fatal(err)
	}

	loaded := loadSettings()
	if loaded.ThemeMode != "dark" || loaded.Language != "ko" || loaded.Speed != 2 {
		t.Fatalf("round-tripped settings = %+v", loaded)
	}
	if len(loaded.RecentFiles) != 1 || loaded.RecentFiles[0] != saved.RecentFiles[0] {
		t.Fatalf("round-tripped recent files = %v", loaded.RecentFiles)
	}
}

// TestRecentFilesUnmarshalsLegacyPlainPathArray guards settings.json written
// before RecentEntry existed ("recent_files": ["a.dat", "b.dat"]) still
// loading without error, each string becoming a path-only entry.
func TestRecentFilesUnmarshalsLegacyPlainPathArray(t *testing.T) {
	var settings Settings
	blob := []byte(`{"recent_files": ["games/a.dat", "games/b.dat"]}`)
	if err := json.Unmarshal(blob, &settings); err != nil {
		t.Fatalf("unmarshal legacy recent_files: %v", err)
	}
	want := []RecentEntry{{Path: "games/a.dat"}, {Path: "games/b.dat"}}
	if len(settings.RecentFiles) != len(want) {
		t.Fatalf("recent files = %#v", settings.RecentFiles)
	}
	for index, entry := range settings.RecentFiles {
		if entry != want[index] {
			t.Fatalf("recent file[%d] = %#v, want %#v", index, entry, want[index])
		}
	}
}
