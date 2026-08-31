package frontend

import (
	"context"
	"testing"
)

// inertSideKeyBackend loads an input and then does nothing: the flag under test
// is set while the result is consumed, and a backend that offers no commands
// keeps the automatic start out of the way.
type inertSideKeyBackend struct{}

func (inertSideKeyBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	return InputInfo{}, nil
}
func (inertSideKeyBackend) State() BackendState          { return StateStopped }
func (inertSideKeyBackend) Supports(BackendCommand) bool { return false }
func (inertSideKeyBackend) Execute(context.Context, BackendCommand) error {
	return nil
}
func (inertSideKeyBackend) Close() error { return nil }

// The volume rocker belongs to the phone, not to an application title running
// on top of one. The rail keypad therefore only offers it to a whole-phone
// firmware session, and it must give up the two cells without disturbing any
// other key: a button that moves when a firmware session opens would retrain
// the user's aim for nothing.
func TestVirtualKeypadOffersSideKeysToFirmwareOnly(t *testing.T) {
	const width, height = 960, 720
	firmware := make(map[string]touchButton)
	for _, button := range virtualKeypadButtonsFor(width, height, true) {
		firmware[button.Control] = button
	}
	for _, control := range sideKeyControls {
		if _, ok := firmware[control]; !ok {
			t.Fatalf("firmware keypad is missing %q", control)
		}
	}

	application := make(map[string]touchButton)
	for _, button := range virtualKeypadButtonsFor(width, height, false) {
		application[button.Control] = button
	}
	for _, control := range sideKeyControls {
		if _, ok := application[control]; ok {
			t.Errorf("application keypad still offers %q", control)
		}
		point := firmware[control].Bounds.Min.
			Add(firmware[control].Bounds.Size().Div(2))
		if hit, ok := virtualKeypadControlAtSize(
			point.X,
			point.Y,
			width,
			height,
			false,
		); ok {
			t.Errorf("application keypad still hits %q at the %q cell", hit, control)
		}
	}
	if len(application) != len(firmware)-len(sideKeyControls) {
		t.Fatalf(
			"application keypad has %d keys, want %d",
			len(application),
			len(firmware)-len(sideKeyControls),
		)
	}
	for control, button := range application {
		if firmware[control].Bounds != button.Bounds {
			t.Errorf(
				"key %q moved between sessions: %v then %v",
				control,
				firmware[control].Bounds,
				button.Bounds,
			)
		}
	}
}

// A keyboard or gamepad binding can still name a side key while an application
// title is loaded. The press must be dropped before the transition pass, and a
// key already held when the session changed must be released rather than left
// stuck down in the backend.
func TestApplicationSessionReleasesHeldSideKey(t *testing.T) {
	recorder := &inputRecorder{}
	shell := &Shell{controlState: map[string]bool{"volume-up": true}}

	next := map[string]bool{"volume-up": true, "ok": true}
	shell.dropUnavailableControls(next)
	shell.queueInputTransitions(recorder, next)

	events := make(map[string]bool, len(recorder.events))
	for _, event := range recorder.events {
		events[event.Control] = event.Pressed
	}
	if pressed, ok := events["volume-up"]; !ok || pressed {
		t.Fatalf("volume-up events = %#v, want a release", recorder.events)
	}
	if shell.controlState["volume-up"] {
		t.Fatal("volume-up stayed pressed in an application session")
	}
	if !events["ok"] {
		t.Fatalf("unrelated control was dropped: %#v", recorder.events)
	}
}

func TestFirmwareSessionKeepsSideKey(t *testing.T) {
	recorder := &inputRecorder{}
	shell := &Shell{
		controlState:    map[string]bool{},
		firmwareSession: true,
	}

	next := map[string]bool{"volume-up": true}
	shell.dropUnavailableControls(next)
	shell.queueInputTransitions(recorder, next)

	if len(recorder.events) != 1 || recorder.events[0].Control != "volume-up" ||
		!recorder.events[0].Pressed {
		t.Fatalf("firmware side key events = %#v", recorder.events)
	}
}

// The session flag has to follow the routing decision the backend makes, which
// is OpenRequest.Firmware, and it has to clear when the input is released so a
// firmware session cannot leak its side keys into the next title.
func TestSideKeysFollowTheOpenedInput(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)

	shell := NewShell(inertSideKeyBackend{}, nil, "")
	if shell.sideKeysAvailable() {
		t.Fatal("an empty shell offers side keys")
	}
	shell.consumeBackendResult(backendResult{
		request: OpenRequest{Path: "phone", Firmware: true},
		info:    InputInfo{DisplayName: "phone", Format: "firmware-directory"},
	})
	if !shell.sideKeysAvailable() {
		t.Fatal("firmware session does not offer side keys")
	}
	if err := shell.releaseCurrentInput(); err != nil {
		t.Fatal(err)
	}
	if shell.sideKeysAvailable() {
		t.Fatal("side keys survived the released firmware input")
	}
	shell.consumeBackendResult(backendResult{
		request: OpenRequest{Path: "game.zip"},
		info:    InputInfo{DisplayName: "game.zip", Format: "java-archive"},
	})
	if shell.sideKeysAvailable() {
		t.Fatal("application session offers side keys")
	}
}
