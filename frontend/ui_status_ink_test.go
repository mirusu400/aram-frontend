package frontend

import (
	"image/color"
	"testing"
)

// statusInkLegibleMargin is the luminance separation a status-bar ink must
// keep from the canvas it sits on. Every dark palette's raw disabled ink falls
// short of it, which is why the status line read as unlit.
const statusInkLegibleMargin = 35_000

// statusInkTextShare is how close to the palette's full text ink a lifted
// status ink has to land.
const statusInkTextShare = 0.75

func TestDarkThemesLiftEveryStatusBarInk(t *testing.T) {
	for _, family := range themeFamilyChoices() {
		t.Run(family, func(t *testing.T) {
			palette := newARAMDesignSystem("dark", family).Palette
			if !paletteIsDark(palette) {
				t.Fatal("the dark variant is not ranked as a dark palette")
			}
			text := relativeLuminance(palette.Text)
			canvas := relativeLuminance(palette.Canvas)
			for _, role := range []struct {
				name string
				ink  color.NRGBA
			}{
				{"muted", palette.TextMuted},
				{"disabled", palette.TextDisabled},
			} {
				lifted := statusBarInk(palette, role.ink)
				got := relativeLuminance(lifted)
				if got <= relativeLuminance(role.ink) {
					t.Fatalf("%s ink was not lifted: %.0f", role.name, got)
				}
				if got < text*statusInkTextShare {
					t.Fatalf(
						"%s ink landed at %.0f, want at least %.0f of the text ink",
						role.name,
						got,
						text*statusInkTextShare,
					)
				}
				if got-canvas < statusInkLegibleMargin {
					t.Fatalf(
						"%s ink clears the canvas by only %.0f",
						role.name,
						got-canvas,
					)
				}
			}
		})
	}
}

// A light theme already carries the contrast, so the status bar must keep the
// inks its palette was designed with.
func TestLightThemesKeepTheirStatusBarInks(t *testing.T) {
	for _, family := range themeFamilyChoices() {
		t.Run(family, func(t *testing.T) {
			palette := newARAMDesignSystem("light", family).Palette
			if paletteIsDark(palette) {
				t.Fatal("the light variant is ranked as a dark palette")
			}
			if got := statusBarInk(palette, palette.TextMuted); got != palette.TextMuted {
				t.Fatalf("muted ink was changed to %+v", got)
			}
			if got := statusBarInk(palette, palette.TextDisabled); got != palette.TextDisabled {
				t.Fatalf("disabled ink was changed to %+v", got)
			}
		})
	}
}

// The bar caches its layout, so a status reading has to be written through
// setStatusLabel. The indicator cluster is anchored from the trailing row's
// preferred width, and a label that grows without a relayout leaves the
// cluster sitting on top of the text it is supposed to follow.
func TestStatusReadingsAreWrittenThroughTheRelayoutPath(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	if view.statusBar == nil {
		t.Fatal("the status bar was not retained, so no relayout can be requested")
	}

	reading := "RUNNING  •  1x (100%)  •  NEAREST"
	view.setStatusLabel(view.statusMeta, reading)
	if view.statusMeta.Label != reading {
		t.Fatalf("meta label = %q", view.statusMeta.Label)
	}
	view.setStatusLabel(view.statusMeta, reading)
	if view.statusMeta.Label != reading {
		t.Fatalf("rewriting the same reading changed the label to %q", view.statusMeta.Label)
	}
	view.setStatusLabel(nil, reading)
}

// The sync path itself must use the helper, or every tick would write the
// growing speed reading straight onto a stale layout again.
func TestStatusSyncPublishesThroughTheRelayoutPath(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	view.statusMeta.Label = ""
	view.sync(shell)
	if view.statusMeta.Label == "" {
		t.Fatal("sync published no status reading")
	}
}
