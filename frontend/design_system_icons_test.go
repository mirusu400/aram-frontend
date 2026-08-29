package frontend

import (
	"image/color"
	"testing"
)

// TestModernToolbarIconsCoverEveryAction guards the modern skin against a
// toolbar action whose icon name has no vector glyph: such a button would
// silently fall back to a text label and break the icon row. Every name the
// toolbar passes to addAction must render here.
func TestModernToolbarIconsCoverEveryAction(t *testing.T) {
	for _, name := range []string{
		"open", "play", "pause", "stop",
		"reset", "settings", "keypad", "fullscreen",
	} {
		if drawModernIcon(name, color.Black) == nil {
			t.Errorf("modern toolbar icon %q did not render", name)
		}
	}
	if drawModernIcon("no-such-icon", color.Black) != nil {
		t.Error("an unknown icon name must return nil so the button keeps its text label")
	}
}
