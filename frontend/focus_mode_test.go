package frontend

import "testing"

var focusTestSizes = [][2]int{
	{logicalWidth, logicalHeight},
	{720, 540},
	{360, 780},
	{780, 360},
}

func TestFocusControlHitTargets(t *testing.T) {
	for _, size := range focusTestSizes {
		width, height := size[0], size[1]
		deckTop := height - focusDeckHeight(width, height)
		for _, button := range focusControlButtonsFor(width, height) {
			point := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
			control, ok := focusControlAtSize(point.X, point.Y, width, height)
			if !ok || control != button.Control {
				t.Errorf(
					"focusControlAtSize(%d, %d, %d, %d) = %q, %t; want %q",
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
				button.Bounds.Min.Y < deckTop ||
				button.Bounds.Max.X > width ||
				button.Bounds.Max.Y > height {
				t.Errorf("focus button outside the %dx%d deck: %#v", width, height, button)
			}
		}
	}
}

func TestFocusControlsKeepOnlyDirectionAndNumberKeys(t *testing.T) {
	want := map[string]bool{
		"up": true, "down": true, "left": true, "right": true, "ok": true,
		"num0": true, "num1": true, "num2": true, "num3": true, "num4": true,
		"num5": true, "num6": true, "num7": true, "num8": true, "num9": true,
		"star": true, "hash": true,
	}
	got := make(map[string]bool)
	for _, button := range focusControlButtonsFor(logicalWidth, logicalHeight) {
		got[button.Control] = true
	}
	for control := range want {
		if !got[control] {
			t.Errorf("focus layout is missing control %q", control)
		}
	}
	for control := range got {
		if !want[control] {
			t.Errorf("focus layout exposes unexpected control %q", control)
		}
	}
}

func TestFocusButtonsDoNotOverlap(t *testing.T) {
	for _, size := range focusTestSizes {
		width, height := size[0], size[1]
		buttons := focusControlButtonsFor(width, height)
		for i := range buttons {
			for j := i + 1; j < len(buttons); j++ {
				if buttons[i].Bounds.Overlaps(buttons[j].Bounds) {
					t.Errorf(
						"focus buttons overlap at %dx%d: %#v and %#v",
						width,
						height,
						buttons[i],
						buttons[j],
					)
				}
			}
		}
	}
}

func TestFocusExitButtonStaysClearOfTheDeck(t *testing.T) {
	for _, size := range focusTestSizes {
		width, height := size[0], size[1]
		exit := focusExitBoundsFor(width, height)
		if exit.Min.X < 0 || exit.Min.Y < 0 || exit.Max.X > width {
			t.Errorf("exit button outside the %dx%d screen: %v", width, height, exit)
		}
		deckTop := height - focusDeckHeight(width, height)
		if exit.Max.Y > deckTop {
			t.Errorf("exit button reaches into the %dx%d deck: %v", width, height, exit)
		}
	}
}

func TestFocusModeCommandTogglesAndClosesTheMenu(t *testing.T) {
	shell := &Shell{menus: defaultMenus(), activeMenu: 2}
	command, found := shell.findCommand("view.focus")
	if !found {
		t.Fatal("view.focus command is not registered")
	}
	if command.Enabled == nil {
		t.Fatal("view.focus must gate on the touch layout")
	}
	if command.Enabled(shell) != platformUsesTouchLayout() {
		t.Fatalf(
			"view.focus enabled = %t, want %t",
			command.Enabled(shell),
			platformUsesTouchLayout(),
		)
	}
	shell.toggleFocusMode()
	if !shell.focusMode {
		t.Fatal("toggleFocusMode did not enter focus mode")
	}
	if shell.activeMenu != -1 {
		t.Fatalf("focus mode left menu %d open", shell.activeMenu)
	}
	shell.toggleFocusMode()
	if shell.focusMode {
		t.Fatal("toggleFocusMode did not leave focus mode")
	}
}

func TestFocusModeActiveRequiresTouchLayout(t *testing.T) {
	shell := &Shell{focusMode: true}
	if shell.focusModeActive() != platformUsesTouchLayout() {
		t.Fatalf(
			"focusModeActive = %t, want %t",
			shell.focusModeActive(),
			platformUsesTouchLayout(),
		)
	}
}
