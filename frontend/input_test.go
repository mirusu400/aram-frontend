package frontend

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

type inputRecorder struct {
	attempts []InputEvent
	events   []InputEvent
	err      error
}

func (recorder *inputRecorder) QueueInput(event InputEvent) error {
	recorder.attempts = append(recorder.attempts, event)
	if recorder.err != nil {
		return recorder.err
	}
	recorder.events = append(recorder.events, event)
	return nil
}

func TestLiveInputTransitionsUseCurrentGuestTime(t *testing.T) {
	recorder := &inputRecorder{}
	shell := &Shell{
		controlState: map[string]bool{"left": true},
	}

	shell.queueInputTransitions(recorder, map[string]bool{"ok": true})

	if len(recorder.events) != 2 {
		t.Fatalf("input events = %#v", recorder.events)
	}
	events := make(map[string]InputEvent, len(recorder.events))
	for _, event := range recorder.events {
		events[event.Control] = event
		if event.At != 0 {
			t.Fatalf("%s input time = %s, want current guest time", event.Control, event.At)
		}
	}
	if event := events["left"]; event.Pressed {
		t.Fatalf("left transition = %#v, want release", event)
	}
	if event := events["ok"]; !event.Pressed {
		t.Fatalf("ok transition = %#v, want press", event)
	}
	if shell.controlState["left"] || !shell.controlState["ok"] {
		t.Fatalf("delivered control state = %#v", shell.controlState)
	}

	shell.queueInputTransitions(recorder, map[string]bool{"ok": true})
	if len(recorder.events) != 2 {
		t.Fatalf("unchanged controls emitted another transition: %#v", recorder.events)
	}
}

func TestFailedInputTransitionIsRetried(t *testing.T) {
	recorder := &inputRecorder{err: errors.New("queue busy")}
	shell := &Shell{controlState: make(map[string]bool)}

	shell.queueInputTransitions(recorder, map[string]bool{"up": true})
	if shell.controlState["up"] {
		t.Fatal("failed input was recorded as delivered")
	}

	recorder.err = nil
	shell.queueInputTransitions(recorder, map[string]bool{"up": true})
	if len(recorder.events) != 1 || !recorder.events[0].Pressed ||
		recorder.events[0].Control != "up" {
		t.Fatalf("retried input events = %#v", recorder.events)
	}
}

func TestDirectionalInputReleasesPreviousDirectionBeforePressingNext(t *testing.T) {
	recorder := &inputRecorder{}
	shell := &Shell{controlState: make(map[string]bool)}

	shell.queueInputTransitions(recorder, map[string]bool{"left": true})
	shell.queueInputTransitions(recorder, map[string]bool{
		"left": true,
		"down": true,
	})

	want := []InputEvent{
		{Control: "left", Pressed: true},
		{Control: "left", Pressed: false},
		{Control: "down", Pressed: true},
	}
	if len(recorder.events) != len(want) {
		t.Fatalf("directional input events = %#v, want %#v", recorder.events, want)
	}
	for index := range want {
		if recorder.events[index] != want[index] {
			t.Fatalf("directional input event %d = %#v, want %#v", index, recorder.events[index], want[index])
		}
	}
	if shell.controlState["left"] || !shell.controlState["down"] {
		t.Fatalf("delivered directional state = %#v, want only down", shell.controlState)
	}

	shell.queueInputTransitions(recorder, map[string]bool{
		"left": true,
		"down": true,
	})
	if len(recorder.events) != len(want) {
		t.Fatalf("held directions emitted another transition: %#v", recorder.events)
	}
}

