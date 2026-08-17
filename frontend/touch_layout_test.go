package frontend

import (
	"math"
	"testing"
)

func TestTouchScaleFactorClampsThePersistedPercent(t *testing.T) {
	tests := map[int]float64{
		0:   1,
		100: 1,
		80:  0.8,
		140: 1.4,
		10:  0.8,
		999: 1.4,
		-50: 0.8,
		120: 1.2,
	}
	for percent, want := range tests {
		if got := touchScaleFactor(percent); math.Abs(got-want) > 1e-9 {
			t.Errorf("touchScaleFactor(%d) = %v, want %v", percent, got, want)
		}
	}
}

func TestTouchPlacementMovesOnlyTheOverriddenButton(t *testing.T) {
	width, height := 411, 914
	options := touchLayoutOptions{
		Scale: 1,
		Placements: map[string]TouchPlacement{
			"menu": {X: 0.5, Y: 0.2},
		},
	}
	defaults := touchControlButtonsFor(width, height)
	custom := touchControlButtonsWithOptions(width, height, options)
	if len(defaults) != len(custom) {
		t.Fatalf("button count changed: %d != %d", len(defaults), len(custom))
	}
	for index := range defaults {
		if defaults[index].ID == "menu" {
			if defaults[index].Bounds == custom[index].Bounds {
				t.Errorf("menu button did not move: %v", custom[index].Bounds)
			}
			center := custom[index].Bounds.Min.Add(custom[index].Bounds.Size().Div(2))
			if center.X < 200 || center.X > 211 {
				t.Errorf("menu center X = %d, want about %d", center.X, width/2)
			}
			continue
		}
		if defaults[index].Bounds != custom[index].Bounds {
			t.Errorf(
				"button %q moved without a placement: %v != %v",
				defaults[index].ID,
				custom[index].Bounds,
				defaults[index].Bounds,
			)
		}
	}
}

func TestTouchPlacementStaysOnTheInteractiveScreen(t *testing.T) {
	width, height := 411, 914
	for _, placement := range []TouchPlacement{
		{X: -3, Y: -3},
		{X: 3, Y: 3},
		{X: 0, Y: 1},
		{X: 1, Y: 0},
	} {
		options := touchLayoutOptions{
			Scale:      1.4,
			Placements: map[string]TouchPlacement{"back": placement},
		}
		for _, button := range touchControlButtonsWithOptions(width, height, options) {
			if button.ID != "back" {
				continue
			}
			if button.Bounds.Min.X < 0 || button.Bounds.Min.Y < 0 ||
				button.Bounds.Max.X > width ||
				button.Bounds.Max.Y > height-statusBarHeight {
				t.Errorf(
					"placed button escapes screen for %+v: %v",
					placement,
					button.Bounds,
				)
			}
		}
	}
}

func TestTouchButtonIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, button := range touchControlButtonsFor(411, 914) {
		if button.ID == "" {
			t.Fatalf("button %q has no ID", button.Control)
		}
		if seen[button.ID] {
			t.Errorf("duplicate touch button ID %q", button.ID)
		}
		seen[button.ID] = true
	}
}

func TestPlacedButtonsUseTheFullScaledSize(t *testing.T) {
	width, height := 411, 914
	options := touchLayoutOptions{
		Scale:      1.4,
		Placements: map[string]TouchPlacement{"ok-action": {X: 0.5, Y: 0.45}},
	}
	var placed, grid int
	for _, button := range touchControlButtonsWithOptions(width, height, options) {
		if button.ID == "ok-action" {
			placed = button.Bounds.Dx()
		} else if button.ID == "ok" {
			grid = button.Bounds.Dx()
		}
	}
	if placed <= grid {
		t.Fatalf("placed button size %d should exceed fit-limited grid size %d", placed, grid)
	}
}

func TestNormalizedTouchPlacementRoundTrips(t *testing.T) {
	width, height := 411, 914
	for _, point := range [][2]int{{60, 700}, {205, 457}, {380, 120}} {
		placement := normalizedTouchPlacement(point[0], point[1], width, height)
		bounds := placedTouchBounds(placement, 56, width, height)
		center := bounds.Min.Add(bounds.Size().Div(2))
		if dx := center.X - point[0]; dx < -2 || dx > 2 {
			t.Errorf("center X drifted: %d -> %d", point[0], center.X)
		}
		if dy := center.Y - point[1]; dy < -2 || dy > 2 {
			t.Errorf("center Y drifted: %d -> %d", point[1], center.Y)
		}
	}
}

