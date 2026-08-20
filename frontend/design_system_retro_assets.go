package frontend

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"sync"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

// The retro sprite pack (CC0, original work — see retrothemes/) replaces the
// flat-color nine-slices of the modern design system with feature-phone era
// chrome. Every tile is 17×17 with an 8px fixed border and a 1px stretchable
// center, so gradients never band no matter how far a control is stretched.
//
//go:embed retrothemes
var retroThemeFS embed.FS

const (
	retroSliceBorder = 8
	retroSliceCenter = 1
	retroSliceTile   = retroSliceBorder*2 + retroSliceCenter
)

var (
	retroMu     sync.Mutex
	retroImages = map[string]*ebiten.Image{}
	retroSlices = map[string]*euiimage.NineSlice{}
)

func retroSpriteImage(theme, kind, name string) (*ebiten.Image, error) {
	path := fmt.Sprintf("retrothemes/%s/%s/%s.png", theme, kind, name)
	retroMu.Lock()
	defer retroMu.Unlock()
	if img, ok := retroImages[path]; ok {
		return img, nil
	}
	raw, err := retroThemeFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("retro sprite: %w", err)
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("retro sprite %s: %w", path, err)
	}
	img := ebiten.NewImageFromImage(src)
	retroImages[path] = img
	return img, nil
}

// retroNineSlice panics on a missing tile: the pack is embedded, so a missing
// name is a programming error caught by the asset completeness test.
func retroNineSlice(theme, name string) *euiimage.NineSlice {
	key := theme + "/" + name
	retroMu.Lock()
	if ns, ok := retroSlices[key]; ok {
		retroMu.Unlock()
		return ns
	}
	retroMu.Unlock()
	img, err := retroSpriteImage(theme, "nineslice", name)
	if err != nil {
		panic(err)
	}
	ns := euiimage.NewNineSlice(img,
		[3]int{retroSliceBorder, retroSliceCenter, retroSliceBorder},
		[3]int{retroSliceBorder, retroSliceCenter, retroSliceBorder})
	retroMu.Lock()
	retroSlices[key] = ns
	retroMu.Unlock()
	return ns
}

// retroButtonImage assembles the EbitenUI button states from a base name.
// Shipped bases: "button" and "button_primary"; the primary variant reuses the
// neutral disabled face so every control greys out alike.
func retroButtonImage(theme, base string) *widget.ButtonImage {
	disabledName := base + "_disabled"
	if base != "button" {
		disabledName = "button_disabled"
	}
	pressed := retroNineSlice(theme, base+"_pressed")
	return &widget.ButtonImage{
		Idle:         retroNineSlice(theme, base+"_idle"),
		Hover:        retroNineSlice(theme, base+"_hover"),
		Pressed:      pressed,
		PressedHover: pressed,
		Disabled:     retroNineSlice(theme, disabledName),
	}
}
