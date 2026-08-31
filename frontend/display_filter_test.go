package frontend

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFeaturePhoneDisplayShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(featurePhoneDisplayShaderSource)); err != nil {
		t.Fatalf("compile feature-phone display shader: %v", err)
	}
}

func TestDisplayPixelPitchFollowsRotatedGuestPixels(t *testing.T) {
	for _, test := range []struct {
		rotation    int
		destination image.Rectangle
		wantX       float32
		wantY       float32
	}{
		{0, image.Rect(0, 0, 720, 960), 3, 3},
		{90, image.Rect(0, 0, 960, 720), 3, 3},
		{270, image.Rect(0, 0, 480, 360), 1.5, 1.5},
	} {
		x, y := displayPixelPitch(test.destination, 240, 320, test.rotation)
		if x != test.wantX || y != test.wantY {
			t.Errorf("rotation %d pitch = (%g, %g), want (%g, %g)",
				test.rotation, x, y, test.wantX, test.wantY)
		}
	}
}

func TestDisplayEffectSurfaceIsReusedUntilItsSizeChanges(t *testing.T) {
	shell := &Shell{}
	first := shell.ensureDisplayEffectImage(240, 320)
	if second := shell.ensureDisplayEffectImage(240, 320); second != first {
		t.Fatal("same-sized display effect rebuilt its render surface")
	}
	if resized := shell.ensureDisplayEffectImage(320, 240); resized == first {
		t.Fatal("resized display effect retained the old render surface")
	}
}
