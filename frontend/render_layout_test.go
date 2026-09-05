package frontend

import (
	"image"
	"testing"
)

func TestFrameDestinationLayoutScaling(t *testing.T) {
	viewport := image.Rect(20, 30, 670, 680)
	for _, test := range []struct {
		name           string
		layout         string
		preserveAspect bool
		integerScaling bool
		immersive      bool
		width, height  int
		want           image.Rectangle
	}{
		{"center integer", "center", true, true, false, 240, 320, image.Rect(105, 35, 585, 675)},
		{"center fractional", "center", true, false, false, 240, 320, image.Rect(101, 30, 589, 680)},
		{"center without aspect", "center", false, true, false, 240, 320, image.Rect(105, 35, 585, 675)},
		{"fill aspect integer", "stretch", true, true, false, 240, 320, image.Rect(101, 30, 589, 680)},
		{"fill aspect fractional", "stretch", true, false, false, 240, 320, image.Rect(101, 30, 589, 680)},
		{"fill aspect rotated", "stretch", true, true, false, 320, 240, image.Rect(20, 111, 670, 599)},
		{"fill aspect downscale", "stretch", true, true, false, 960, 1280, image.Rect(101, 30, 589, 680)},
		{"fill without aspect integer", "stretch", false, true, false, 240, 320, viewport},
		{"fill without aspect fractional", "stretch", false, false, false, 240, 320, viewport},
		{"immersive center", "center", true, true, true, 240, 320, image.Rect(101, 30, 589, 680)},
		{"immersive fill", "stretch", true, true, true, 240, 320, image.Rect(101, 30, 589, 680)},
	} {
		t.Run(test.name, func(t *testing.T) {
			shell := &Shell{settings: defaultSettings(), fillGuestViewport: test.immersive}
			shell.settings.ScreenLayout = test.layout
			shell.settings.PreserveAspect = test.preserveAspect
			shell.settings.IntegerScaling = test.integerScaling
			if got := shell.frameDestination(viewport, test.width, test.height); got != test.want {
				t.Fatalf("destination = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFrameDestinationUsesTitleFillProfile(t *testing.T) {
	shell := &Shell{
		settings: defaultSettings(),
		input:    &InputInfo{SHA256: "synthetic-layout-title"},
	}
	profile := shell.settings.globalDisplayProfile()
	profile.ScreenLayout = "stretch"
	shell.settings.TitleDisplays = map[string]DisplayProfile{titleSettingsKey(shell.input): profile}
	viewport := image.Rect(0, 0, 392, 520)
	want := image.Rect(1, 0, 391, 520)
	if got := shell.frameDestination(viewport, 240, 320); got != want {
		t.Fatalf("title fill destination = %v, want %v", got, want)
	}
	if shell.settings.ScreenLayout != "center" || !shell.settings.IntegerScaling {
		t.Fatal("title fill changed the global display defaults")
	}
}
