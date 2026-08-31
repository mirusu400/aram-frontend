package frontend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
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
	settings.DisplayEffect = "broken"
	settings.DisplayEffectStrength = 140
	settings.StateSlot = 99
	settings.Speed = 3.7
	settings.normalize()
	if settings.Rotation != 0 ||
		settings.Language != string(LanguageEnglish) ||
		settings.ThemeMode != "light" ||
		settings.ScreenLayout != "center" ||
		settings.Filter != "nearest" ||
		settings.DisplayEffect != displayEffectFeaturePhoneTFT ||
		settings.DisplayEffectStrength != displayEffectStrengthMax ||
		settings.StateSlot != 0 ||
		settings.Speed != 1 {
		t.Fatalf("normalized settings = %#v", settings)
	}
}

func TestDisplayEffectPresetsHaveAStableCycleOrder(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)

	shell := &Shell{settings: defaultSettings()}
	if shell.settings.DisplayEffect != displayEffectFeaturePhoneTFT {
		t.Fatalf("default display effect = %q, want TFT", shell.settings.DisplayEffect)
	}
	shell.settings.DisplayEffect = displayEffectCRTTV
	want := displayEffectChoices()
	for _, effect := range want {
		shell.cycleDisplayEffect()
		if shell.settings.DisplayEffect != effect {
			t.Fatalf("cycled display effect = %q, want %q", shell.settings.DisplayEffect, effect)
		}
	}
}

func TestDisplayEffectPresetChoicesMatchTheProductOrder(t *testing.T) {
	want := []string{
		displayEffectOff,
		displayEffectCrispFit,
		displayEffectFeaturePhoneTFT,
		displayEffectFeaturePhoneSTN,
		displayEffectSmoothPixel,
		displayEffectCRTTV,
	}
	if got := displayEffectChoices(); !slices.Equal(got, want) {
		t.Fatalf("display effect choices = %q, want %q", got, want)
	}
	for _, effect := range want {
		if displayEffectValueLabel(effect) == "" {
			t.Errorf("display effect %q has no label", effect)
		}
	}
}

func TestLegacySettingsMigrateToFeaturePhoneTFT(t *testing.T) {
	settings := defaultSettings()
	if err := json.Unmarshal([]byte(`{"filter":"linear","display_effect":"feature-phone"}`), &settings); err != nil {
		t.Fatal(err)
	}
	settings.normalize()
	if settings.Filter != "linear" ||
		settings.DisplayEffect != displayEffectFeaturePhoneTFT ||
		settings.DisplayEffectStrength != displayEffectStrengthDefault {
		t.Fatalf("legacy display settings = %#v", settings)
	}
}

func TestPerTitleDisplayProfileDoesNotChangeGlobalDefaults(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := &Shell{
		settings: defaultSettings(),
		input:    &InputInfo{DisplayName: "first.dat", SHA256: "ABC123"},
	}
	global := shell.settings.globalDisplayProfile()

	shell.cycleRotation()
	shell.toggleAspectRatio()
	shell.setDisplayEffectStrength(40)

	first := shell.settings.TitleDisplays["sha256:abc123"]
	if first.Rotation != 90 || first.PreserveAspect ||
		first.DisplayEffectStrength != 40 {
		t.Fatalf("first title display profile = %#v", first)
	}
	if got := shell.settings.globalDisplayProfile(); got != global {
		t.Fatalf("title changes modified global display defaults: got %#v, want %#v", got, global)
	}

	shell.input = &InputInfo{DisplayName: "second.dat", SHA256: "def456"}
	if got := shell.displayProfile(); got != global {
		t.Fatalf("new title inherited %#v, want global %#v", got, global)
	}
	shell.cycleScreenLayout()

	shell.input = &InputInfo{DisplayName: "first-renamed.dat", SHA256: "ABC123"}
	if got := shell.displayProfile(); got != first {
		t.Fatalf("first title restored %#v, want %#v", got, first)
	}
	shell.input = nil
	if got := shell.displayProfile(); got != global {
		t.Fatalf("closing the title restored %#v, want global %#v", got, global)
	}
	loaded := loadSettings()
	if got := loaded.TitleDisplays["sha256:abc123"]; got != first {
		t.Fatalf("persisted first title profile = %#v, want %#v", got, first)
	}
}

