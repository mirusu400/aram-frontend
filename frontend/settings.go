package frontend

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
)

const recentFileLimit = 10

const (
	displayEffectStrengthMin     = 0
	displayEffectStrengthMax     = 100
	displayEffectStrengthDefault = 100
)

const (
	displayEffectOff                = "off"
	displayEffectCrispFit           = "crisp-fit"
	displayEffectFeaturePhoneTFT    = "feature-phone-tft"
	displayEffectFeaturePhoneSTN    = "feature-phone-stn"
	displayEffectSmoothPixel        = "smooth-pixel"
	displayEffectCRTTV              = "crt-tv"
	displayEffectFeaturePhoneLegacy = "feature-phone"
)

func displayEffectChoices() []string {
	return []string{
		displayEffectOff,
		displayEffectCrispFit,
		displayEffectFeaturePhoneTFT,
		displayEffectFeaturePhoneSTN,
		displayEffectSmoothPixel,
		displayEffectCRTTV,
	}
}

func isDisplayEffectChoice(effect string) bool {
	for _, choice := range displayEffectChoices() {
		if effect == choice {
			return true
		}
	}
	return false
}

// displayEffectIndex returns the dropdown position of effect, defaulting to the
// first entry (Original) when the saved value is unknown or legacy.
func displayEffectIndex(effect string) int {
	for index, choice := range displayEffectChoices() {
		if choice == effect {
			return index
		}
	}
	return 0
}

// speedPresets are the emulation speeds the control offers, in slider order.
var speedPresets = []float64{0.5, 1, 1.5, 2, 2.5, 3, 4}

// speedPresetIndex returns the preset closest to speed, so values saved by
// older builds still land on a valid slider position.
func speedPresetIndex(speed float64) int {
	best := 0
	for index, preset := range speedPresets {
		if math.Abs(preset-speed) < math.Abs(speedPresets[best]-speed) {
			best = index
		}
	}
	return best
}

// isSpeedPreset reports whether speed is one of the offered presets. Settings
// written by another build can name a speed this build no longer offers, and
// such a value must fall back to 1x rather than sit outside the slider.
func isSpeedPreset(speed float64) bool {
	for _, preset := range speedPresets {
		if preset == speed {
			return true
		}
	}
	return false
}

type ControllerProfile struct {
	KeyboardProfile  string            `json:"keyboard_profile"`
	KeyboardBindings map[string]string `json:"keyboard_bindings,omitempty"`
	GamepadEnabled   bool              `json:"gamepad_enabled"`
	GamepadLayout    string            `json:"gamepad_layout"`
	GamepadAnalog    bool              `json:"gamepad_analog"`
	GamepadDeadzone  int               `json:"gamepad_deadzone"`
	GamepadBindings  map[string]string `json:"gamepad_bindings,omitempty"`
}

// DisplayProfile is the complete guest-presentation policy. The top-level
// Settings fields remain the global defaults for backwards compatibility;
// identified titles can override them in TitleDisplays without changing what
// a newly opened title inherits.
type DisplayProfile struct {
	IntegerScaling        bool   `json:"integer_scaling"`
	PreserveAspect        bool   `json:"preserve_aspect"`
	Rotation              int    `json:"rotation"`
	ScreenLayout          string `json:"screen_layout"`
	Filter                string `json:"filter"`
	DisplayEffect         string `json:"display_effect"`
	DisplayEffectStrength int    `json:"display_effect_strength"`
}

