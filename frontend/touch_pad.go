package frontend

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	// circularPadDeadzoneRatio is the share of the pad radius a thumb has to
	// travel from center before any direction fires. Inside it the pad is at
	// rest, which is also what lets a still tap read as OK rather than a nudge.
	circularPadDeadzoneRatio = 0.28
	// circularPadKnobRatio sizes the movable knob against the pad radius.
	circularPadKnobRatio = 0.44
	// circularPadOKPulseFrames is how long a center tap holds OK so the guest
	// polls a clean press even though the finger has already lifted.
	circularPadOKPulseFrames = 6
)

// isCircularPadSlotID reports whether a deck slot belongs to the directional
// cluster the round pad replaces. The center OK ("ok") folds into the pad as a
// tap; the action cluster's own OK ("ok-action") is a separate button and
// stays.
func isCircularPadSlotID(id string) bool {
	switch id {
	case "up", "down", "left", "right", "ok":
		return true
	}
	return false
}

// circularPadCircle is the pad's center and radius, inscribed in the 3x3
// directional cluster so the round pad covers exactly the cross's footprint.
func circularPadCircle(metrics touchDeckMetrics) (image.Point, int) {
	center := image.Pt(
		metrics.dpadX+metrics.buttonSize/2,
		metrics.dpadY+metrics.buttonSize/2,
	)
	radius := (metrics.buttonSize*3 + metrics.gap*2) / 2
	return center, radius
}

// resolvePadDirection4 snaps a thumb vector to the nearest of the four
// directions, never two at once: the dominant axis wins, ties go horizontal.
func resolvePadDirection4(dx, dy float64) string {
	if math.Abs(dx) >= math.Abs(dy) {
		if dx < 0 {
			return "left"
		}
		return "right"
	}
	if dy < 0 {
		return "up"
	}
	return "down"
}

// drawCircularPad renders the round thumb pad in place of the directional
// cross: a dished well, four cardinal hints, and a knob that follows the live
// thumb vector and lights while steering or pulsing OK.
func (s *Shell) drawCircularPad(
	screen *ebiten.Image,
	width, height int,
	options touchLayoutOptions,
) {
	metrics := touchDeckMetricsFor(width, height, options)
	center, radius := circularPadCircle(metrics)
	if radius <= 0 {
		return
	}
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	cx, cy, r := float32(center.X), float32(center.Y), float32(radius)
	// An outer ring under an inset face reads as a dish without a stroke.
	vector.DrawFilledCircle(screen, cx, cy, r, palette.BorderStrong, true)
	vector.DrawFilledCircle(screen, cx, cy, r-2, palette.SurfaceRaised, true)
	glyph := max(touchControlMinSize/2, metrics.buttonSize/2)
	off := int(float64(radius) * 0.6)
	for _, hint := range []struct {
		name   string
		dx, dy int
	}{
		{"up", 0, -off}, {"down", 0, off},
		{"left", -off, 0}, {"right", off, 0},
	} {
		ink := color.Color(palette.TextMuted)
		if s.padDir == hint.name {
			ink = palette.Accent
		}
		drawDirectionGlyph(
			screen,
			rectAt(center.X+hint.dx-glyph/2, center.Y+hint.dy-glyph/2, glyph, glyph),
			hint.name,
			ink,
		)
	}
	knobR := float32(float64(radius) * circularPadKnobRatio)
	knobX, knobY := cx+float32(s.padKnob.X), cy+float32(s.padKnob.Y)
	engaged := s.padDir != "" || s.padOKPulse > 0
	ring, fill := palette.Border, palette.Surface
	if engaged {
		ring, fill = palette.Accent, palette.Accent
	}
	vector.DrawFilledCircle(screen, knobX, knobY, knobR, ring, true)
	vector.DrawFilledCircle(screen, knobX, knobY, knobR-2, fill, true)
	// OK on the knob makes tap-to-confirm discoverable.
	if s.design != nil {
		ink := color.Color(palette.TextMuted)
		if engaged {
			ink = palette.OnAccent
		}
		knobBounds := rectAt(
			int(knobX-knobR), int(knobY-knobR),
			int(knobR*2), int(knobR*2),
		)
		textFace := s.design.Type.Caption
		top := centeredTextTop(textFace, knobBounds, s.design.Type.CenterNudge)
		drawCenteredText(screen, s.tr("OK"), textFace, ink, knobBounds, top)
	}
}
