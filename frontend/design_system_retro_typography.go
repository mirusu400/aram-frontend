package frontend

import (
	"bytes"
	_ "embed"
	"sync"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Terrarum Sans Bitmap is a pixel font by CuriousTorvald covering Latin,
// Cyrillic, kana, and all 11,172 Hangul syllables, licensed under the SIL
// Open Font License 1.1 — see assets/terrarum/LICENSE.md. The OTF build
// renders 1:1 on the pixel grid at size 20 (16px ascent + 4px descent), so
// sprite skins swap the whole type ramp onto it at 20px, with 40px for the
// doubled heading and display sizes. The face ships a single weight, which
// fits the era: handset shells drew everything with one bitmap font.
//
//go:embed assets/terrarum/TerrarumSansBitmap.otf
var terrarumSansOTF []byte

// retroFallbackScale keeps the Noto fallback's line box from exceeding the
// pixel font's own. EbitenUI centers text by its line box and then puts the
// baseline one ascent below the top, and a MultiFace reports the largest
// ascent of its members — so a fallback with a taller ascent silently pushes
// every pixel glyph down inside its widget. Noto's ascent is a full em
// against Terrarum's 0.8, so the fallback runs at 80% of the nominal size and
// the combined metrics stay exactly Terrarum's.
const retroFallbackScale = 0.8

// retroCenterNudge lifts text by one pixel. Terrarum's declared descent
// (0.2em) is deeper than any glyph actually reaches, which leaves its ink
// sitting a pixel below the middle of the line box at the 20px body size.
const retroCenterNudge = 1

// retroTextPadding applies the nudge to EbitenUI text widgets. The negative
// top is paired with an equal bottom so the widget's preferred size, which
// sums both, is unchanged.
func retroTextPadding() *widget.Insets {
	return &widget.Insets{Top: -retroCenterNudge, Bottom: retroCenterNudge}
}

var (
	retroTypeOnce sync.Once
	retroType     ARAMTypography
)

// retroTypography lazily builds the shared pixel faces. The embedded Noto
// face stays as a fallback for the few symbols the pixel font lacks.
func retroTypography() ARAMTypography {
	retroTypeOnce.Do(func() {
		pixel, err := text.NewGoTextFaceSource(bytes.NewReader(terrarumSansOTF))
		if err != nil {
			panic("load embedded Terrarum Sans font: " + err.Error())
		}
		fallback, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansKR))
		if err != nil {
			panic("load embedded ARAM Korean font: " + err.Error())
		}
		retroType = ARAMTypography{
			Caption:     pixelTextFace(pixel, fallback, 20),
			Body:        pixelTextFace(pixel, fallback, 20),
			Strong:      pixelTextFace(pixel, fallback, 20),
			Heading:     pixelTextFace(pixel, fallback, 40),
			Display:     pixelTextFace(pixel, fallback, 40),
			CenterNudge: retroCenterNudge,
		}
	})
	return retroType
}

// pixelTextFace pairs the pixel font with a metric-safe fallback so the
// combined line box stays the pixel font's own.
func pixelTextFace(
	pixel *text.GoTextFaceSource,
	fallback *text.GoTextFaceSource,
	size float64,
) *text.Face {
	primary := &text.GoTextFace{Source: pixel, Size: size}
	secondary := &text.GoTextFace{Source: fallback, Size: size * retroFallbackScale}
	multi, err := text.NewMultiFace(primary, secondary)
	if err != nil {
		panic("create ARAM pixel font fallback: " + err.Error())
	}
	var face text.Face = multi
	return &face
}
