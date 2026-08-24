package frontend

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const recentFileLimit = 10

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

type Settings struct {
	RecentFiles       []string                     `json:"recent_files"`
	Language          string                       `json:"language"`
	ThemeMode         string                       `json:"theme_mode"`
	ThemeFamily       string                       `json:"theme_family"`
	IntegerScaling    bool                         `json:"integer_scaling"`
	PreserveAspect    bool                         `json:"preserve_aspect"`
	LastFirmwarePath  string                       `json:"last_firmware_path,omitempty"`
	Rotation          int                          `json:"rotation"`
	ScreenLayout      string                       `json:"screen_layout"`
	Filter            string                       `json:"filter"`
	StateSlot         int                          `json:"state_slot"`
	FontChoice        string                       `json:"font_choice"`
	CustomFontPath    string                       `json:"custom_font_path,omitempty"`
	CPUChoice         string                       `json:"cpu_choice"`
	Speed             float64                      `json:"speed"`
	Muted             bool                         `json:"muted"`
	Volume            int                          `json:"volume"`
	AudioLatencyMS    int                          `json:"audio_latency_ms"`
	AudioDeviceID     string                       `json:"audio_device_id,omitempty"`
	AudioMixMode      bool                         `json:"audio_mix_mode"`
	AudioSoften       bool                         `json:"audio_soften"`
	KeyboardProfile   string                       `json:"keyboard_profile"`
	KeyboardBindings  map[string]string            `json:"keyboard_bindings,omitempty"`
	GamepadEnabled    bool                         `json:"gamepad_enabled"`
	GamepadLayout     string                       `json:"gamepad_layout"`
	GamepadAnalog     bool                         `json:"gamepad_analog"`
	GamepadDeadzone   int                          `json:"gamepad_deadzone"`
	GamepadBindings   map[string]string            `json:"gamepad_bindings,omitempty"`
	PerTitleControls  bool                         `json:"per_title_controls"`
	TitleControllers  map[string]ControllerProfile `json:"title_controller_profiles,omitempty"`
	ShowVirtualKeypad bool                         `json:"show_virtual_keypad"`
	TouchControlScale int                          `json:"touch_control_scale,omitempty"`
	TouchDeckRatio    int                          `json:"touch_deck_ratio,omitempty"`
	TouchLayout       map[string]TouchPlacement    `json:"touch_layout,omitempty"`
	TouchHidden       map[string]bool              `json:"touch_hidden,omitempty"`
	UpdateChannel     string                       `json:"update_channel"`
	WelcomeCompleted  bool                         `json:"welcome_completed"`
	IssueReports      []IssueReportRecord          `json:"issue_reports,omitempty"`
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
		Language:         string(systemLanguage()),
		ThemeMode:        "light",
		ThemeFamily:      themeFamilyModern,
		IntegerScaling:   true,
		PreserveAspect:   true,
		ScreenLayout:     "center",
		Filter:           "nearest",
		StateSlot:        0,
		FontChoice:       "galmuri9",
		CPUChoice:        "fastest",
		Speed:            1,
		Volume:           100,
		AudioLatencyMS:   60,
		KeyboardProfile:  "default",
		GamepadEnabled:   true,
		GamepadLayout:    "standard",
		GamepadAnalog:    true,
		GamepadDeadzone:  30,
		TitleControllers: make(map[string]ControllerProfile),
		UpdateChannel:    string(updateChannelStable),
	}
}

func loadSettings() Settings {
	settings := defaultSettings()
	path, err := settingsPath()
	if err != nil {
		return settings
	}
	data, err := os.ReadFile(path)
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
	switch s.Rotation {
	case 0, 90, 180, 270:
	default:
		s.Rotation = 0
	}
	if s.ScreenLayout != "center" && s.ScreenLayout != "stretch" {
		s.ScreenLayout = "center"
	}
	if s.Filter != "nearest" && s.Filter != "linear" {
		s.Filter = "nearest"
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
	if s.Volume < 0 || s.Volume > 100 {
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

func (s *Settings) addRecent(path string) {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	recent := []string{path}
	for _, existing := range s.RecentFiles {
		if strings.EqualFold(filepath.Clean(existing), path) {
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
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func settingsPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ARAM", "settings.json"), nil
}
