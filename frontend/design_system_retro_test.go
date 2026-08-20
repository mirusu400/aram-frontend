package frontend

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// requiredRetroSlices lists every nine-slice tile retroComponents consumes.
// The pack ships more (tabs, input fields, lcd_bezel, …) but a hole in this
// set would panic at skin-switch time, so it is pinned here.
var requiredRetroSlices = []string{
	"button_idle", "button_hover", "button_pressed", "button_disabled",
	"button_primary_idle", "button_primary_hover", "button_primary_pressed",
	"selection", "panel", "panel_sunken", "statusbar", "titlebar",
	"progress_track", "scroll_track", "lcd_bezel",
	"softkey_idle", "softkey_pressed",
}

// requiredRetroIcons lists the toolbar icons buildApplicationToolbar wires up;
// each must exist in both the normal and the inverted ink set.
var requiredRetroIcons = []string{"open", "play", "pause", "stop", "reset", "settings"}

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
			for _, name := range requiredRetroIcons {
				for _, kind := range []string{"icon", "icon_inv"} {
					path := fmt.Sprintf("retrothemes/%s/%s/%s.png", theme, kind, name)
					if _, err := retroThemeFS.ReadFile(path); err != nil {
						t.Errorf("missing icon: %v", err)
					}
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

// TestRetroTypographyLineBox pins the metric invariant the sprite skins rely
// on: EbitenUI centers a label by its line box and places the baseline one
// ascent below the top, and a MultiFace reports the largest ascent among its
// members. A fallback face taller than the pixel font therefore pushes every
// glyph down inside its widget, which is exactly what a naive same-size
// fallback did.
func TestRetroTypographyLineBox(t *testing.T) {
	pixel, err := text.NewGoTextFaceSource(bytes.NewReader(terrarumSansOTF))
	if err != nil {
		t.Fatalf("load pixel font: %v", err)
	}
	typography := retroTypography()
	if typography.CenterNudge != retroCenterNudge {
		t.Errorf("CenterNudge = %d, want %d",
			typography.CenterNudge, retroCenterNudge)
	}
	for _, tc := range []struct {
		name string
		face *text.Face
		size float64
	}{
		{"Caption", typography.Caption, 20},
		{"Body", typography.Body, 20},
		{"Strong", typography.Strong, 20},
		{"Heading", typography.Heading, 40},
		{"Display", typography.Display, 40},
	} {
		var alone text.Face = &text.GoTextFace{Source: pixel, Size: tc.size}
		want := alone.Metrics()
		got := (*tc.face).Metrics()
		if got.HAscent != want.HAscent || got.HDescent != want.HDescent {
			t.Errorf("%s: line box is asc=%.2f desc=%.2f, want the pixel font's asc=%.2f desc=%.2f",
				tc.name, got.HAscent, got.HDescent, want.HAscent, want.HDescent)
		}
	}
}

// TestRetroTextPaddingIsSizeNeutral keeps the vertical nudge from changing any
// widget's preferred size, which sums the top and bottom insets.
func TestRetroTextPaddingIsSizeNeutral(t *testing.T) {
	p := retroTextPadding()
	if p.Top+p.Bottom != 0 {
		t.Errorf("padding top %d + bottom %d = %d, want 0",
			p.Top, p.Bottom, p.Top+p.Bottom)
	}
	if p.Top >= 0 {
		t.Errorf("padding top = %d, want a negative value that lifts text", p.Top)
	}
}
