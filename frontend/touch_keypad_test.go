package frontend

import (
	"image"
	"testing"
)

// numericTouchControls is what a handset title reaches for beyond the
// direction and action clusters.
var numericTouchControls = []string{
	"num0", "num1", "num2", "num3", "num4", "num5",
	"num6", "num7", "num8", "num9", "star", "hash",
}

func keypadOptions() touchLayoutOptions {
	options := defaultTouchLayoutOptions()
	options.Keypad = true
	return options
}

// TestTouchDeckOmitsKeypadUntilEnabled keeps the default deck as it was.
func TestTouchDeckOmitsKeypadUntilEnabled(t *testing.T) {
	buttons := touchControlButtonsFor(1080, 2280)
	for _, button := range buttons {
		for _, control := range numericTouchControls {
			if button.ID == control {
				t.Fatalf("default deck carries %q", control)
			}
		}
	}
}

// TestTouchDeckKeypadCoversEveryNumericControl is the point of the feature: a
// touch layout has no keyboard, so without these the digits are unreachable.
func TestTouchDeckKeypadCoversEveryNumericControl(t *testing.T) {
	for _, size := range [][2]int{{1080, 2280}, {720, 1440}, {393, 851}, {2280, 1080}} {
		width, height := size[0], size[1]
		buttons := touchControlButtonsWithOptions(width, height, keypadOptions())
		found := make(map[string]touchButton, len(buttons))
		for _, button := range buttons {
			found[button.ID] = button
		}
		screen := image.Rect(0, 0, width, height-statusBarHeight)
		for _, control := range numericTouchControls {
			button, ok := found[control]
			if !ok {
				t.Errorf("%dx%d: keypad is missing %q", width, height, control)
				continue
			}
			if button.Control != control {
				t.Errorf("%dx%d: %q drives control %q", width, height, control, button.Control)
			}
			if !button.Bounds.In(screen) {
				t.Errorf("%dx%d: %q sits at %v, outside %v",
					width, height, control, button.Bounds, screen)
			}
			if button.Bounds.Dx() < touchControlMinSize {
				t.Errorf("%dx%d: %q is %dpx wide, below the %dpx minimum",
					width, height, control, button.Bounds.Dx(), touchControlMinSize)
			}
		}
	}
}

// TestTouchKeypadButtonsAreSeparate guards against a keypad key landing on top
// of another button, which would make one of them unpressable.
func TestTouchKeypadButtonsAreSeparate(t *testing.T) {
	for _, size := range [][2]int{{1080, 2280}, {393, 851}} {
		width, height := size[0], size[1]
		buttons := touchControlButtonsWithOptions(width, height, keypadOptions())
		for i := range buttons {
			for j := i + 1; j < len(buttons); j++ {
				if buttons[i].Bounds.Intersect(buttons[j].Bounds).Empty() {
					continue
				}
				t.Errorf("%dx%d: %q overlaps %q (%v and %v)",
					width, height, buttons[i].ID, buttons[j].ID,
					buttons[i].Bounds, buttons[j].Bounds)
			}
		}
	}
}

// TestTouchKeypadKeysAcceptPlacements is what the user asked for: the numeric
// keys move with the layout editor like any other deck button.
func TestTouchKeypadKeysAcceptPlacements(t *testing.T) {
	const width, height = 1080, 2280
	options := keypadOptions()
	options.Placements = map[string]TouchPlacement{
		"num5": {X: 0.25, Y: 0.4},
		"star": {X: 0.75, Y: 0.6},
	}
	for _, button := range touchControlButtonsWithOptions(width, height, options) {
		placement, ok := options.Placements[button.ID]
		if !ok {
			continue
		}
		center := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
		wantX := int(placement.X * float64(width))
		wantY := int(placement.Y * float64(height))
		if abs(center.X-wantX) > 2 || abs(center.Y-wantY) > 2 {
			t.Errorf("%q centered at %v, want about (%d,%d)",
				button.ID, center, wantX, wantY)
		}
	}
}

// TestTouchKeypadIsHitTestable makes sure a press on a numeric key resolves to
// it, which is what the deck's input path relies on.
func TestTouchKeypadIsHitTestable(t *testing.T) {
	const width, height = 1080, 2280
	options := keypadOptions()
	for _, want := range touchControlButtonsWithOptions(width, height, options) {
		center := want.Bounds.Min.Add(want.Bounds.Size().Div(2))
		got, ok := touchButtonAtWithOptions(center.X, center.Y, width, height, options)
		if !ok {
			t.Errorf("no button at the center of %q", want.ID)
			continue
		}
		if got.ID != want.ID {
			t.Errorf("center of %q resolved to %q", want.ID, got.ID)
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
