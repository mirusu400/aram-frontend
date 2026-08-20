package frontend

import (
	"bytes"
	"fmt"
	"image"
	"testing"
)

// requiredRetroSlices lists every nine-slice tile retroComponents consumes.
// The pack ships more (tabs, input fields, lcd_bezel, …) but a hole in this
// set would panic at skin-switch time, so it is pinned here.
var requiredRetroSlices = []string{
	"button_idle", "button_hover", "button_pressed", "button_disabled",
	"button_primary_idle", "button_primary_hover", "button_primary_pressed",
	"selection", "panel", "panel_sunken", "statusbar", "titlebar",
	"tooltip", "progress_track", "scroll_track",
}

func TestRetroThemeAssetsComplete(t *testing.T) {
	for _, family := range retroFamilies() {
		for _, mode := range []string{"light", "dark"} {
			theme := retroThemeID(family, mode)
			if _, ok := retroPaletteSpecs[theme]; !ok {
				t.Errorf("theme %s has no palette spec", theme)
			}
			for _, name := range requiredRetroSlices {
				path := fmt.Sprintf("retrothemes/%s/nineslice/%s.png", theme, name)
				raw, err := retroThemeFS.ReadFile(path)
				if err != nil {
					t.Errorf("missing tile: %v", err)
					continue
				}
				cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
				if err != nil {
					t.Errorf("decode %s: %v", path, err)
					continue
				}
				if cfg.Width != retroSliceTile || cfg.Height != retroSliceTile {
					t.Errorf("%s is %dx%d, want %dx%d",
						path, cfg.Width, cfg.Height, retroSliceTile, retroSliceTile)
				}
			}
		}
	}
}

func TestRetroPaletteSpecsMatchThemes(t *testing.T) {
	if len(retroPaletteSpecs) != 2*len(retroFamilies()) {
		t.Errorf("palette specs cover %d themes, want %d",
			len(retroPaletteSpecs), 2*len(retroFamilies()))
	}
	base := aramPalette("light")
	for theme := range retroPaletteSpecs {
		p := retroPalette(theme, base)
		if p.Text == p.Canvas {
			t.Errorf("theme %s: text and canvas are identical", theme)
		}
		if p.Success != base.Success || p.Overlay != base.Overlay {
			t.Errorf("theme %s: status/overlay roles must stay on the base palette", theme)
		}
	}
}

func TestNewARAMDesignSystemFamilies(t *testing.T) {
	modern := newARAMDesignSystem("light", themeFamilyModern)
	if modern.Family != themeFamilyModern {
		t.Fatalf("modern family = %q", modern.Family)
	}
	unknown := newARAMDesignSystem("light", "definitely-not-a-family")
	if unknown.Family != themeFamilyModern {
		t.Fatalf("unknown family should fall back to modern, got %q", unknown.Family)
	}
	for _, family := range retroFamilies() {
		for _, mode := range []string{"light", "dark"} {
			ds := newARAMDesignSystem(mode, family)
			if ds.Family != family || ds.Mode != mode {
				t.Fatalf("design system built %s/%s, want %s/%s",
					ds.Family, ds.Mode, family, mode)
			}
			c := ds.Components
			for name, ns := range map[string]any{
				"MenuBar": c.MenuBar, "Surface": c.Surface,
				"DialogTitle": c.DialogTitle, "Dropdown": c.Dropdown,
				"Scroll": c.Scroll, "SliderTrack": c.SliderTrack,
				"SliderHandle": c.SliderHandle, "Checkbox": c.Checkbox,
			} {
				if ns == nil {
					t.Fatalf("%s/%s: component %s is nil", family, mode, name)
				}
			}
			if c.PrimaryButton.Image == nil || c.CommandButton.Image == nil {
				t.Fatalf("%s/%s: button images missing", family, mode)
			}
		}
	}
}

func TestThemeFamilySettingNormalization(t *testing.T) {
	s := defaultSettings()
	s.ThemeFamily = "glass-touch"
	s.normalize()
	if s.ThemeFamily != "glass-touch" {
		t.Fatalf("valid family was rewritten to %q", s.ThemeFamily)
	}
	s.ThemeFamily = "bogus"
	s.normalize()
	if s.ThemeFamily != themeFamilyModern {
		t.Fatalf("invalid family normalized to %q, want %q",
			s.ThemeFamily, themeFamilyModern)
	}
	s.ThemeFamily = ""
	s.normalize()
	if s.ThemeFamily != themeFamilyModern {
		t.Fatalf("empty family normalized to %q, want %q",
			s.ThemeFamily, themeFamilyModern)
	}
}
