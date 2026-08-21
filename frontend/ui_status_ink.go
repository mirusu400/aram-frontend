package frontend

import "image/color"

// statusBarInkLift is how far a quiet status-bar ink is pulled toward the
// palette's full text ink in a dark theme.
const statusBarInkLift = 0.75

// paletteIsDark reports whether a palette paints light ink on a dark ground.
// It ranks the palette itself rather than reading the mode name, so a sprite
// skin that ships its own dark variant is covered as well.
func paletteIsDark(palette ARAMPalette) bool {
	return relativeLuminance(palette.Text) > relativeLuminance(palette.Canvas)
}

// statusBarInk returns the ink a status-bar element draws with.
//
// The bar carries the readings that have nowhere else to live - machine state,
// the achieved speed, the scaling filter, and the handset indicators - and
// every dark palette sets its muted and disabled inks close to the bar's own
// surface. On a dark ground that reads as unlit rather than quiet, which made
// the status line the hardest text in the shell to read. Dark themes therefore
// lift those inks toward the palette's full text ink; a light theme already
// carries the contrast and is left as designed.
func statusBarInk(palette ARAMPalette, role color.NRGBA) color.NRGBA {
	if !paletteIsDark(palette) {
		return role
	}
	return mixNRGBA(role, palette.Text, statusBarInkLift)
}
