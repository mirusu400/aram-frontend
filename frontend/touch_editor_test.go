package frontend

import (
	"image"
	"testing"
)

func editorOptions(ratio int, hidden ...string) touchLayoutOptions {
	options := defaultTouchLayoutOptions()
	options.Keypad = true
	options.DeckRatio = ratio
	if len(hidden) > 0 {
		options.Hidden = make(map[string]bool, len(hidden))
		for _, id := range hidden {
			options.Hidden[id] = true
		}
	}
	return options
}

// TestTouchDeckRatioTradesWithTheGuestScreen is the point of the ratio
// control: the deck and the guest display share one screen, so shrinking the
// deck has to give that height back.
func TestTouchDeckRatioTradesWithTheGuestScreen(t *testing.T) {
	const width, height = 1080, 2340
	small := touchDeckHeightWithOptions(width, height, editorOptions(touchDeckRatioMin))
	large := touchDeckHeightWithOptions(width, height, editorOptions(touchDeckRatioMax))
	if small >= large {
		t.Fatalf("deck at %d%% is %dpx, at %d%% is %dpx; it must grow with the ratio",
			touchDeckRatioMin, small, touchDeckRatioMax, large)
	}
	for _, ratio := range []int{touchDeckRatioMin, 35, 50, touchDeckRatioMax} {
		deck := touchDeckHeightWithOptions(width, height, editorOptions(ratio))
		if deck >= height-statusBarHeight {
			t.Errorf("ratio %d%% leaves no room for the guest screen (%dpx of %dpx)",
				ratio, deck, height)
		}
		if got := touchDeckRatioPercent(width, height, editorOptions(ratio)); got != ratio {
			t.Errorf("ratio %d%% reads back as %d%%", ratio, got)
		}
	}
}

// TestTouchDeckRatioIsClamped keeps a hand-edited settings file from leaving
// the deck unusable or swallowing the screen.
func TestTouchDeckRatioIsClamped(t *testing.T) {
	const width, height = 1080, 2340
	for _, ratio := range []int{1, 5, 95, 400} {
		deck := touchDeckHeightWithOptions(width, height, editorOptions(ratio))
		lowest := touchDeckHeightWithOptions(width, height, editorOptions(touchDeckRatioMin))
		highest := touchDeckHeightWithOptions(width, height, editorOptions(touchDeckRatioMax))
		if deck < lowest || deck > highest {
			t.Errorf("ratio %d%% produced a %dpx deck, outside %d..%dpx",
				ratio, deck, lowest, highest)
		}
	}
	settings := defaultSettings()
	settings.TouchDeckRatio = 400
	settings.normalize()
	if settings.TouchDeckRatio != touchDeckRatioMax {
		t.Errorf("normalize left the ratio at %d%%", settings.TouchDeckRatio)
	}
}

// TestHiddenTouchButtonsLeaveTheDeck covers the put-away half of the feature:
// a hidden button must not draw and must not take a press.
func TestHiddenTouchButtonsLeaveTheDeck(t *testing.T) {
	const width, height = 1080, 2340
	visible := touchControlButtonsWithOptions(width, height, editorOptions(0))
	var target touchButton
	for _, button := range visible {
		if button.ID == "num5" {
			target = button
		}
	}
	if target.ID == "" {
		t.Fatal("num5 is not on the default keypad deck")
	}
	center := target.Bounds.Min.Add(target.Bounds.Size().Div(2))

	hiddenOptions := editorOptions(0, "num5")
	for _, button := range touchControlButtonsWithOptions(width, height, hiddenOptions) {
		if button.ID == "num5" {
			t.Error("a hidden button is still on the deck")
		}
	}
	if _, ok := touchButtonAtWithOptions(center.X, center.Y, width, height, hiddenOptions); ok {
		t.Error("a hidden button still takes a press")
	}
}

// TestHiddenTouchButtonsWaitInTheTray is the other half: the editor has to
// offer a way back, so every hidden button gets a chip inside the tray.
func TestHiddenTouchButtonsWaitInTheTray(t *testing.T) {
	for _, size := range [][2]int{{1080, 2340}, {393, 851}} {
		width, height := size[0], size[1]
		hidden := []string{"num5", "star", "hash", "soft-left"}
		options := editorOptions(0, hidden...)
		tray := touchEditorTrayBounds(width, height, options)
		chips := touchEditorTrayButtons(width, height, options)
		if len(chips) != len(hidden) {
			t.Errorf("%dx%d: tray holds %d chips, want %d",
				width, height, len(chips), len(hidden))
		}
		for _, chip := range chips {
			if !chip.Hidden {
				t.Errorf("%dx%d: tray chip %q is not marked hidden", width, height, chip.ID)
			}
			if !chip.Bounds.In(tray) {
				t.Errorf("%dx%d: chip %q at %v escapes the tray %v",
					width, height, chip.ID, chip.Bounds, tray)
			}
		}
	}
}

