package frontend

import "testing"

// The pad resolves to exactly one of four directions: the dominant axis wins
// and a diagonal tie falls to the horizontal, so it never presses two ways at
// once the way an 8-way stick would.
func TestResolvePadDirection4(t *testing.T) {
	cases := []struct {
		dx, dy float64
		want   string
	}{
		{1, 0, "right"},
		{-1, 0, "left"},
		{0, 1, "down"},
		{0, -1, "up"},
		{5, 3, "right"}, // shallow right, x dominates
		{3, 5, "down"},  // steep down, y dominates
		{-2, -5, "up"},  // steep up
		{-5, 2, "left"}, // shallow left
		{4, 4, "right"}, // exact tie goes horizontal
		{-4, 4, "left"}, // tie, negative x
	}
	for _, tc := range cases {
		if got := resolvePadDirection4(tc.dx, tc.dy); got != tc.want {
			t.Errorf("resolvePadDirection4(%g, %g) = %q; want %q",
				tc.dx, tc.dy, got, tc.want)
		}
	}
}

// The round pad stands in for the whole directional cluster - the four arrows
// and the center OK - while the action cluster's own OK and every other key
// stay ordinary buttons.
func TestIsCircularPadSlotID(t *testing.T) {
	for _, id := range []string{"up", "down", "left", "right", "ok"} {
		if !isCircularPadSlotID(id) {
			t.Errorf("%q should belong to the circular pad", id)
		}
	}
	for _, id := range []string{"ok-action", "menu", "back", "soft-left", "num5", "star"} {
		if isCircularPadSlotID(id) {
			t.Errorf("%q must stay an ordinary button", id)
		}
	}
}

// The pad's circle sits exactly on the directional cross it replaces: centered
// on the middle key, and wide enough that every arrow center falls inside its
// bounding square, so a thumb anywhere on the old cross reaches the pad.
func TestCircularPadCoversTheCross(t *testing.T) {
	for _, size := range [][2]int{{logicalWidth, logicalHeight}, {720, 540}, {1440, 900}} {
		width, height := size[0], size[1]
		options := defaultTouchLayoutOptions()
		metrics := touchDeckMetricsFor(width, height, options)
		center, radius := circularPadCircle(metrics)
		if radius <= 0 {
			t.Fatalf("pad radius must be positive at %dx%d", width, height)
		}
		square := rectAt(center.X-radius, center.Y-radius, radius*2, radius*2)
		for _, button := range touchControlButtonsFor(width, height) {
			if !isCircularPadSlotID(button.ID) {
				continue
			}
			mid := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
			if button.ID == "ok" && mid != center {
				t.Errorf("pad center %v not on the middle key %v at %dx%d",
					center, mid, width, height)
			}
			if !pointInRect(mid.X, mid.Y, square) {
				t.Errorf("cross key %q center %v outside pad square %v at %dx%d",
					button.ID, mid, square, width, height)
			}
		}
	}
}

// A center tap - a press that never leaves the deadzone - confirms with OK; a
// drag that moved the knob just stops with nothing pressed.
func TestCircularPadReleaseTapArmsOK(t *testing.T) {
	shell := &Shell{padTouchActive: true, padTouchMoved: false}
	shell.releaseCircularPad()
	if shell.padOKPulse <= 0 {
		t.Fatal("a still center tap should arm the OK pulse")
	}
	if shell.padTouchActive {
		t.Fatal("release must clear the pad touch")
	}

	dragged := &Shell{padTouchActive: true, padTouchMoved: true, padDir: "left"}
	dragged.releaseCircularPad()
	if dragged.padOKPulse != 0 {
		t.Fatal("a drag release must not press OK")
	}
	if dragged.padDir != "" {
		t.Fatal("release must clear the held direction")
	}
}

// The held pad state folds into the collected control set: a live direction
// while steering, and OK for as long as the tap pulse lasts.
func TestCircularPadStateFeedsControls(t *testing.T) {
	shell := &Shell{padDir: "up"}
	state := map[string]bool{}
	shell.collectTouchState(state)
	if !state["up"] {
		t.Fatalf("held pad direction missing: %v", state)
	}

	shell = &Shell{padOKPulse: circularPadOKPulseFrames}
	state = map[string]bool{}
	shell.collectTouchState(state)
	if !state["ok"] {
		t.Fatalf("OK pulse not reported pressed: %v", state)
	}

	shell = &Shell{}
	state = map[string]bool{}
	shell.collectTouchState(state)
	if len(state) != 0 {
		t.Fatalf("an idle pad must press nothing: %v", state)
	}
}

// The round pad is the shipped default on a touch layout: a thumb steers it
// far more easily than it hits four separate d-pad keys.
func TestCircularPadDefaultsOn(t *testing.T) {
	if !defaultSettings().TouchDpadCircular {
		t.Fatal("the circular d-pad should be on by default")
	}
}

// The mode is a plain toggle the player can flip back and forth.
func TestToggleTouchDpadCircularFlips(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	before := shell.settings.TouchDpadCircular
	shell.toggleTouchDpadCircular()
	if shell.settings.TouchDpadCircular == before {
		t.Fatal("toggle did not flip the setting")
	}
	shell.toggleTouchDpadCircular()
	if shell.settings.TouchDpadCircular != before {
		t.Fatal("toggling twice did not restore the setting")
	}
}