type Settings struct {
	RecentFiles           []RecentEntry             `json:"recent_files"`
	Language              string                    `json:"language"`
	ThemeMode             string                    `json:"theme_mode"`
	ThemeFamily           string                    `json:"theme_family"`
	IntegerScaling        bool                      `json:"integer_scaling"`
	PreserveAspect        bool                      `json:"preserve_aspect"`
	LastFirmwarePath      string                    `json:"last_firmware_path,omitempty"`
	Rotation              int                       `json:"rotation"`
	ScreenLayout          string                    `json:"screen_layout"`
	Filter                string                    `json:"filter"`
	DisplayEffect         string                    `json:"display_effect"`
	DisplayEffectStrength int                       `json:"display_effect_strength"`
	TitleDisplays         map[string]DisplayProfile `json:"title_display_profiles,omitempty"`
	StateSlot             int                       `json:"state_slot"`
	FontChoice            string                    `json:"font_choice"`
	CustomFontPath        string                    `json:"custom_font_path,omitempty"`
	CPUChoice             string                    `json:"cpu_choice"`
	Speed                 float64                   `json:"speed"`
	Muted                 bool                      `json:"muted"`
	Volume                int                       `json:"volume"`
	AudioLatencyMS        int                       `json:"audio_latency_ms"`
	AudioDeviceID         string                    `json:"audio_device_id,omitempty"`
	AudioMixMode          bool                      `json:"audio_mix_mode"`
	AudioSoften           bool                      `json:"audio_soften"`
	// AudioLowPower trades audio fidelity for CPU on weak hardware by rendering
	// SMAF FM synthesis at a reduced sample rate. False (the default) renders
	// at full quality. Baked into the next machine created, like AudioMixMode.
	AudioLowPower bool `json:"audio_low_power"`
	CPUProfile    bool `json:"cpu_profile"`
	UIPriority    bool `json:"ui_priority"`
	// DisplaySync paces the guest by host ticks instead of wall time whenever
	// the display refresh is within a few percent of the guest frame rate, so
	// a 62.5 Hz title on a 60 Hz display shows one guest frame per refresh
	// instead of a doubled frame every 400 ms. True (the default) enables it;
	// it only engages while vsync is on, because the tick rate must be the
	// refresh rate for the plan to mean anything.
	DisplaySync bool `json:"display_sync"`
	// VsyncDisabled lets the host draw as fast as it can instead of waiting
	// for the display, trading tearing and power for lower input latency.
	VsyncDisabled bool `json:"vsync_disabled"`
	// GuestWidthOverride widens the guest framebuffer (experimental widescreen).
	// Zero keeps the device-native width. Height stays native.
	GuestWidthOverride  int                          `json:"guest_width_override,omitempty"`
	KeyboardProfile     string                       `json:"keyboard_profile"`
	KeyboardBindings    map[string]string            `json:"keyboard_bindings,omitempty"`
	GamepadEnabled      bool                         `json:"gamepad_enabled"`
	GamepadLayout       string                       `json:"gamepad_layout"`
	GamepadAnalog       bool                         `json:"gamepad_analog"`
	GamepadDeadzone     int                          `json:"gamepad_deadzone"`
	GamepadBindings     map[string]string            `json:"gamepad_bindings,omitempty"`
	PerTitleControls    bool                         `json:"per_title_controls"`
	TitleControllers    map[string]ControllerProfile `json:"title_controller_profiles,omitempty"`
	ShowVirtualKeypad   bool                         `json:"show_virtual_keypad"`
	ShowControlsWithPad bool                         `json:"show_controls_with_pad"`
	TouchDpadCircular   bool                         `json:"touch_dpad_circular"`
	VibrationEnabled    bool                         `json:"vibration_enabled"`
	TouchControlScale   int                          `json:"touch_control_scale,omitempty"`
	TouchDeckRatio      int                          `json:"touch_deck_ratio,omitempty"`
	TouchLayout         map[string]TouchPlacement    `json:"touch_layout,omitempty"`
	TouchHidden         map[string]bool              `json:"touch_hidden,omitempty"`
	TouchGridStep       int                          `json:"touch_grid_step,omitempty"`
	UpdateChannel       string                       `json:"update_channel"`
	WelcomeCompleted    bool                         `json:"welcome_completed"`
	IssueReports        []IssueReportRecord          `json:"issue_reports,omitempty"`
	// GameLibraryFolders are the roots the Home "Installed" tab scans
	// recursively for openable titles. FavoriteFiles are the paths starred on
	// Home. Both are cleaned and de-duplicated by normalize.
	GameLibraryFolders []string `json:"game_library_folders,omitempty"`
	FavoriteFiles      []string `json:"favorite_files,omitempty"`
}