// TestTouchEditorControlsDoNotOverlap keeps the actions, steppers, and tray
// from covering each other, which would make one of them unreachable.
func TestTouchEditorControlsDoNotOverlap(t *testing.T) {
	for _, size := range [][2]int{{1080, 2340}, {393, 851}, {720, 1440}} {
		width, height := size[0], size[1]
		options := editorOptions(0, "num5")
		regions := append([]touchButton{}, touchLayoutEditorActions(width)...)
		regions = append(regions, touchLayoutEditorSteppers(width)...)
		for i := range regions {
			for j := i + 1; j < len(regions); j++ {
				if !regions[i].Bounds.Intersect(regions[j].Bounds).Empty() {
					t.Errorf("%dx%d: %q overlaps %q",
						width, height, regions[i].ID, regions[j].ID)
				}
			}
		}
		tray := touchEditorTrayBounds(width, height, options)
		for _, region := range regions {
			if !region.Bounds.Intersect(tray).Empty() {
				t.Errorf("%dx%d: %q overlaps the tray", width, height, region.ID)
			}
		}
		deckTop := height - statusBarHeight -
			touchDeckHeightWithOptions(width, height, options)
		if tray.Max.Y > deckTop {
			t.Errorf("%dx%d: tray reaches %d, past the deck top at %d",
				width, height, tray.Max.Y, deckTop)
		}
	}
}

// TestTouchEditorStepperHitTesting makes sure each stepper answers a press at
// its own center, which is how the ratio and size controls are driven.
func TestTouchEditorStepperHitTesting(t *testing.T) {
	const width = 1080
	for _, want := range touchLayoutEditorSteppers(width) {
		center := want.Bounds.Min.Add(want.Bounds.Size().Div(2))
		got, ok := touchLayoutEditorActionAt(center.X, center.Y, width)
		if !ok || got != want.ID {
			t.Errorf("press at the center of %q resolved to %q (%t)", want.ID, got, ok)
		}
	}
}

// TestTouchEditorTrayIsAValidDropTarget pins the geometry the hide gesture
// relies on: a point inside the tray has to read as inside it.
func TestTouchEditorTrayIsAValidDropTarget(t *testing.T) {
	const width, height = 1080, 2340
	options := editorOptions(0)
	tray := touchEditorTrayBounds(width, height, options)
	if tray.Dx() <= 0 || tray.Dy() <= 0 {
		t.Fatalf("tray is empty: %v", tray)
	}
	center := tray.Min.Add(tray.Size().Div(2))
	if !pointInRect(center.X, center.Y, tray) {
		t.Errorf("the tray's own center %v is not inside %v", center, tray)
	}
	outside := image.Pt(tray.Min.X, tray.Max.Y+40)
	if pointInRect(outside.X, outside.Y, tray) {
		t.Errorf("%v below the tray reads as inside %v", outside, tray)
	}
}

// TestTouchDeckGridStaysOnScreen is the regression guard for a deck ratio that
// could not seat its rows: the grid overflowed and the last keypad row was
// drawn under the status bar, where nothing can press it.
func TestTouchDeckGridStaysOnScreen(t *testing.T) {
	for _, size := range [][2]int{{1080, 2340}, {393, 851}, {720, 1440}, {480, 800}} {
		width, height := size[0], size[1]
		for _, keypad := range []bool{false, true} {
			for _, ratio := range []int{0, touchDeckRatioMin, 30, 38, 50, touchDeckRatioMax} {
				for _, scale := range []int{touchControlScaleMin, 100, touchControlScaleMax} {
					options := defaultTouchLayoutOptions()
					options.Keypad = keypad
					options.DeckRatio = ratio
					options.Scale = touchScaleFactor(scale)
					limit := height - statusBarHeight
					for _, button := range touchControlButtonsWithOptions(width, height, options) {
						if button.Bounds.Max.Y <= limit && button.Bounds.Min.Y >= 0 &&
							button.Bounds.Min.X >= 0 && button.Bounds.Max.X <= width {
							continue
						}
						t.Errorf(
							"%dx%d keypad=%t ratio=%d scale=%d: %q at %v leaves the %dx%d play area",
							width, height, keypad, ratio, scale,
							button.ID, button.Bounds, width, limit)
					}
				}
			}
		}
	}
}

// TestTouchDeckHeightSeatsItsRows states the invariant behind that guard.
func TestTouchDeckHeightSeatsItsRows(t *testing.T) {
	for _, size := range [][2]int{{1080, 2340}, {393, 851}, {480, 800}} {
		width, height := size[0], size[1]
		for _, keypad := range []bool{false, true} {
			options := defaultTouchLayoutOptions()
			options.Keypad = keypad
			options.DeckRatio = touchDeckRatioMin
			deck := touchDeckHeightWithOptions(width, height, options)
			if want := minimumTouchDeckHeight(options); deck < want {
				t.Errorf("%dx%d keypad=%t: deck is %dpx, needs %dpx for %d rows",
					width, height, keypad, deck, want, touchDeckRowCount(options))
			}
		}
	}
}
