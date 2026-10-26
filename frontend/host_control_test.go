package frontend

import (
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// A second physical panel presses controls through SetHostControl, and the
// game loop samples the held set with collectHostControlState. A press stays
// held until an explicit release, an empty name is ignored, and releasing one
// control leaves the others untouched.
func TestHostControlInjectionHoldsUntilReleased(t *testing.T) {
	shell := &Shell{}

	shell.SetHostControl("num5", true)
	shell.SetHostControl("ok", true)
	shell.SetHostControl("", true) // no control name: ignored

	state := map[string]bool{}
	shell.collectHostControlState(state)
	if !state["num5"] || !state["ok"] {
		t.Fatalf("held host controls missing: %v", state)
	}
	if _, ok := state[""]; ok {
		t.Fatal("an empty control name must be ignored")
	}

	shell.SetHostControl("num5", false)
	state = map[string]bool{}
	shell.collectHostControlState(state)
	if state["num5"] {
		t.Fatal("released control is still reported held")
	}
	if !state["ok"] {
		t.Fatal("releasing one control dropped another that was still held")
	}
}

// Turning the second panel off must release anything it still held so no
// control is left stuck pressed in the backend after the panel disappears.
func TestSecondaryKeypadDeactivationReleasesHostControls(t *testing.T) {
	shell := &Shell{}
	shell.SetSecondaryKeypadActive(true)
	shell.SetHostControl("left", true)

	shell.SetSecondaryKeypadActive(false)

	state := map[string]bool{}
	shell.collectHostControlState(state)
	if len(state) != 0 {
		t.Fatalf("host controls must clear when the panel goes away: %v", state)
	}
}

// With a controller connected the on-screen controls hide by default, so the
// handset's physical buttons play without the deck in the way.
func TestOnScreenControlsHideWithControllerByDefault(t *testing.T) {
	if defaultSettings().ShowControlsWithPad {
		t.Fatal("default should hide the on-screen controls with a controller")
	}
}

// The setting is a straight toggle the player can flip back and forth.
func TestToggleShowControlsWithPadFlips(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	before := shell.settings.ShowControlsWithPad
	shell.toggleShowControlsWithPad()
	if shell.settings.ShowControlsWithPad == before {
		t.Fatal("toggle did not flip the setting")
	}
	shell.toggleShowControlsWithPad()
	if shell.settings.ShowControlsWithPad != before {
		t.Fatal("toggling twice did not restore the setting")
	}
}

// Android docks touch controls by default so they do not cover the guest;
// other platforms retain the overlay default. Loading a settings file written
// before the option existed must inherit the platform default, while an
// explicit choice must still survive serialization and loading.
func TestTouchControlsOverlayDefaultMatchesPlatformAndPersists(t *testing.T) {
	wantDefault := runtime.GOOS != "android"
	if got := defaultSettings().TouchControlsOverlay; got != wantDefault {
		t.Fatalf("touch controls overlay default on %s = %t, want %t", runtime.GOOS, got, wantDefault)
	}
	legacy := defaultSettings()
	if err := json.Unmarshal([]byte(`{"theme_mode":"dark"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.TouchControlsOverlay != wantDefault {
		t.Fatalf("legacy settings overlay on %s = %t, want %t", runtime.GOOS, legacy.TouchControlsOverlay, wantDefault)
	}

	for _, explicit := range []bool{false, true} {
		blob, err := json.Marshal(Settings{TouchControlsOverlay: explicit})
		if err != nil {
			t.Fatal(err)
		}
		wantField := `"touch_controls_overlay":` + strconv.FormatBool(explicit)
		if !strings.Contains(string(blob), wantField) {
			t.Fatalf("touch_controls_overlay was omitted from %s", blob)
		}
		loaded := defaultSettings()
		if err := json.Unmarshal(blob, &loaded); err != nil {
			t.Fatal(err)
		}
		if loaded.TouchControlsOverlay != explicit {
			t.Fatalf("an explicit overlay value %t loaded as %t", explicit, loaded.TouchControlsOverlay)
		}
	}
}

func TestTouchControlsOverlayToggleAndDeckReservation(t *testing.T) {
	isolateSettings(t)
	shell := &Shell{settings: defaultSettings()}
	shell.settings.TouchControlsOverlay = true
	shell.input = &InputInfo{DisplayName: "loaded.dat"}
	shell.settings.ShowVirtualKeypad = true
	const width, height = 1080, 2280
	if got := shell.touchDeckHeight(width, height); got != 0 {
		t.Fatalf("overlay deck reserved %d pixels, want 0", got)
	}

	shell.toggleTouchControlsOverlay()
	if shell.settings.TouchControlsOverlay {
		t.Fatal("overlay toggle did not turn the option off")
	}
	want := touchDeckHeightWithOptions(width, height, shell.touchLayoutOptions())
	if got := shell.touchDeckHeight(width, height); got != want {
		t.Fatalf("docked deck height = %d, want %d", got, want)
	}

	shell.toggleTouchControlsOverlay()
	if !shell.settings.TouchControlsOverlay {
		t.Fatal("second overlay toggle did not restore the option")
	}
}

// The controller rule only touches on-screen layouts; a desktop window keeps
// its own controls no matter what the host reports.
func TestControllerConnectionLeavesDesktopControls(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.SetControllerConnected(true)
	if shell.onScreenControlsHidden() {
		t.Fatal("desktop must keep its controls when a controller connects")
	}
}
