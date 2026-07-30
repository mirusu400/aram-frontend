package frontend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestAddRecentDeduplicatesAndLimits(t *testing.T) {
	settings := defaultSettings()
	for index := 0; index < recentFileLimit+3; index++ {
		settings.addRecent(filepath.Join("games", fmt.Sprintf("%02d.dat", index)))
	}
	if len(settings.RecentFiles) != recentFileLimit {
		t.Fatalf("recent count = %d, want %d", len(settings.RecentFiles), recentFileLimit)
	}
	settings.addRecent(settings.RecentFiles[4])
	if len(settings.RecentFiles) != recentFileLimit {
		t.Fatalf("deduplication changed count to %d", len(settings.RecentFiles))
	}
}

func TestSettingsNormalizePreservesMutedZeroVolume(t *testing.T) {
	settings := defaultSettings()
	settings.Volume = 0
	settings.normalize()
	if settings.Volume != 0 {
		t.Fatalf("Volume = %d, want 0", settings.Volume)
	}
}

func TestSettingsNormalizeRepairsDisplayOptions(t *testing.T) {
	settings := defaultSettings()
	settings.ThemeMode = "blue"
	settings.Rotation = 17
	settings.ScreenLayout = "broken"
	settings.Filter = "broken"
	settings.StateSlot = 99
	settings.Speed = 3
	settings.normalize()
	if settings.Rotation != 0 ||
		settings.ThemeMode != "light" ||
		settings.ScreenLayout != "center" ||
		settings.Filter != "nearest" ||
		settings.StateSlot != 0 ||
		settings.Speed != 1 {
		t.Fatalf("normalized settings = %#v", settings)
	}
}

func TestSettingsNormalizeRepairsControllerOptions(t *testing.T) {
	settings := defaultSettings()
	settings.KeyboardProfile = "broken"
	settings.GamepadLayout = "broken"
	settings.GamepadDeadzone = 5
	settings.normalize()
	if settings.KeyboardProfile != "default" ||
		settings.GamepadLayout != "standard" ||
		settings.GamepadDeadzone != 30 {
		t.Fatalf("normalized controller settings = %#v", settings)
	}
	if !settings.GamepadEnabled || !settings.GamepadAnalog {
		t.Fatalf("default controller switches were changed = %#v", settings)
	}
}

func TestLegacySettingsReceiveControllerDefaults(t *testing.T) {
	settings := defaultSettings()
	if err := json.Unmarshal([]byte(`{"keyboard_profile":"wasd"}`), &settings); err != nil {
		t.Fatal(err)
	}
	settings.normalize()

	if settings.KeyboardProfile != "wasd" ||
		!settings.GamepadEnabled ||
		settings.GamepadLayout != "standard" ||
		!settings.GamepadAnalog ||
		settings.GamepadDeadzone != 30 {
		t.Fatalf("legacy controller settings = %#v", settings)
	}
}

func TestPerTitleControllerProfileDoesNotChangeGlobalDefaults(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := &Shell{
		settings: defaultSettings(),
		input:    &InputInfo{DisplayName: "example.dat", SHA256: "abc123"},
	}
	shell.settings.PerTitleControls = true

	option, ok := gamepadButtonOptionByID("face-east")
	if !ok {
		t.Fatal("face-east gamepad option is unavailable")
	}
	shell.applyGamepadBinding("ok", option)

	title := shell.settings.TitleControllers["sha256:abc123"]
	if title.GamepadBindings["ok"] != "face-east" {
		t.Fatalf("title confirm binding = %#v", title.GamepadBindings)
	}
	if shell.settings.GamepadBindings["ok"] != "" {
		t.Fatalf("global binding was modified = %#v", shell.settings.GamepadBindings)
	}
	if shell.controllerProfile().GamepadLayout != "custom" {
		t.Fatalf("effective title profile = %#v", shell.controllerProfile())
	}

	shell.resetControllerBindings()
	if _, ok := shell.settings.TitleControllers["sha256:abc123"]; ok {
		t.Fatal("title override remained after reset")
	}
}

func TestShortenPreservesKoreanRunes(t *testing.T) {
	if got := shorten("마법홀게임실행", 5); got != "마법..." {
		t.Fatalf("shorten Korean = %q", got)
	}
}
