package frontend

import "testing"

func TestTouchControlHitTargets(t *testing.T) {
	for _, size := range [][2]int{{logicalWidth, logicalHeight}, {720, 540}, {1440, 900}} {
		width, height := size[0], size[1]
		for _, button := range touchControlButtonsFor(width, height) {
			point := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
			control, ok := touchControlAtSize(point.X, point.Y, width, height)
			if !ok || control != button.Control {
				t.Errorf(
					"touchControlAtSize(%d, %d, %d, %d) = %q, %t; want %q",
					point.X,
					point.Y,
					width,
					height,
					control,
					ok,
					button.Control,
				)
			}
			if button.Bounds.Min.X < 0 ||
				button.Bounds.Min.Y < 0 ||
				button.Bounds.Max.X > width ||
				button.Bounds.Max.Y > height-statusBarHeight {
				t.Errorf("touch button outside %dx%d: %#v", width, height, button)
			}
		}
	}
	if control, ok := touchControlAtSize(900, 300, logicalWidth, logicalHeight); ok {
		t.Fatalf("non-control point mapped to %q", control)
	}
}

func TestTouchDeckHasNoNavigationRow(t *testing.T) {
	for _, size := range [][2]int{{411, 914}, {914, 411}, {logicalWidth, logicalHeight}} {
		width, height := size[0], size[1]
		deckTop := height - statusBarHeight - touchDeckHeight(width, height)
		for _, button := range touchControlButtonsFor(width, height) {
			if button.Bounds.Min.Y < deckTop {
				t.Errorf(
					"touch button %q above deck at %dx%d: %v (deck top %d)",
					button.Control,
					width,
					height,
					button.Bounds,
					deckTop,
				)
			}
		}
	}
}

func TestTouchControlButtonsGrewWithoutOverlap(t *testing.T) {
	for _, size := range [][2]int{{411, 914}, {914, 411}, {720, 540}, {1440, 900}} {
		width, height := size[0], size[1]
		buttons := touchControlButtonsFor(width, height)
		for _, button := range buttons {
			if button.Bounds.Dx() < 44 || button.Bounds.Dy() < 44 {
				t.Errorf(
					"touch button %q too small at %dx%d: %v",
					button.Control,
					width,
					height,
					button.Bounds,
				)
			}
		}
		for i, first := range buttons {
			for _, second := range buttons[i+1:] {
				if first.Bounds.Overlaps(second.Bounds) {
					t.Errorf(
						"touch buttons %q and %q overlap at %dx%d",
						first.Control,
						second.Control,
						width,
						height,
					)
				}
			}
		}
	}
}

func TestResponsiveLayoutUsesAvailableWindow(t *testing.T) {
	shell := &Shell{}
	if width, height := shell.Layout(1440, 900); width != 1440 || height != 900 {
		t.Fatalf("large layout = %dx%d", width, height)
	}
	if width, height := shell.viewportSize(); width != 1440 || height != 900 {
		t.Fatalf("stored viewport = %dx%d", width, height)
	}
	if width, height := shell.Layout(720, 540); width != 720 || height != 540 {
		t.Fatalf("compact layout = %dx%d", width, height)
	}
}

func TestVirtualKeypadStaysInRightRailAndMapsEveryButton(t *testing.T) {
	for _, size := range [][2]int{{720, 540}, {960, 720}, {1440, 900}} {
		width, height := size[0], size[1]
		panel := virtualKeypadPanelBoundsFor(width, height)
		if panel.Min.X <= width/2 || panel.Max.X > width ||
			panel.Min.Y < menuHeight+applicationToolbarHeight ||
			panel.Max.Y > height-statusHeight {
			t.Fatalf("virtual keypad panel outside right rail at %dx%d: %v", width, height, panel)
		}
		buttons := virtualKeypadButtonsFor(width, height, true)
		if len(buttons) != 25 {
			t.Fatalf("virtual keypad button count = %d, want 25", len(buttons))
		}
		phoneKeys := map[string]string{
			"send": "CALL", "end": "END", "soft-right": "CANCEL", "back": "C",
			"volume-up": "VOL+", "volume-down": "VOL-",
		}
		for _, button := range buttons {
			if label, ok := phoneKeys[button.Control]; ok {
				if button.Label != label {
					t.Errorf("virtual key %q label = %q, want %q", button.Control, button.Label, label)
				}
				delete(phoneKeys, button.Control)
			}
			point := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
			control, ok := virtualKeypadControlAtSize(
				point.X,
				point.Y,
				width,
				height,
				true,
			)
			if !ok || control != button.Control {
				t.Errorf("virtual key %q hit result = %q, %t", button.Control, control, ok)
			}
			if !button.Bounds.In(panel) {
				t.Errorf("virtual key %q outside panel: %v not in %v", button.Control, button.Bounds, panel)
			}
		}
		if len(phoneKeys) != 0 {
			t.Errorf("virtual keypad is missing phone controls: %v", phoneKeys)
		}
	}
}

func TestTouchDeckDistinguishesCancelFromClear(t *testing.T) {
	labels := make(map[string]string)
	for _, button := range touchControlButtonsFor(720, 900) {
		labels[button.Control] = button.Label
	}
	if got := labels["soft-right"]; got != "CANCEL" {
		t.Errorf("right soft key label = %q, want CANCEL", got)
	}
	if got := labels["back"]; got != "C" {
		t.Errorf("clear key label = %q, want C", got)
	}
}
