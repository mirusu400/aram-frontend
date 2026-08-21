package frontend

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
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
	retroMu      sync.Mutex
	retroImages  = map[string]*ebiten.Image{}
	retroSlices  = map[string]*euiimage.NineSlice{}
	retroDecoded = map[string]image.Image{}
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

// retroIconGraphic assembles a button icon: the normal ink glyph for idle,
// the inverted (on-accent) glyph for the accent-filled pressed face, and a
// faded copy for disabled controls.
func retroIconGraphic(theme, name string) *widget.GraphicImage {
	idle, err := retroSpriteImage(theme, "icon", name)
	if err != nil {
		panic(err)
	}
	pressed, err := retroSpriteImage(theme, "icon_inv", name)
	if err != nil {
		panic(err)
	}
	return &widget.GraphicImage{
		Idle:     idle,
		Pressed:  pressed,
		Disabled: retroFadedIcon(theme, name, idle),
	}
}

func retroFadedIcon(theme, name string, src *ebiten.Image) *ebiten.Image {
	key := "faded/" + theme + "/" + name
	retroMu.Lock()
	defer retroMu.Unlock()
	if img, ok := retroImages[key]; ok {
		return img
	}
	bounds := src.Bounds()
	faded := ebiten.NewImage(bounds.Dx(), bounds.Dy())
	opts := &ebiten.DrawImageOptions{}
	opts.ColorScale.ScaleAlpha(0.35)
	faded.DrawImage(src, opts)
	retroImages[key] = faded
	return faded
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

// retroScaledNineSlice renders a tile at an integer pixel scale before slicing
// it. The bevel lives entirely in the fixed 8px border, so a control far
// taller than the 17px source shows that bevel as a thin line at each edge
// while the flat center fills everything between. Doubling the tile doubles
// the gloss band and the drop shadow with it, which is what makes a 58px
// keypad key read as a moulded key rather than a flat rectangle.
func retroScaledNineSlice(theme, name string, scale int) *euiimage.NineSlice {
	if scale <= 1 {
		return retroNineSlice(theme, name)
	}
	key := fmt.Sprintf("%s/%s@%dx", theme, name, scale)
	retroMu.Lock()
	if ns, ok := retroSlices[key]; ok {
		retroMu.Unlock()
		return ns
	}
	retroMu.Unlock()
	src, err := retroSpriteImage(theme, "nineslice", name)
	if err != nil {
		panic(err)
	}
	bounds := src.Bounds()
	scaled := ebiten.NewImage(bounds.Dx()*scale, bounds.Dy()*scale)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(float64(scale), float64(scale))
	scaled.DrawImage(src, options)
	ns := euiimage.NewNineSlice(scaled,
		[3]int{retroSliceBorder * scale, retroSliceCenter * scale, retroSliceBorder * scale},
		[3]int{retroSliceBorder * scale, retroSliceCenter * scale, retroSliceBorder * scale})
	retroMu.Lock()
	retroSlices[key] = ns
	retroMu.Unlock()
	return ns
}

// retroTintedIcon recolors a pack glyph, keeping its alpha. The status
// indicators reuse one glyph for several readings — a battery near empty, an
// idle machine — and the pack ships a single ink per icon, so the reading has
// to come from the color.
func retroTintedIcon(theme, name string, tint color.Color) *ebiten.Image {
	red, green, blue, alpha := tint.RGBA()
	key := fmt.Sprintf("tint/%s/%s/%04x%04x%04x%04x", theme, name, red, green, blue, alpha)
	retroMu.Lock()
	if img, ok := retroImages[key]; ok {
		retroMu.Unlock()
		return img
	}
	retroMu.Unlock()
	src, err := retroDecodedSprite(theme, "icon", name)
	if err != nil {
		panic(err)
	}
	bounds := src.Bounds()
	// Repainted per pixel from the decoded PNG rather than through a
	// ColorScale on the GPU image: a scale can only multiply, so it can darken
	// the pack's ink but never lift a dark glyph to a bright reading, and
	// reading pixels back off an ebiten.Image needs a graphics context a
	// headless test does not have. Only the glyph's coverage carries over.
	recolored := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	ink := color.NRGBAModel.Convert(tint).(color.NRGBA)
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			_, _, _, srcAlpha := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if srcAlpha == 0 {
				continue
			}
			recolored.SetNRGBA(x, y, color.NRGBA{
				R: ink.R,
				G: ink.G,
				B: ink.B,
				A: uint8(srcAlpha >> 8),
			})
		}
	}
	out := ebiten.NewImageFromImage(recolored)
	retroMu.Lock()
	retroImages[key] = out
	retroMu.Unlock()
	return out
}

// retroDecodedSprite returns the CPU-side decode of a pack sprite. Anything
// that has to inspect or rewrite pixels reads it here rather than off the
// ebiten.Image, whose pixel readback needs a live graphics context.
func retroDecodedSprite(theme, kind, name string) (image.Image, error) {
	path := fmt.Sprintf("retrothemes/%s/%s/%s.png", theme, kind, name)
	retroMu.Lock()
	if img, ok := retroDecoded[path]; ok {
		retroMu.Unlock()
		return img, nil
	}
	retroMu.Unlock()
	raw, err := retroThemeFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("retro sprite: %w", err)
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("retro sprite %s: %w", path, err)
	}
	retroMu.Lock()
	retroDecoded[path] = src
	retroMu.Unlock()
	return src, nil
}
