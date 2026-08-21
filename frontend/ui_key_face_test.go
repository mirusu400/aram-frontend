package frontend

import (
	"image/color"
	"testing"
)

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// A key legend is read mid-game, so it takes the full ink role. The muted role
// is for secondary chrome, and on the dark sprite skins it sits close enough to
// the key face that the legend stops reading at a glance.
func TestKeyLegendTakesFullInkAtRest(t *testing.T) {
	for _, family := range themeFamilyChoices() {
		for _, mode := range []string{"light", "dark"} {
			design := newARAMDesignSystem(mode, family)
			key := design.Components.TouchButton
			if !sameColor(key.Text.Idle, design.Palette.Text) {
				t.Fatalf("%s/%s key legend is not the full ink role: %#v",
					family, mode, key.Text.Idle)
			}
			if sameColor(key.Text.Idle, design.Palette.TextMuted) {
				t.Fatalf("%s/%s full ink and muted ink collapsed together", family, mode)
			}
		}
	}
}

// Only the skins whose pressed face fills solid with the accent invert the
// legend; the ones that keep a gloss or gel body under the fill would lose the
// legend entirely against the inverted ink.
func TestHeldKeyLegendInvertsOnlyOnSolidFills(t *testing.T) {
	solid := map[string]bool{"mono-lcd": true, "neon-edge": true}
	for _, family := range retroFamilies() {
		for _, mode := range []string{"light", "dark"} {
			design := newARAMDesignSystem(mode, family)
			pressed := design.Components.TouchButton.Text.Pressed
			want := design.Palette.Text
			if solid[family] {
				want = design.Palette.OnAccent
			}
			if !sameColor(pressed, want) {
				t.Fatalf("%s/%s held legend = %#v, want %#v",
					family, mode, pressed, want)
			}
		}
	}
}

// The key face is drawn from a doubled tile so the bevel stays thick enough to
// read on a control several times the sprite's own size. A doubled tile needs
// twice the border plus twice the center before it can stretch, so a key
// shorter than that would clip its own bevel.
func TestKeyFaceNeverShorterThanItsDoubledTile(t *testing.T) {
	doubled := retroSliceTile * retroKeyScale
	for _, family := range retroFamilies() {
		design := newARAMDesignSystem("dark", family)
		if height := design.Components.TouchButton.MinHeight; height < doubled {
			t.Fatalf("%s key minimum %d is under the doubled tile %d",
				family, height, doubled)
		}
	}
	if touchControlMinSize < doubled {
		t.Fatalf("smallest deck key %d is under the doubled tile %d",
			touchControlMinSize, doubled)
	}
}

// The gel skin's highlight covers the top half of a key, so a pale legend on it
// needs a dark offset copy behind it. No other skin asks for one, and a shadow
// that is not darker than the legend it backs would only thicken the glyph.
func TestOnlyThePaleGelLegendIsShadowed(t *testing.T) {
	for _, family := range themeFamilyChoices() {
		for _, mode := range []string{"light", "dark"} {
			design := newARAMDesignSystem(mode, family)
			shadow, ok := retroKeyLegendShadow(family, design.Palette)
			pale := relativeLuminance(design.Palette.Text) >
				relativeLuminance(design.Palette.Canvas)
			if family != "glass-touch" || !pale {
				if ok {
					t.Fatalf("%s/%s asked for a legend shadow", family, mode)
				}
				continue
			}
			if !ok {
				t.Fatalf("%s/%s dropped its legend shadow", family, mode)
			}
			if relativeLuminance(shadow) >= relativeLuminance(design.Palette.Text) {
				t.Fatalf("%s/%s legend shadow is not darker than the legend", family, mode)
			}
		}
	}
}