// RecentEntry pairs an openable path with the display name it was opened
// under. The path alone is not readable for every input: a desktop
// drag-and-drop opens from a private cache copy ("drop-<random>.ext") and an
// Android document picker hands back an opaque content:// URI, so the name
// captured at open time is the only readable label either surface has.
type RecentEntry struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// UnmarshalJSON accepts a bare path string (the format written before this
// type existed) in addition to the {path, name} object, so an existing
// settings.json still loads without a migration step.
func (e *RecentEntry) UnmarshalJSON(data []byte) error {
	var path string
	if err := json.Unmarshal(data, &path); err == nil {
		e.Path = path
		e.Name = ""
		return nil
	}
	type recentEntryAlias RecentEntry
	var alias recentEntryAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*e = RecentEntry(alias)
	return nil
}

// recentEntriesFromPaths builds recent entries with no explicit display name;
// display falls back to each path's own basename. Convenience for callers
// that only have bare paths (tests, legacy migration).
func recentEntriesFromPaths(paths ...string) []RecentEntry {
	entries := make([]RecentEntry, 0, len(paths))
	for _, path := range paths {
		entries = append(entries, RecentEntry{Path: path})
	}
	return entries
}

// recentEntryPaths extracts the bare paths, for callers (native recent-file
// pickers) that only accept []string.
func recentEntryPaths(entries []RecentEntry) []string {
	paths := make([]string, len(entries))
	for index, entry := range entries {
		paths[index] = entry.Path
	}
	return paths
}

// TouchPlacement stores a custom on-screen button position as its center
// point, normalized to the current display size so a saved layout survives
// rotation and window-size changes.
type TouchPlacement struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

const (
	touchControlScaleMin = 80
	touchControlScaleMax = 140
	// touchDeckRatio bounds how much of the screen height the control deck
	// may claim. The guest display keeps the rest, so the floor protects the
	// controls and the ceiling protects the game.
	touchDeckRatioMin = 20
	touchDeckRatioMax = 65
	// touchGridStep bounds the layout editor's snap grid, measured in screen
	// pixels. Zero means the grid is off and a button drops wherever the
	// finger lifts; a non-zero step aligns every drag to that pixel lattice.
	touchGridStepMin = 8
	touchGridStepMax = 64
)

// touchScaleFactor maps the persisted percentage (0 means unset) to a
// clamped multiplier for the on-screen touch controls.
func touchScaleFactor(percent int) float64 {
	if percent == 0 {
		return 1
	}
	return float64(clampInt(percent, touchControlScaleMin, touchControlScaleMax)) / 100
}

func defaultSettings() Settings {
	return Settings{
		Language:              string(systemLanguage()),
		ThemeMode:             systemThemeMode(),
		ThemeFamily:           themeFamilyModern,
		IntegerScaling:        true,
		PreserveAspect:        true,
		ScreenLayout:          "center",
		Filter:                "nearest",
		DisplayEffect:         displayEffectFeaturePhoneTFT,
		DisplayEffectStrength: displayEffectStrengthDefault,
		TitleDisplays:         make(map[string]DisplayProfile),
		StateSlot:             0,
		FontChoice:            "mulmaru",
		CPUChoice:             "fastest",
		Speed:                 1,
		Volume:                100,
		AudioLatencyMS:        60,
		AudioSoften:           true,
		KeyboardProfile:       "default",
		GamepadEnabled:        true,
		GamepadLayout:         "standard",
		GamepadAnalog:         true,
		GamepadDeadzone:       30,
		TitleControllers:      make(map[string]ControllerProfile),
		UpdateChannel:         string(updateChannelStable),
		VibrationEnabled:      true,
		TouchDpadCircular:     true,
		CPUProfile:            true,
		DisplaySync:           true,
	}
}

func loadSettings() Settings {
	settings := defaultSettings()
	// readSettingsBlob is the platform storage seam: a settings.json file on
	// hosts with a filesystem, browser localStorage on the web/wasm build.
	data, err := readSettingsBlob()
	if err != nil || json.Unmarshal(data, &settings) != nil {
		return settings
	}
	if len(settings.RecentFiles) > recentFileLimit {
		settings.RecentFiles = settings.RecentFiles[:recentFileLimit]
	}
	settings.normalize()
	return settings
}

