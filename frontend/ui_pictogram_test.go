package frontend

import (
	"image"
	"testing"
)

// A direction key has to be readable without reading. The deck used to spell
// the direction out, which localizes to "왼쪽"/"오른쪽" — words too long for a
// key and too slow to parse mid-game.
func TestDirectionKeysCarryAGlyphAndOtherKeysKeepTheirLabel(t *testing.T) {
	for _, control := range []string{"up", "down", "left", "right"} {
		if directionGlyphFor(control) == "" {
			t.Fatalf("control %q draws no direction glyph", control)
		}
	}
	for _, control := range []string{"ok", "menu", "back", "soft-left", "num5", "star"} {
		if glyph := directionGlyphFor(control); glyph != "" {
			t.Fatalf("control %q took the direction glyph %q", control, glyph)
		}
	}
}

// The glyph is rasterized, not a scaled sprite, so its size has to be derived
// from the key. It must clear the key's bevel at every size the deck draws and
// never vanish on the smallest one.
func TestDirectionGlyphFitsEveryKeySize(t *testing.T) {
	for size := touchControlMinSize; size <= touchControlMaxSize; size++ {
		bounds := image.Rect(0, 0, size, size)
		span, depth := directionGlyphUnits(bounds)
		if span < 1 || depth < 1 {
			t.Fatalf("size %d produced no glyph: span %d depth %d", size, span, depth)
		}
		base := (directionGlyphRows*2 - 1) * span
		reach := directionGlyphRows * depth
		if base > size || reach > size {
			t.Fatalf("size %d glyph overflows: base %d reach %d", size, base, reach)
		}
		// An arrow that fills less than a quarter of its key reads as a speck.
		if base*4 < size {
			t.Fatalf("size %d glyph too small: base %d", size, base)
		}
		// Deeper than it is wide would no longer read as a direction arrow,
		// and much wider than it is deep reads as a bar rather than a point.
		if reach > base || base*2 > reach*3 {
			t.Fatalf("size %d glyph mis-shaped: base %d reach %d", size, base, reach)
		}
	}
}
