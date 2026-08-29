package frontend

import (
	"image/color"
	"math"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// modernIconSize is the square canvas every toolbar glyph is drawn on. It
// matches the checkbox artwork so the flat vector strokes share a weight.
const modernIconSize = 24

// actionIcon returns the toolbar glyph for an action, letting one toolbar
// builder serve both skin families: the sprite pack draws the retro icon, and
// the modern family draws a flat vector glyph. nil means the caller should
// fall back to a text button.
func (d *ARAMDesignSystem) actionIcon(name string) *widget.GraphicImage {
	if icon := d.retroIcon(name); icon != nil {
		return icon
	}
	return d.modernToolbarIcon(name)
}

// modernToolbarIcon draws a flat vector glyph for the modern (non-sprite)
// skin so its toolbar reads as icons the way the retro skins do. Graphics
// carry no hover state, so the rest position uses the full ink colour for
// legibility and the sunken accent face marks an active toggle instead.
func (d *ARAMDesignSystem) modernToolbarIcon(name string) *widget.GraphicImage {
	if isRetroFamily(d.Family) {
		return nil
	}
	idle := drawModernIcon(name, d.Palette.Text)
	if idle == nil {
		return nil
	}
	return &widget.GraphicImage{
		Idle:     idle,
		Pressed:  idle,
		Disabled: drawModernIcon(name, d.Palette.TextDisabled),
	}
}

// drawModernIcon rasterises one glyph in a single ink colour. An unknown name
// returns nil so the toolbar keeps its text label for it.
func drawModernIcon(name string, ink color.Color) *ebiten.Image {
	const s = modernIconSize
	const w float32 = 2.1
	img := ebiten.NewImage(s, s)
	switch name {
	case "open":
		// A folder with a raised tab.
		fillIconPath(img, ink, func(p *vector.Path) {
			p.MoveTo(3, 7)
			p.LineTo(9, 7)
			p.LineTo(11, 9)
			p.LineTo(21, 9)
			p.LineTo(21, 19)
			p.LineTo(3, 19)
		})
	case "play":
		fillIconPath(img, ink, func(p *vector.Path) {
			p.MoveTo(8, 6)
			p.LineTo(8, 18)
			p.LineTo(18, 12)
		})
	case "pause":
		vector.DrawFilledRect(img, 8, 6, 3, 12, ink, true)
		vector.DrawFilledRect(img, 13, 6, 3, 12, ink, true)
	case "stop":
		vector.DrawFilledRect(img, 7, 7, 10, 10, ink, true)
	case "reset":
		// A most-of-the-way-round reload arrow: a ring open at the upper right
		// with a solid arrowhead closing the clockwise sweep.
		strokeIconPath(img, ink, w, func(p *vector.Path) {
			p.Arc(12, 12, 6.2, rad(-70), rad(200), vector.Clockwise)
		})
		fillIconPath(img, ink, func(p *vector.Path) {
			p.MoveTo(13.2, 3.4)
			p.LineTo(18.9, 5.9)
			p.LineTo(13.0, 8.4)
		})
	case "settings":
		// A three-track tune glyph: a line per row with a filled handle.
		vector.StrokeLine(img, 4, 8, 20, 8, w, ink, true)
		vector.DrawFilledCircle(img, 9, 8, 2.6, ink, true)
		vector.StrokeLine(img, 4, 13, 20, 13, w, ink, true)
		vector.DrawFilledCircle(img, 15, 13, 2.6, ink, true)
		vector.StrokeLine(img, 4, 18, 20, 18, w, ink, true)
		vector.DrawFilledCircle(img, 11, 18, 2.6, ink, true)
	case "keypad":
		for _, y := range []float32{7, 12, 17} {
			for _, x := range []float32{7, 12, 17} {
				vector.DrawFilledCircle(img, x, y, 1.7, ink, true)
			}
		}
	case "fullscreen":
		// Four outward corner brackets: the expand-to-fill mark.
		vector.StrokeLine(img, 4, 10, 4, 4, w, ink, true)
		vector.StrokeLine(img, 4, 4, 10, 4, w, ink, true)
		vector.StrokeLine(img, 20, 10, 20, 4, w, ink, true)
		vector.StrokeLine(img, 20, 4, 14, 4, w, ink, true)
		vector.StrokeLine(img, 4, 14, 4, 20, w, ink, true)
		vector.StrokeLine(img, 4, 20, 10, 20, w, ink, true)
		vector.StrokeLine(img, 20, 14, 20, 20, w, ink, true)
		vector.StrokeLine(img, 20, 20, 14, 20, w, ink, true)
	default:
		return nil
	}
	return img
}

func fillIconPath(dst *ebiten.Image, ink color.Color, build func(*vector.Path)) {
	var p vector.Path
	build(&p)
	p.Close()
	var op vector.DrawPathOptions
	op.ColorScale.ScaleWithColor(ink)
	op.AntiAlias = true
	vector.FillPath(dst, &p, &vector.FillOptions{}, &op)
}

func strokeIconPath(dst *ebiten.Image, ink color.Color, width float32, build func(*vector.Path)) {
	var p vector.Path
	build(&p)
	var op vector.DrawPathOptions
	op.ColorScale.ScaleWithColor(ink)
	op.AntiAlias = true
	vector.StrokePath(dst, &p, &vector.StrokeOptions{
		Width:    width,
		LineCap:  vector.LineCapRound,
		LineJoin: vector.LineJoinRound,
	}, &op)
}

func rad(degrees float64) float32 {
	return float32(degrees * math.Pi / 180)
}