func (s *Settings) normalize() {
	s.Language = string(normalizeLanguage(s.Language))
	if s.ThemeMode != "light" && s.ThemeMode != "dark" {
		s.ThemeMode = "light"
	}
	if s.ThemeFamily != themeFamilyModern && !isRetroFamily(s.ThemeFamily) {
		s.ThemeFamily = themeFamilyModern
	}
	display := s.globalDisplayProfile()
	display.normalize()
	s.setGlobalDisplayProfile(display)
	if s.TitleDisplays == nil {
		s.TitleDisplays = make(map[string]DisplayProfile)
	}
	for key, profile := range s.TitleDisplays {
		profile.normalize()
		s.TitleDisplays[key] = profile
	}
	if s.FontChoice != "galmuri9" && s.FontChoice != "neodgm" && s.FontChoice != "mulmaru" && s.FontChoice != "custom" {
		s.FontChoice = "galmuri9"
	}
	if s.FontChoice == "custom" && s.CustomFontPath == "" {
		s.FontChoice = "galmuri9"
	}
	// "fastest" asks the backend for the best core this build provides instead
	// of naming one, so the stored setting stays correct across platforms and
	// as faster cores land. Settings written before that existed carry the old
	// default, "jit", which is now the slower of the two recompilers on every
	// measured workload; move them forward rather than leaving them behind.
	if s.CPUChoice == "" || s.CPUChoice == "jit" {
		s.CPUChoice = "fastest"
	}
	if s.TouchDeckRatio != 0 {
		s.TouchDeckRatio = clampInt(s.TouchDeckRatio, touchDeckRatioMin, touchDeckRatioMax)
	}
	if s.TouchGridStep != 0 {
		s.TouchGridStep = clampInt(s.TouchGridStep, touchGridStepMin, touchGridStepMax)
	}
	if len(s.TouchHidden) > 0 {
		// A button hidden by an older layout that no longer exists would
		// otherwise sit in the file forever.
		for id := range s.TouchHidden {
			if !s.TouchHidden[id] || !isTouchButtonID(id) {
				delete(s.TouchHidden, id)
			}
		}
	}
	if s.StateSlot < 0 || s.StateSlot > 9 {
		s.StateSlot = 0
	}
	if !isSpeedPreset(s.Speed) {
		s.Speed = 1
	}
	if s.Volume < 0 || s.Volume > 200 {
		s.Volume = 100
	}
	if s.AudioLatencyMS < 20 || s.AudioLatencyMS > 250 {
		s.AudioLatencyMS = 60
	}
	if s.KeyboardProfile != "default" &&
		s.KeyboardProfile != "wasd" &&
		s.KeyboardProfile != "custom" {
		s.KeyboardProfile = "default"
	}
	if s.GamepadLayout != "standard" &&
		s.GamepadLayout != "swapped" &&
		s.GamepadLayout != "custom" {
		s.GamepadLayout = "standard"
	}
	if s.GamepadDeadzone < 15 || s.GamepadDeadzone > 50 {
		s.GamepadDeadzone = 30
	}
	s.UpdateChannel = string(normalizeUpdateChannel(s.UpdateChannel))
	s.normalizeIssueReports()
	s.KeyboardBindings = normalizeKeyboardBindingIDs(s.KeyboardBindings)
	s.GamepadBindings = normalizeGamepadBindingIDs(s.GamepadBindings)
	if s.TitleControllers == nil {
		s.TitleControllers = make(map[string]ControllerProfile)
	}
	for key, profile := range s.TitleControllers {
		profile.normalize()
		s.TitleControllers[key] = profile
	}
	s.GameLibraryFolders = cleanPathList(s.GameLibraryFolders)
	s.FavoriteFiles = cleanPathList(s.FavoriteFiles)
}

func (s Settings) globalDisplayProfile() DisplayProfile {
	return DisplayProfile{
		IntegerScaling:        s.IntegerScaling,
		PreserveAspect:        s.PreserveAspect,
		Rotation:              s.Rotation,
		ScreenLayout:          s.ScreenLayout,
		Filter:                s.Filter,
		DisplayEffect:         s.DisplayEffect,
		DisplayEffectStrength: s.DisplayEffectStrength,
	}
}

func (s *Settings) setGlobalDisplayProfile(profile DisplayProfile) {
	profile.normalize()
	s.IntegerScaling = profile.IntegerScaling
	s.PreserveAspect = profile.PreserveAspect
	s.Rotation = profile.Rotation
	s.ScreenLayout = profile.ScreenLayout
	s.Filter = profile.Filter
	s.DisplayEffect = profile.DisplayEffect
	s.DisplayEffectStrength = profile.DisplayEffectStrength
}

