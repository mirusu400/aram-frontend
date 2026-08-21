package frontend

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// directionGlyphFor maps a control to the arrow a key carries instead of a
// word. A direction key is the one control a player finds without reading, and
// a localized word ("위", "왼쪽") is both slower to parse mid-game and unable to
// fit a key at handset sizes; the pack's own 5-way pad draws four triangles.
// An empty string means the control keeps its text label.
func directionGlyphFor(control string) string {
	switch control {
	case "up", "down", "left", "right":
		return control
	}
	return ""
}

// directionGlyphRows is how many stepped rows a triangle is built from. Each
// row is two pixels wider than the one before, which is the same 2:1 spread
// the pack's pad arrows use.
const directionGlyphRows = 4

// drawDirectionGlyph fills bounds with a stepped pixel triangle pointing the
// named way. The triangle is rasterized from whole-pixel rows rather than a
// scaled sprite so it stays on the pixel grid at any key size, which is what
// keeps it beside a pixel font without looking resampled.
func drawDirectionGlyph(
	screen *ebiten.Image,
	bounds image.Rectangle,
	direction string,
	ink color.Color,
) {
	spanUnit, depthUnit := directionGlyphUnits(bounds)
	if spanUnit <= 0 || depthUnit <= 0 {
		return
	}
	short := directionGlyphRows * depthUnit
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2
	for row := 0; row < directionGlyphRows; row++ {
		span := (row*2 + 1) * spanUnit
		// depth walks from the apex toward the base, so row 0 is the point.
		depth := row * depthUnit
		unit := depthUnit
		var x, y, width, height int
		switch direction {
		case "up":
			x, y = centerX-span/2, centerY-short/2+depth
			width, height = span, unit
		case "down":
			x, y = centerX-span/2, centerY+short/2-depth-unit
			width, height = span, unit
		case "left":
			x, y = centerX-short/2+depth, centerY-span/2
			width, height = unit, span
		case "right":
			x, y = centerX+short/2-depth-unit, centerY-span/2
			width, height = unit, span
		default:
			return
		}
		ebitenutil.DrawRect(
			screen,
			float64(x), float64(y),
			float64(width), float64(height),
			ink,
		)
	}
}

// directionGlyphUnits sizes one step of the triangle along its base and along
// its depth. The two differ because stepping the base by two units per row
// would otherwise give a triangle almost twice as wide as it is deep; a
// slightly taller depth step lands near the 4:3 arrow the pack's own pad
// draws. The glyph spans about three fifths of the key so it stays clear of
// the bevel, and floors at one pixel per step so the smallest key the deck
// draws still gets an arrow.
func directionGlyphUnits(bounds image.Rectangle) (span, depth int) {
	shortest := min(bounds.Dx(), bounds.Dy())
	if shortest <= 0 {
		return 0, 0
	}
	// The base is (2n-1) span units across, so it is the binding dimension.
	span = max(1, shortest*3/5/(directionGlyphRows*2-1))
	// Rounded up from span*5/4 so a one-pixel span still deepens the arrow.
	depth = max(1, (span*5+2)/4)
	return span, depth
}
