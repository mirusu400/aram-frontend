package frontend

import "testing"

type textInputHintBackend struct {
	NullBackend
	active bool
}

func (backend textInputHintBackend) TextInputActive() bool { return backend.active }

func TestFocusedGuestTextInputRevealsVirtualKeypad(t *testing.T) {
	shell := &Shell{
		backend:  textInputHintBackend{active: true},
		settings: defaultSettings(),
	}
	shell.settings.ShowVirtualKeypad = false
	if !shell.virtualKeypadVisible() {
		t.Fatal("focused guest text input did not reveal the virtual keypad")
	}

	shell.backend = textInputHintBackend{active: false}
	if shell.virtualKeypadVisible() {
		t.Fatal("inactive text input forced the virtual keypad to remain visible")
	}

	shell.settings.ShowVirtualKeypad = true
	if !shell.virtualKeypadVisible() {
		t.Fatal("manual virtual keypad setting stopped working")
	}
}