func TestTouchLayoutEditorActionsAreHittable(t *testing.T) {
	for _, width := range []int{360, 411, 914} {
		for _, action := range touchLayoutEditorActions(width) {
			point := action.Bounds.Min.Add(action.Bounds.Size().Div(2))
			id, ok := touchLayoutEditorActionAt(point.X, point.Y, width)
			if !ok || id != action.ID {
				t.Errorf(
					"editor action %q hit result = %q, %t at width %d",
					action.ID,
					id,
					ok,
					width,
				)
			}
			if action.Bounds.Min.X < 0 || action.Bounds.Max.X > width {
				t.Errorf("editor action %q escapes width %d: %v", action.ID, width, action.Bounds)
			}
		}
	}
}

func TestSavingTheTouchLayoutPersistsTheDraft(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	t.Setenv("HOME", temporary)

	shell := &Shell{settings: defaultSettings()}
	shell.touchLayoutEditing = true
	shell.touchLayoutDraft = map[string]TouchPlacement{
		"menu": {X: 0.4, Y: 0.3},
	}
	shell.saveTouchLayoutEdit()
	if shell.touchLayoutEditing {
		t.Fatal("editing still active after save")
	}
	if placement := shell.settings.TouchLayout["menu"]; placement.X != 0.4 || placement.Y != 0.3 {
		t.Fatalf("layout not applied: %+v", shell.settings.TouchLayout)
	}

	loaded := loadSettings()
	if placement := loaded.TouchLayout["menu"]; placement.X != 0.4 || placement.Y != 0.3 {
		t.Fatalf("layout not persisted: %+v", loaded.TouchLayout)
	}
}

func TestCancelingTheTouchLayoutKeepsTheSavedLayout(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	shell.settings.TouchLayout = map[string]TouchPlacement{"back": {X: 0.2, Y: 0.9}}
	shell.touchLayoutEditing = true
	shell.touchLayoutDraft = map[string]TouchPlacement{"back": {X: 0.7, Y: 0.1}}
	shell.cancelTouchLayoutEdit()
	if shell.touchLayoutEditing {
		t.Fatal("editing still active after cancel")
	}
	if placement := shell.settings.TouchLayout["back"]; placement.X != 0.2 || placement.Y != 0.9 {
		t.Fatalf("cancel altered the saved layout: %+v", shell.settings.TouchLayout)
	}
}

func TestResettingTheTouchLayoutClearsTheDraft(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	shell.touchLayoutEditing = true
	shell.touchLayoutDraft = map[string]TouchPlacement{"up": {X: 0.5, Y: 0.5}}
	shell.resetTouchLayoutDraft()
	if len(shell.touchLayoutDraft) != 0 {
		t.Fatalf("draft not cleared: %+v", shell.touchLayoutDraft)
	}
	if !shell.touchLayoutEditing {
		t.Fatal("reset should stay in the editor")
	}
}

func TestFocusControlsScaleWithTheTouchSetting(t *testing.T) {
	width, height := 914, 411
	small := focusControlButtonsScaled(width, height, 0.8)
	base := focusControlButtonsScaled(width, height, 1)
	large := focusControlButtonsScaled(width, height, 1.4)
	if small[0].Bounds.Dx() >= base[0].Bounds.Dx() {
		t.Fatalf(
			"scale 0.8 pad %d should shrink below %d",
			small[0].Bounds.Dx(),
			base[0].Bounds.Dx(),
		)
	}
	if large[0].Bounds.Dx() < base[0].Bounds.Dx() {
		t.Fatalf(
			"scale 1.4 pad %d should not shrink below %d",
			large[0].Bounds.Dx(),
			base[0].Bounds.Dx(),
		)
	}
	for i, first := range large {
		for _, second := range large[i+1:] {
			if first.Bounds.Overlaps(second.Bounds) {
				t.Errorf(
					"scaled focus buttons %q and %q overlap",
					first.Control,
					second.Control,
				)
			}
		}
	}
}
