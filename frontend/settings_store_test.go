package frontend

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// TestWriteSettingsBlobLeavesNoTempFileBehind guards the atomic write/rename
// in writeFileAtomically: settings.save runs on nearly every setting change,
// so a stray *.tmp file left behind on every successful write would otherwise
// accumulate in the config directory for as long as the app is used.
func TestWriteSettingsBlobLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	settings := defaultSettings()
	settings.RecentFiles = []RecentEntry{{Path: "probe-marker.dat"}}
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}

	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("settings directory = %v, want only %q", entries, filepath.Base(path))
	}
}

// TestWriteFileAtomicallyPreservesExistingFileOnFailure covers the crash
// window writeFileAtomically closes: a process torn down mid-write (a crash,
// a forced quit, a power loss) must never leave settings.json truncated,
// since loadSettings treats a file that fails to parse as no settings file at
// all and silently resets everything - the recent titles list included -
// back to defaults. A write that cannot even create its temp file (simulated
// here with a missing directory) must leave whatever was already on disk
// untouched rather than losing it.
func TestWriteFileAtomicallyPreservesExistingFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"original":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	missingDir := filepath.Join(dir, "does-not-exist")
	if err := writeFileAtomically(missingDir, path, []byte(`{"new":true}`)); err == nil {
		t.Fatal("write with a missing temp directory unexpectedly succeeded")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"original":true}` {
		t.Fatalf("existing file = %q, want it untouched by the failed write", data)
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
