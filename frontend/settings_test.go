package frontend

import (
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
	settings.Rotation = 17
	settings.ScreenLayout = "broken"
	settings.Filter = "broken"
	settings.StateSlot = 99
	settings.Speed = 3
	settings.normalize()
	if settings.Rotation != 0 ||
		settings.ScreenLayout != "center" ||
		settings.Filter != "nearest" ||
		settings.StateSlot != 0 ||
		settings.Speed != 1 {
		t.Fatalf("normalized settings = %#v", settings)
	}
}

func TestShortenPreservesKoreanRunes(t *testing.T) {
	if got := shorten("마법홀게임실행", 5); got != "마법..." {
		t.Fatalf("shorten Korean = %q", got)
	}
}