func (profile *DisplayProfile) normalize() {
	switch profile.Rotation {
	case 0, 90, 180, 270:
	default:
		profile.Rotation = 0
	}
	if profile.ScreenLayout != "center" && profile.ScreenLayout != "stretch" {
		profile.ScreenLayout = "center"
	}
	if profile.Filter != "nearest" && profile.Filter != "linear" {
		profile.Filter = "nearest"
	}
	if profile.DisplayEffect == displayEffectFeaturePhoneLegacy {
		profile.DisplayEffect = displayEffectFeaturePhoneTFT
	} else if !isDisplayEffectChoice(profile.DisplayEffect) {
		profile.DisplayEffect = displayEffectFeaturePhoneTFT
	}
	profile.DisplayEffectStrength = clampInt(
		profile.DisplayEffectStrength,
		displayEffectStrengthMin,
		displayEffectStrengthMax,
	)
}

func (s Settings) globalControllerProfile() ControllerProfile {
	profile := ControllerProfile{
		KeyboardProfile:  s.KeyboardProfile,
		KeyboardBindings: cloneStringMap(s.KeyboardBindings),
		GamepadEnabled:   s.GamepadEnabled,
		GamepadLayout:    s.GamepadLayout,
		GamepadAnalog:    s.GamepadAnalog,
		GamepadDeadzone:  s.GamepadDeadzone,
		GamepadBindings:  cloneStringMap(s.GamepadBindings),
	}
	profile.normalize()
	return profile
}

func (s *Settings) setGlobalControllerProfile(profile ControllerProfile) {
	profile.normalize()
	s.KeyboardProfile = profile.KeyboardProfile
	s.KeyboardBindings = cloneStringMap(profile.KeyboardBindings)
	s.GamepadEnabled = profile.GamepadEnabled
	s.GamepadLayout = profile.GamepadLayout
	s.GamepadAnalog = profile.GamepadAnalog
	s.GamepadDeadzone = profile.GamepadDeadzone
	s.GamepadBindings = cloneStringMap(profile.GamepadBindings)
}

func (profile *ControllerProfile) normalize() {
	if profile.KeyboardProfile != "default" &&
		profile.KeyboardProfile != "wasd" &&
		profile.KeyboardProfile != "custom" {
		profile.KeyboardProfile = "default"
	}
	switch profile.GamepadLayout {
	case "standard", "swapped", "custom":
	default:
		profile.GamepadLayout = "standard"
	}
	if profile.GamepadDeadzone < 15 || profile.GamepadDeadzone > 50 {
		profile.GamepadDeadzone = 30
	}
	profile.KeyboardBindings = normalizeKeyboardBindingIDs(profile.KeyboardBindings)
	profile.GamepadBindings = normalizeGamepadBindingIDs(profile.GamepadBindings)
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

// titleSettingsKey is shared by every per-title frontend profile. A content
// hash survives renames and moves; display name is the fallback for backends
// that cannot identify the input bytes.
func titleSettingsKey(input *InputInfo) string {
	if input == nil {
		return ""
	}
	if input.SHA256 != "" {
		return "sha256:" + strings.ToLower(input.SHA256)
	}
	if input.DisplayName != "" {
		return "title:" + strings.ToLower(input.DisplayName)
	}
	return ""
}

// addRecent records path as the most recent input, deduplicating by path and
// capping the list at recentFileLimit. name is the display name it was opened
// under (see RecentEntry); an empty name keeps whatever name a prior entry for
// the same path already carried, so a caller that cannot supply one does not
// blank out a good name already on file.
func (s *Settings) addRecent(path, name string) {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	if name == "" {
		for _, existing := range s.RecentFiles {
			if strings.EqualFold(filepath.Clean(existing.Path), path) {
				name = existing.Name
				break
			}
		}
	}
	recent := []RecentEntry{{Path: path, Name: name}}
	for _, existing := range s.RecentFiles {
		if strings.EqualFold(filepath.Clean(existing.Path), path) {
			continue
		}
		recent = append(recent, existing)
		if len(recent) == recentFileLimit {
			break
		}
	}
	s.RecentFiles = recent
}

func (s Settings) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeSettingsBlob(append(data, '\n'))
}
