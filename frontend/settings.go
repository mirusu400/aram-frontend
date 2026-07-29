package frontend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const recentFileLimit = 10

type Settings struct {
	RecentFiles      []string `json:"recent_files"`
	IntegerScaling   bool     `json:"integer_scaling"`
	PreserveAspect   bool     `json:"preserve_aspect"`
	LastFirmwarePath string   `json:"last_firmware_path,omitempty"`
	Rotation         int      `json:"rotation"`
	ScreenLayout     string   `json:"screen_layout"`
	Filter           string   `json:"filter"`
	StateSlot        int      `json:"state_slot"`
	Speed            float64  `json:"speed"`
	Muted            bool     `json:"muted"`
	Volume           int      `json:"volume"`
	AudioLatencyMS   int      `json:"audio_latency_ms"`
	KeyboardProfile  string   `json:"keyboard_profile"`
}

func defaultSettings() Settings {
	return Settings{
		IntegerScaling:  true,
		PreserveAspect:  true,
		ScreenLayout:    "center",
		Filter:          "nearest",
		StateSlot:       0,
		Speed:           1,
		Volume:          100,
		AudioLatencyMS:  60,
		KeyboardProfile: "default",
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
	if s.StateSlot < 0 || s.StateSlot > 9 {
		s.StateSlot = 0
	}
	if s.Speed != 0.5 && s.Speed != 1 && s.Speed != 2 && s.Speed != 4 {
		s.Speed = 1
	}
	if s.Volume < 0 || s.Volume > 100 {
		s.Volume = 100
	}
	if s.AudioLatencyMS < 20 || s.AudioLatencyMS > 250 {
		s.AudioLatencyMS = 60
	}
	if s.KeyboardProfile == "" {
		s.KeyboardProfile = "default"
	}
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
