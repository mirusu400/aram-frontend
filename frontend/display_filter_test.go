package frontend

import (
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFeaturePhoneDisplayShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(featurePhoneDisplayShaderSource)); err != nil {
		t.Fatalf("compile feature-phone display shader: %v", err)
	}
}

func TestDisplayPresetShadersCompile(t *testing.T) {
	for name, source := range map[string]string{
		"sharp bilinear":    sharpBilinearShaderSource,
		"LCD persistence":   temporalBlendShaderSource,
		"smooth pixel 2x":   smoothPixelShaderSource,
		"feature phone STN": featurePhoneSTNShaderSource,
		"CRT TV":            crtTVShaderSource,
	} {
		if _, err := ebiten.NewShader([]byte(source)); err != nil {
			t.Fatalf("compile %s shader: %v", name, err)
		}
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

func TestSharpBilinearScaleFollowsSourceAxesAfterRotation(t *testing.T) {
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
		x, y := sharpBilinearScale(test.destination, 240, 320, test.rotation)
		if x != test.wantX || y != test.wantY {
			t.Errorf("rotation %d sharp scale = (%g, %g), want (%g, %g)",
				test.rotation, x, y, test.wantX, test.wantY)
		}
	}
}

func TestDisplayQuadMapsClockwiseRotationToSourceCorners(t *testing.T) {
	source := image.Rect(10, 20, 250, 340)
	want := map[int][4][2]float32{
		0:   {{10, 20}, {250, 20}, {10, 340}, {250, 340}},
		90:  {{10, 340}, {10, 20}, {250, 340}, {250, 20}},
		180: {{250, 340}, {10, 340}, {250, 20}, {10, 20}},
		270: {{250, 20}, {250, 340}, {10, 20}, {10, 340}},
	}
	for rotation, corners := range want {
		vertices := displayQuadVertices(image.Rect(0, 0, 960, 720), source, rotation)
		for index, corner := range corners {
			if vertices[index].SrcX != corner[0] || vertices[index].SrcY != corner[1] {
				t.Errorf("rotation %d vertex %d source = (%g, %g), want (%g, %g)",
					rotation, index, vertices[index].SrcX, vertices[index].SrcY, corner[0], corner[1])
			}
		}
	}
}

func TestTFTPersistenceUsesDisplayTimeAndDecays(t *testing.T) {
	halfLife := 22 * time.Millisecond
	if got := displayPersistenceWeight(displayEffectFeaturePhoneTFT, halfLife); math.Abs(float64(got)-0.5) > 0.001 {
		t.Fatalf("one TFT half-life weight = %g, want 0.5", got)
	}
	if first, second := displayPersistenceWeight(displayEffectFeaturePhoneTFT, halfLife), displayPersistenceWeight(displayEffectFeaturePhoneTFT, 2*halfLife); second >= first {
		t.Fatalf("TFT persistence did not decay: first=%g second=%g", first, second)
	}
}

func TestSTNPersistenceIsSlowerThanTFT(t *testing.T) {
	stnHalfLife := 75 * time.Millisecond
	if got := displayPersistenceWeight(displayEffectFeaturePhoneSTN, stnHalfLife); math.Abs(float64(got)-0.5) > 0.001 {
		t.Fatalf("one STN half-life weight = %g, want 0.5", got)
	}
	frameTime := time.Second / 60
	tft := displayPersistenceWeight(displayEffectFeaturePhoneTFT, frameTime)
	stn := displayPersistenceWeight(displayEffectFeaturePhoneSTN, frameTime)
	if stn <= tft || stn < 0.8 {
		t.Fatalf("STN response is not visibly slower: TFT=%g STN=%g", tft, stn)
	}
	if got := displayPersistenceWeight(displayEffectSmoothPixel, frameTime); got != 0 {
		t.Fatalf("non-panel persistence = %g, want 0", got)
	}
}

func TestSmoothPixelBuildsAndCachesNative2xSurface(t *testing.T) {
	shell := &Shell{
		frame:      VideoFrame{Sequence: 4, Generation: 2},
		frameImage: ebiten.NewImage(24, 32),
	}
	first, err := shell.smoothPixel2xImage()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := first.Bounds(), image.Rect(0, 0, 48, 64); got != want {
		t.Fatalf("smooth pixel surface = %v, want %v", got, want)
	}
	second, err := shell.smoothPixel2xImage()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("unchanged guest sequence rebuilt the smooth pixel surface")
	}
	shell.frame.Sequence++
	if _, err := shell.smoothPixel2xImage(); err != nil {
		t.Fatal(err)
	}
	if shell.displayScaleSequence != shell.frame.Sequence {
		t.Fatalf("cached smooth sequence = %d, want %d", shell.displayScaleSequence, shell.frame.Sequence)
	}
}

func TestSmoothPixelCoordinatesAreAtlasIndependent(t *testing.T) {
	if !strings.Contains(
		smoothPixelShaderSource,
		"sourceLocal := src0Pos - imageSrc0Origin()",
	) || strings.Contains(smoothPixelShaderSource, "floor(dstPos.xy)") {
		t.Fatal("smooth pixel quadrant selection must use source space, not atlas-relative destination space")
	}
}

func TestDisplayPersistenceAdvancesWithDisplayTime(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	clock := time.Unix(100, 0)
	shell.nowFunc = func() time.Time { return clock }
	shell.frame = VideoFrame{Sequence: 1, GuestNS: 10_000_000, Generation: 7}
	current := ebiten.NewImage(24, 32)

	first := shell.updateDisplayPersistence(current)
	if !shell.displayHistoryValid || shell.displayHistorySequence != 1 {
		t.Fatalf("initial display history = valid:%t sequence:%d",
			shell.displayHistoryValid, shell.displayHistorySequence)
	}
	if same := shell.updateDisplayPersistence(current); same != first {
		t.Fatal("LCD persistence advanced without display time passing")
	}

	clock = clock.Add(time.Second / 60)
	settled := shell.updateDisplayPersistence(current)
	if settled == first {
		t.Fatal("a static guest frame left its old LCD response frozen")
	}

	shell.frame.Sequence = 2
	clock = clock.Add(time.Second / 60)
	second := shell.updateDisplayPersistence(current)
	if second == settled || shell.displayHistorySequence != 2 {
		t.Fatalf("new guest frame did not ping-pong persistence: settled=%p second=%p sequence=%d",
			settled, second, shell.displayHistorySequence)
	}

	shell.releaseDisplaySurfaces()
	if shell.displayEffectImage != nil || shell.displayScaleImage != nil ||
		shell.displayHistoryImage != nil || shell.displayResponseImage != nil ||
		shell.displayScaleValid || shell.displayHistoryValid {
		t.Fatal("display surfaces survived title release")
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
