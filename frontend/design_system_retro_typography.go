package frontend

import (
	"bytes"
	_ "embed"
	"sync"

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
			Caption: goTextFace(pixel, fallback, 20, 400),
			Body:    goTextFace(pixel, fallback, 20, 400),
			Strong:  goTextFace(pixel, fallback, 20, 700),
			Heading: goTextFace(pixel, fallback, 40, 700),
			Display: goTextFace(pixel, fallback, 40, 700),
		}
	})
	return retroType
}