func TestDisplayProfilesNormalizeTheirOwnValues(t *testing.T) {
	settings := defaultSettings()
	settings.TitleDisplays["sha256:broken"] = DisplayProfile{
		Rotation:              45,
		ScreenLayout:          "outside",
		Filter:                "blurred",
		DisplayEffect:         "unknown",
		DisplayEffectStrength: -20,
	}
	settings.normalize()

	profile := settings.TitleDisplays["sha256:broken"]
	if profile.Rotation != 0 ||
		profile.ScreenLayout != "center" ||
		profile.Filter != "nearest" ||
		profile.DisplayEffect != displayEffectFeaturePhoneTFT ||
		profile.DisplayEffectStrength != displayEffectStrengthMin {
		t.Fatalf("normalized title display profile = %#v", profile)
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

func TestSliderSettingSettersClamp(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := &Shell{settings: defaultSettings()}

	shell.setVolume(150)
	if shell.settings.Volume != 150 {
		t.Fatalf("setVolume(150) = %d, want 150", shell.settings.Volume)
	}
	shell.setVolume(240)
	if shell.settings.Volume != 200 {
		t.Fatalf("setVolume(240) = %d, want clamped to 200", shell.settings.Volume)
	}
	shell.setVolume(-10)
	if shell.settings.Volume != 0 {
		t.Fatalf("setVolume(-10) = %d, want clamped to 0", shell.settings.Volume)
	}
	shell.setAudioLatency(5)
	if shell.settings.AudioLatencyMS != 20 {
		t.Fatalf("setAudioLatency(5) = %d, want clamped to 20", shell.settings.AudioLatencyMS)
	}
	shell.setAudioLatency(999)
	if shell.settings.AudioLatencyMS != 250 {
		t.Fatalf("setAudioLatency(999) = %d, want clamped to 250", shell.settings.AudioLatencyMS)
	}
	shell.setStateSlot(42)
	if shell.settings.StateSlot != 9 {
		t.Fatalf("setStateSlot(42) = %d, want clamped to 9", shell.settings.StateSlot)
	}
	shell.setStateSlot(-3)
	if shell.settings.StateSlot != 0 {
		t.Fatalf("setStateSlot(-3) = %d, want clamped to 0", shell.settings.StateSlot)
	}
	shell.setDisplayEffectStrength(150)
	if got := shell.displayProfile().DisplayEffectStrength; got != displayEffectStrengthMax {
		t.Fatalf("setDisplayEffectStrength(150) = %d, want %d", got, displayEffectStrengthMax)
	}
	shell.setDisplayEffectStrength(-10)
	if got := shell.displayProfile().DisplayEffectStrength; got != displayEffectStrengthMin {
		t.Fatalf("setDisplayEffectStrength(-10) = %d, want %d", got, displayEffectStrengthMin)
	}
}

func TestGraphicsSettingsExposePerTitleStrengthSlider(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := &Shell{
		settings: defaultSettings(),
		input:    &InputInfo{DisplayName: "example.dat", SHA256: "abc123"},
	}
	u := &shellUI{settingsSection: "Graphics"}
	var foundScope, foundStrength bool
	for _, row := range u.settingsRowModels(shell) {
		switch row.label {
		case "Display profile":
			foundScope = row.value == "This title"
		case "Filter strength":
			if row.slider == nil || row.disabled {
				t.Fatalf("filter strength row = %#v", row)
			}
			if row.slider.min != 0 || row.slider.max != 10 || row.slider.value() != 10 {
				t.Fatalf("filter strength slider = %#v, value %d", row.slider, row.slider.value())
			}
			row.slider.apply(6)
			foundStrength = shell.displayProfile().DisplayEffectStrength == 60
		}
	}
	if !foundScope || !foundStrength {
		t.Fatalf("graphics rows missing scope=%t strength=%t", foundScope, foundStrength)
	}
	if shell.settings.DisplayEffectStrength != displayEffectStrengthDefault {
		t.Fatal("per-title strength slider changed the global default")
	}
}

func TestSpeedPresetIndexPicksClosest(t *testing.T) {
	cases := []struct {
		speed float64
		index int
	}{
		{0.5, 0}, {1, 1}, {1.5, 2}, {2, 3}, {2.5, 4}, {3, 5}, {4, 6},
		{0, 0}, {1.4, 2}, {3.9, 6}, {100, 6},
	}
	for _, c := range cases {
		if got := speedPresetIndex(c.speed); got != c.index {
			t.Fatalf("speedPresetIndex(%g) = %d, want %d", c.speed, got, c.index)
		}
	}
}

func TestCycleSpeedAdvancesThroughPresets(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := &Shell{settings: defaultSettings()}
	shell.settings.Speed = 1

	expected := []float64{1.5, 2, 2.5, 3, 4, 0.5, 1}
	for _, want := range expected {
		shell.cycleSpeed()
		if shell.settings.Speed != want {
			t.Fatalf("cycleSpeed advanced to %g, want %g", shell.settings.Speed, want)
		}
	}
}

// TestSettingsDefaultCPUIsFastestAvailable pins the CPU default to the
// backend-resolved "fastest" rather than a named core, and migrates the older
// "jit" default forward. Naming a core in the stored settings is what made the
// choice wrong on platforms that do not have it and stale once a faster core
// landed; "fastest" is resolved by the backend at open time and always exists.
func TestCPUProfilingDefaultsOnAndPersistsOff(t *testing.T) {
	if !defaultSettings().CPUProfile {
		t.Fatal("CPU profiling should default on")
	}
	// Dropping omitempty is what lets an explicit off survive a save/load
	// round trip instead of being omitted and defaulting back on.
	blob, err := json.Marshal(Settings{CPUProfile: false})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(blob), `"cpu_profile":false`) {
		t.Fatalf("cpu_profile was omitted from %s", blob)
	}
	loaded := defaultSettings()
	if err := json.Unmarshal(blob, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.CPUProfile {
		t.Fatal("an explicit off did not survive a load over the on default")
	}
}

func TestSettingsDefaultCPUIsFastestAvailable(t *testing.T) {
	if got := defaultSettings().CPUChoice; got != "fastest" {
		t.Fatalf("default CPUChoice = %q, want %q", got, "fastest")
	}
	for _, stored := range []string{"", "jit"} {
		s := defaultSettings()
		s.CPUChoice = stored
		s.normalize()
		if s.CPUChoice != "fastest" {
			t.Fatalf("stored CPUChoice %q normalized to %q, want %q", stored, s.CPUChoice, "fastest")
		}
	}
	// An explicit choice of a specific core is preserved.
	for _, stored := range []string{"precise", "native"} {
		s := defaultSettings()
		s.CPUChoice = stored
		s.normalize()
		if s.CPUChoice != stored {
			t.Fatalf("explicit CPUChoice %q was rewritten to %q", stored, s.CPUChoice)
		}
	}
}
