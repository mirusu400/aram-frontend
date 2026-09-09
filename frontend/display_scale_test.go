package frontend

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestScaledScreenSizePreservesFractionalDensity(t *testing.T) {
	width, height := scaledScreenSize(390, 844, 2.75)
	if width != 1073 || height != 2321 {
		t.Fatalf("scaled screen = %dx%d, want 1073x2321", width, height)
	}
}

func TestShellHiDPILayoutKeepsOutsideSizeSeparate(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := NewShell(NullBackend{}, nil, "")

	width, height := shell.layoutAtScale(390, 844, 3)
	if width != 1170 || height != 2532 {
		t.Fatalf("Layout = %dx%d, want 1170x2532", width, height)
	}
	if outsideWidth, outsideHeight := shell.outsideSize(); outsideWidth != 390 || outsideHeight != 844 {
		t.Fatalf("outside = %dx%d, want 390x844", outsideWidth, outsideHeight)
	}
	if viewportWidth, viewportHeight := shell.viewportSize(); viewportWidth != width || viewportHeight != height {
		t.Fatalf("viewport = %dx%d, want render size %dx%d", viewportWidth, viewportHeight, width, height)
	}

	shell.syncDesignSystem()
	if shell.design.Scale != 3 {
		t.Fatalf("design scale = %g, want 3", shell.design.Scale)
	}
	shell.interfaceUI.sync(shell)
	if !shell.interfaceUI.compact {
		t.Fatal("390 DIP wide HiDPI viewport was not treated as compact")
	}
	// Exercise the complete custom-draw and EbitenUI paths at the physical
	// target size. This catches any remaining DIP-sized fixed images or window
	// geometry that cannot be laid out on the scaled surface.
	shell.Draw(ebiten.NewImage(width, height))
}

func TestScaledDesignRasterizesTypeAndIconsAtRenderDensity(t *testing.T) {
	base := newScaledARAMDesignSystem("light", themeFamilyModern, 1)
	high := newScaledARAMDesignSystem("light", themeFamilyModern, 3)

	baseWidth, baseHeight := text.Measure("ARAM 한글", *base.Type.Body, 0)
	highWidth, highHeight := text.Measure("ARAM 한글", *high.Type.Body, 0)
	if highWidth < baseWidth*2.8 || highHeight < baseHeight*2.8 {
		t.Fatalf(
			"HiDPI text = %.1fx%.1f, base = %.1fx%.1f; glyphs were not rasterized near 3x",
			highWidth, highHeight, baseWidth, baseHeight,
		)
	}
	if high.Space.L != 3*base.Space.L {
		t.Fatalf("large spacing = %d, want %d", high.Space.L, 3*base.Space.L)
	}
	icon := high.modernToolbarIcon("settings")
	if icon == nil || icon.Idle.Bounds().Dx() != 3*modernIconSize || icon.Idle.Bounds().Dy() != 3*modernIconSize {
		t.Fatalf("HiDPI toolbar icon does not use a %dx%d backing image", 3*modernIconSize, 3*modernIconSize)
	}
}

func TestScaledRetroDesignKeepsPixelAssetsOnAnIntegerGrid(t *testing.T) {
	design := newScaledARAMDesignSystem("dark", "chrome-blue", 3)
	if design.Type.CenterNudge != 3*retroCenterNudge {
		t.Fatalf("retro center nudge = %d, want %d", design.Type.CenterNudge, 3*retroCenterNudge)
	}
	icon := design.retroIcon("settings")
	if icon == nil || icon.Idle.Bounds().Dx() != 48 || icon.Idle.Bounds().Dy() != 48 {
		t.Fatal("3x retro icon was not nearest-neighbor rasterized to 48x48")
	}
}

func TestHiDPITouchLayoutKeepsItsDIPFootprint(t *testing.T) {
	const scale = 3.0
	base := touchDeckMetricsForScale(390, 844, defaultTouchLayoutOptions(), 1)
	high := touchDeckMetricsForScale(390*3, 844*3, defaultTouchLayoutOptions(), scale)
	for name, pair := range map[string][2]int{
		"button": {base.buttonSize, high.buttonSize},
		"gap":    {base.gap, high.gap},
		"dpad x": {base.dpadX, high.dpadX},
		"dpad y": {base.dpadY, high.dpadY},
	} {
		if math.Abs(float64(pair[1]-pair[0]*3)) > 2 {
			t.Errorf("%s = %d at 3x, want about %d", name, pair[1], pair[0]*3)
		}
	}
}
