package frontend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	settings.Language = "fr"
	settings.ThemeMode = "blue"
	settings.Rotation = 17
	settings.ScreenLayout = "broken"
	settings.Filter = "broken"
	settings.StateSlot = 99
	settings.Speed = 3
	settings.normalize()
	if settings.Rotation != 0 ||
		settings.Language != string(LanguageEnglish) ||
		settings.ThemeMode != "light" ||
		settings.ScreenLayout != "center" ||
		settings.Filter != "nearest" ||
		settings.StateSlot != 0 ||
		settings.Speed != 1 {
		t.Fatalf("normalized settings = %#v", settings)
	}
}

func TestDefaultSettingsRequireIntegratedWelcome(t *testing.T) {
	settings := defaultSettings()
	if settings.WelcomeCompleted {
		t.Fatal("fresh settings unexpectedly skip the integrated Welcome")
	}
	if settings.UpdateChannel != string(updateChannelStable) {
		t.Fatalf("fresh update channel = %q", settings.UpdateChannel)
	}
}

func TestIssueReportHistoryDeduplicatesValidEntriesAndLimitsSize(t *testing.T) {
	settings := defaultSettings()
	for index := 0; index < issueReportHistoryLimit+3; index++ {
		settings.rememberIssueReport(IssueReportRecord{
			ReportID: fmt.Sprintf(
				"%08x-1111-4111-8111-111111111111",
				index,
			),
			IssueURL: fmt.Sprintf(
				"https://github.com/mirusu400/aram-frontend/issues/%d",
				index+1,
			),
			Capability: "aram_rpt_" + strings.Repeat("A", 43),
			Repository: "aram-frontend",
			Situation:  fmt.Sprintf("Report %d", index),
			CreatedAt:  time.Unix(int64(index+1), 0).UTC(),
		})
	}
	if len(settings.IssueReports) != issueReportHistoryLimit {
		t.Fatalf(
			"report history count = %d, want %d",
			len(settings.IssueReports),
			issueReportHistoryLimit,
		)
	}
	latest := settings.IssueReports[0]
	settings.rememberIssueReport(latest)
	if len(settings.IssueReports) != issueReportHistoryLimit ||
		settings.IssueReports[0] != latest {
		t.Fatalf("deduplicated report history = %#v", settings.IssueReports)
	}

	settings.IssueReports = append(settings.IssueReports, IssueReportRecord{
		ReportID:   "not-a-report-id",
		IssueURL:   "https://example.com/not-an-issue",
		Capability: "invalid",
		Repository: "aram-frontend",
		Situation:  "Invalid",
		CreatedAt:  time.Now(),
	})
	settings.normalize()
	if len(settings.IssueReports) != issueReportHistoryLimit {
		t.Fatalf("invalid report survived normalization: %#v", settings.IssueReports)
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