func TestDirectionalInputWaitsForFailedRelease(t *testing.T) {
	recorder := &inputRecorder{}
	shell := &Shell{controlState: make(map[string]bool)}
	shell.queueInputTransitions(recorder, map[string]bool{"left": true})

	recorder.err = errors.New("queue busy")
	shell.queueInputTransitions(recorder, map[string]bool{
		"left": true,
		"down": true,
	})
	if len(recorder.attempts) != 2 || recorder.attempts[1] != (InputEvent{
		Control: "left",
		Pressed: false,
	}) {
		t.Fatalf("failed direction switch attempts = %#v, want only left release", recorder.attempts)
	}
	if !shell.controlState["left"] || shell.controlState["down"] {
		t.Fatalf("state after failed release = %#v, want only left", shell.controlState)
	}

	recorder.err = nil
	shell.queueInputTransitions(recorder, map[string]bool{
		"left": true,
		"down": true,
	})
	want := []InputEvent{
		{Control: "left", Pressed: true},
		{Control: "left", Pressed: false},
		{Control: "down", Pressed: true},
	}
	if len(recorder.events) != len(want) {
		t.Fatalf("retried direction switch events = %#v, want %#v", recorder.events, want)
	}
	for index := range want {
		if recorder.events[index] != want[index] {
			t.Fatalf("retried direction event %d = %#v, want %#v", index, recorder.events[index], want[index])
		}
	}
}

func TestGamepadLayoutSwapsConfirmAndBack(t *testing.T) {
	standard := bindingButtons(gamepadBindings("standard"))
	swapped := bindingButtons(gamepadBindings("swapped"))

	if standard["ok"] != ebiten.StandardGamepadButtonRightBottom ||
		standard["back"] != ebiten.StandardGamepadButtonRightRight {
		t.Fatalf("standard confirm/back = %#v", standard)
	}
	if swapped["ok"] != ebiten.StandardGamepadButtonRightRight ||
		swapped["back"] != ebiten.StandardGamepadButtonRightBottom {
		t.Fatalf("swapped confirm/back = %#v", swapped)
	}
}

func TestAnalogDirectionsRespectDeadzoneAndDiagonals(t *testing.T) {
	state := map[string]bool{"up": true}
	applyAnalogDirections(state, 0.29, -0.30, 0.30)
	if state["right"] {
		t.Fatal("horizontal movement inside the dead zone pressed right")
	}
	if !state["up"] {
		t.Fatal("vertical movement at the dead-zone boundary did not press up")
	}

	applyAnalogDirections(state, -0.8, 0.7, 0.30)
	if !state["left"] || !state["down"] {
		t.Fatalf("diagonal analog state = %#v", state)
	}
}

func TestCustomGamepadBindingOverridesPreset(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()
	profile.GamepadLayout = "custom"
	profile.GamepadBindings = map[string]string{"ok": "face-north"}

	bindings := bindingButtons(gamepadBindingsForProfile(profile))
	if bindings["ok"] != ebiten.StandardGamepadButtonRightTop {
		t.Fatalf("custom confirm binding = %v", bindings["ok"])
	}
	if bindings["back"] != ebiten.StandardGamepadButtonRightRight {
		t.Fatalf("unchanged back binding = %v", bindings["back"])
	}
}

func TestKeyboardBindingCaptureUsesThePressedKey(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()
	if swapped := assignKeyboardBinding(&profile, "left", ebiten.KeyJ); swapped != "" {
		t.Fatalf("unexpected first swap with %q", swapped)
	}
	if profile.KeyboardProfile != "custom" {
		t.Fatalf("keyboard profile = %q, want custom", profile.KeyboardProfile)
	}
	keys := keyboardBindingIDsForProfile(profile)
	if keys["left"] != ebiten.KeyJ.String() {
		t.Fatalf("left binding = %q, want J", keys["left"])
	}

	if swapped := assignKeyboardBinding(&profile, "right", ebiten.KeyJ); swapped != "left" {
		t.Fatalf("duplicate binding swapped with %q, want left", swapped)
	}
	keys = keyboardBindingIDsForProfile(profile)
	if keys["right"] != ebiten.KeyJ.String() ||
		keys["left"] != ebiten.KeyArrowRight.String() {
		t.Fatalf("swapped keyboard bindings = %#v", keys)
	}
}

