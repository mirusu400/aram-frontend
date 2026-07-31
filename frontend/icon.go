package frontend

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
)

//go:embed assets/icon.png
var appIcon []byte

// appIconSizes are the reductions handed to the window manager alongside the
// full size artwork. They are integer divisors of the source so the pixel art
// keeps its hard edges.
var appIconSizes = []int{2, 4, 8}

// appIcons decodes the window icon at a few scales. Window managers pick the
// closest match and scale it themselves, which turns pixel art into mush, so
// the reductions are done here with nearest neighbour sampling instead.
func appIcons() []image.Image {
	source, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		return nil
	}
	icons := []image.Image{source}
	for _, divisor := range appIconSizes {
		if scaled := shrinkNearest(source, divisor); scaled != nil {
			icons = append(icons, scaled)
		}
	}
	return icons
}

func shrinkNearest(source image.Image, divisor int) *image.NRGBA {
	bounds := source.Bounds()
	width, height := bounds.Dx()/divisor, bounds.Dy()/divisor
	if divisor < 1 || width < 1 || height < 1 {
		return nil
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			scaled.Set(x, y, source.At(
				bounds.Min.X+x*divisor,
				bounds.Min.Y+y*divisor,
			))
		}
	}
	return scaled
}
