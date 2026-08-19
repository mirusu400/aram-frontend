package frontend

import "testing"

func TestTouchChromeToggleBoundsStayClearOfTheDeck(t *testing.T) {
	for _, size := range [][2]int{{411, 914}, {914, 411}, {logicalWidth, logicalHeight}} {
		width, height := size[0], size[1]
		deckTop := height - statusBarHeight - touchDeckHeight(width, height)
		for _, hidden := range []bool{true, false} {
			bounds := touchChromeToggleBounds(width, hidden)
			if bounds.Min.X < 0 || bounds.Max.X > width || bounds.Min.Y < 0 {
				t.Fatalf(
					"toggle (hidden=%t) outside %dx%d: %v",
					hidden,
					width,
					height,
					bounds,
				)
			}
			if bounds.Max.Y > deckTop {
				t.Fatalf(
					"toggle (hidden=%t) reaches the control deck: %v",
					hidden,
					bounds,
				)
			}
		}
		visible := touchChromeToggleBounds(width, false)
		if visible.Min.Y < menuBarHeight ||
			visible.Max.Y > menuBarHeight+applicationToolbarHeight {
			t.Fatalf("visible-chrome toggle leaves the toolbar row: %v", visible)
		}
	}
}

func TestSyncTouchChromeHidesWhenATitleStartsRunning(t *testing.T) {
	shell := &Shell{state: FrontendReady}
	shell.syncTouchChrome()
	if shell.touchChromeHidden {
		t.Fatal("chrome hid before any title ran")
	}
	shell.state = FrontendRunning
	shell.syncTouchChrome()
	if shell.touchChromeHidden != platformUsesTouchLayout() {
		t.Fatalf(
			"running transition hid chrome = %t, want %t",
			shell.touchChromeHidden,
			platformUsesTouchLayout(),
		)
	}
	// Showing the chrome during play must stick until the next transition.
	shell.touchChromeHidden = false
	shell.syncTouchChrome()
	if shell.touchChromeHidden {
		t.Fatal("chrome re-hid without a state transition")
	}
	shell.state = FrontendPaused
	shell.syncTouchChrome()
	shell.state = FrontendRunning
	shell.syncTouchChrome()
	if shell.touchChromeHidden != platformUsesTouchLayout() {
		t.Fatalf(
			"resume transition hid chrome = %t, want %t",
			shell.touchChromeHidden,
			platformUsesTouchLayout(),
		)
	}
}

func TestTouchChromeHiddenYieldsToPanelsEditorAndFocusMode(t *testing.T) {
	shell := &Shell{touchChromeHidden: true}
	if shell.touchChromeHiddenActive() != platformUsesTouchLayout() {
		t.Fatalf(
			"touchChromeHiddenActive = %t, want %t",
			shell.touchChromeHiddenActive(),
			platformUsesTouchLayout(),
		)
	}
	shell.panel = &Panel{Kind: "settings"}
	if shell.touchChromeHiddenActive() {
		t.Fatal("an open panel must force the full chrome")
	}
	shell.panel = nil
	shell.touchLayoutEditing = true
	if shell.touchChromeHiddenActive() {
		t.Fatal("the layout editor must own the whole screen")
	}
	shell.touchLayoutEditing = false
	shell.focusMode = true
	if shell.touchChromeHiddenActive() {
		t.Fatal("focus mode must keep its own chrome-less surface")
	}
}

func TestFilledGuestViewportIgnoresIntegerScalingFloor(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	if !shell.settings.IntegerScaling {
		t.Fatal("integer scaling is expected to default on")
	}
	viewport := rectAt(0, 0, 392, 520)
	windowed := shell.frameDestination(viewport, 240, 320)
	if windowed.Dx() != 240 || windowed.Dy() != 320 {
		t.Fatalf("windowed integer-scaled frame = %v", windowed)
	}
	shell.fillGuestViewport = true
	filled := shell.frameDestination(viewport, 240, 320)
	if filled.Dx() <= windowed.Dx() || filled.Dy() <= windowed.Dy() {
		t.Fatalf("filled frame %v is not larger than windowed %v", filled, windowed)
	}
	if filled.Dx() > viewport.Dx() || filled.Dy() > viewport.Dy() {
		t.Fatalf("filled frame %v leaves the viewport", filled)
	}
	// The aspect ratio must survive the fill (within rounding).
	skew := filled.Dx()*320 - filled.Dy()*240
	if skew < -320 || skew > 320 {
		t.Fatalf("filled frame %v distorts the 240x320 aspect", filled)
	}
}

func TestToggleTouchChromeClosesTheActiveMenu(t *testing.T) {
	shell := &Shell{activeMenu: 2}
	shell.toggleTouchChrome()
	if !shell.touchChromeHidden || shell.activeMenu != -1 {
		t.Fatalf(
			"toggle state = hidden:%t menu:%d",
			shell.touchChromeHidden,
			shell.activeMenu,
		)
	}
	shell.toggleTouchChrome()
	if shell.touchChromeHidden {
		t.Fatal("second toggle did not restore the chrome")
	}
}