func TestKeyboardBindingsIncludePhoneNumberControls(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()
	bindings := keyboardBindingIDsForProfile(profile)
	for number := 0; number <= 9; number++ {
		control := fmt.Sprintf("num%d", number)
		if bindings[control] == "" {
			t.Errorf("%s has no keyboard binding", control)
		}
	}
}

func TestKeyboardBindingsIncludeIndependentPhoneVolumeControls(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()
	bindings := keyboardBindingIDsForProfile(profile)
	if got := bindings["volume-up"]; got != ebiten.KeyPageUp.String() {
		t.Fatalf("volume-up binding = %q, want %q", got, ebiten.KeyPageUp.String())
	}
	if got := bindings["volume-down"]; got != ebiten.KeyPageDown.String() {
		t.Fatalf("volume-down binding = %q, want %q", got, ebiten.KeyPageDown.String())
	}
	if bindings["volume-up"] == bindings["volume-down"] {
		t.Fatal("phone volume controls share one keyboard binding")
	}
}

func TestKeyboardBindingsIncludeIndependentPhoneActionControls(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()
	bindings := keyboardBindingIDsForProfile(profile)
	want := map[string]ebiten.Key{
		"send": ebiten.KeyHome,
		"end":  ebiten.KeyEnd,
		"back": ebiten.KeyBackspace,
	}
	seen := make(map[string]bool, len(want))
	for control, key := range want {
		if got := bindings[control]; got != key.String() {
			t.Errorf("%s binding = %q, want %q", control, got, key.String())
		}
		if seen[bindings[control]] {
			t.Errorf("phone action controls reuse keyboard binding %q", bindings[control])
		}
		seen[bindings[control]] = true
	}

	gamepadIDs := bindingIDs(gamepadBindingsForProfile(profile))
	for _, control := range []string{"send", "end", "volume-up", "volume-down"} {
		if gamepadIDs[control] != "" {
			t.Errorf("%s should start with no gamepad binding, got %q", control, gamepadIDs[control])
		}
	}
}

func TestGamepadBindingCaptureSupportsNumberControls(t *testing.T) {
	profile := defaultSettings().globalControllerProfile()

	// Numbers have no default gamepad button; they start Unassigned.
	ids := bindingIDs(gamepadBindingsForProfile(profile))
	if ids["num1"] != "" {
		t.Fatalf("num1 should start unassigned, got %q", ids["num1"])
	}

	// A spare button binds a number without stealing anything.
	if swapped := assignGamepadBinding(&profile, "num1", "trigger-left"); swapped != "" {
		t.Fatalf("binding num1 to a spare button swapped %q", swapped)
	}
	if profile.GamepadLayout != "custom" {
		t.Fatalf("gamepad layout = %q, want custom", profile.GamepadLayout)
	}
	ids = bindingIDs(gamepadBindingsForProfile(profile))
	if ids["num1"] != "trigger-left" {
		t.Fatalf("num1 binding = %q, want trigger-left", ids["num1"])
	}

	// Binding a number to a button a base control holds steals it, leaving the
	// base control Unassigned rather than snapping it back to its default.
	if swapped := assignGamepadBinding(&profile, "num2", "face-south"); swapped != "ok" {
		t.Fatalf("stealing face-south swapped %q, want ok", swapped)
	}
	profile.normalize()
	ids = bindingIDs(gamepadBindingsForProfile(profile))
	if ids["num2"] != "face-south" {
		t.Fatalf("num2 binding = %q, want face-south", ids["num2"])
	}
	if ids["ok"] != "" {
		t.Fatalf("stolen ok binding = %q, want unassigned", ids["ok"])
	}
}

func bindingButtons(bindings []gamepadBinding) map[string]ebiten.StandardGamepadButton {
	result := make(map[string]ebiten.StandardGamepadButton, len(bindings))
	for _, binding := range bindings {
		result[binding.Control] = binding.Button
	}
	return result
}

func bindingIDs(bindings []gamepadBinding) map[string]string {
	result := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		result[binding.Control] = binding.ID
	}
	return result
}
