package frontend

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type bindingDevice string

const (
	bindingDeviceKeyboard bindingDevice = "keyboard"
	bindingDeviceGamepad  bindingDevice = "gamepad"
)

type capturedGamepadButton struct {
	Gamepad ebiten.GamepadID
	Button  ebiten.StandardGamepadButton
}

type bindingCapture struct {
	Device         bindingDevice
	Control        string
	BlockedKeys    map[ebiten.Key]bool
	BlockedButtons map[capturedGamepadButton]bool
}

func (s *Shell) beginKeyboardBindingCapture(control string) {
	if !containsControl(keyboardControlOrder, control) {
		return
	}
	blocked := make(map[ebiten.Key]bool)
	for _, key := range inpututil.AppendPressedKeys(nil) {
		blocked[key] = true
	}
	s.bindingCapture = &bindingCapture{
		Device:      bindingDeviceKeyboard,
		Control:     control,
		BlockedKeys: blocked,
	}
	s.invalidateSettingsPanel()
	s.setStatus(controlDisplayName(control) + ": press a keyboard key (Esc cancels)")
}

func (s *Shell) beginGamepadBindingCapture(control string) {
	if !containsControl(controllerControlOrder, control) {
		return
	}
	blocked := make(map[capturedGamepadButton]bool)
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for _, button := range inpututil.AppendPressedStandardGamepadButtons(id, nil) {
			blocked[capturedGamepadButton{Gamepad: id, Button: button}] = true
		}
	}
	s.bindingCapture = &bindingCapture{
		Device:         bindingDeviceGamepad,
		Control:        control,
		BlockedButtons: blocked,
	}
	s.invalidateSettingsPanel()
	s.setStatus(controlDisplayName(control) + ": press a gamepad button (Esc cancels)")
}

// handleBindingCapture consumes input while a binding row is listening. It
// returns true even when no new key arrived so global shortcuts cannot fire.
func (s *Shell) handleBindingCapture() bool {
	capture := s.bindingCapture
	if capture == nil {
		return false
	}
	if s.panel == nil || s.panel.Kind != "settings" {
		s.bindingCapture = nil
		return false
	}

	for key := range capture.BlockedKeys {
		if !ebiten.IsKeyPressed(key) {
			delete(capture.BlockedKeys, key)
		}
	}
	for button := range capture.BlockedButtons {
		if !ebiten.IsStandardGamepadButtonPressed(button.Gamepad, button.Button) {
			delete(capture.BlockedButtons, button)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) &&
		!capture.BlockedKeys[ebiten.KeyEscape] {
		s.cancelBindingCapture("Binding capture canceled")
		return true
	}

	switch capture.Device {
	case bindingDeviceKeyboard:
		for _, key := range inpututil.AppendJustPressedKeys(nil) {
			if capture.BlockedKeys[key] {
				continue
			}
			if !isBindableKeyboardKey(key) {
				s.setStatus(keyboardKeyLabel(key) + " is reserved; press another key")
				continue
			}
			s.applyKeyboardBinding(capture.Control, key)
			return true
		}
	case bindingDeviceGamepad:
		for _, id := range ebiten.AppendGamepadIDs(nil) {
			if !ebiten.IsStandardGamepadLayoutAvailable(id) {
				continue
			}
			for _, button := range inpututil.AppendJustPressedStandardGamepadButtons(id, nil) {
				captured := capturedGamepadButton{Gamepad: id, Button: button}
				if capture.BlockedButtons[captured] {
					continue
				}
				option, ok := gamepadButtonOptionByButton(button)
				if !ok {
					continue
				}
				s.applyGamepadBinding(capture.Control, option)
				return true
			}
		}
	}
	return true
}

func (s *Shell) cancelBindingCapture(status string) {
	s.bindingCapture = nil
	s.invalidateSettingsPanel()
	s.setStatus(status)
}

func (s *Shell) applyKeyboardBinding(control string, key ebiten.Key) {
	swapped := ""
	s.updateControllerProfile(func(profile *ControllerProfile) {
		swapped = assignKeyboardBinding(profile, control, key)
	}, fmt.Sprintf(
		"%s keyboard binding: %s",
		controlDisplayName(control),
		keyboardKeyLabel(key),
	))
	s.bindingCapture = nil
	s.invalidateSettingsPanel()
	if swapped != "" {
		s.setStatus(fmt.Sprintf(
			"%s mapped to %s; swapped with %s",
			controlDisplayName(control),
			keyboardKeyLabel(key),
			controlDisplayName(swapped),
		))
	}
}

func (s *Shell) applyGamepadBinding(control string, option gamepadButtonOption) {
	swapped := ""
	s.updateControllerProfile(func(profile *ControllerProfile) {
		swapped = assignGamepadBinding(profile, control, option.ID)
	}, fmt.Sprintf(
		"%s gamepad binding: %s",
		controlDisplayName(control),
		option.Label,
	))
	s.bindingCapture = nil
	s.invalidateSettingsPanel()
	if swapped != "" {
		s.setStatus(fmt.Sprintf(
			"%s mapped to %s; swapped with %s",
			controlDisplayName(control),
			option.Label,
			controlDisplayName(swapped),
		))
	}
}

func assignKeyboardBinding(
	profile *ControllerProfile,
	control string,
	key ebiten.Key,
) string {
	if profile == nil || !containsControl(keyboardControlOrder, control) ||
		!isBindableKeyboardKey(key) {
		return ""
	}
	bindings := keyboardBindingIDsForProfile(*profile)
	nextID := key.String()
	previousID := bindings[control]
	swapped := ""
	for _, other := range keyboardControlOrder {
		if other == control || bindings[other] != nextID {
			continue
		}
		bindings[other] = previousID
		swapped = other
		break
	}
	bindings[control] = nextID
	profile.KeyboardProfile = "custom"
	profile.KeyboardBindings = bindings
	return swapped
}

func assignGamepadBinding(
	profile *ControllerProfile,
	control string,
	buttonID string,
) string {
	if profile == nil || !containsControl(controllerControlOrder, control) {
		return ""
	}
	if _, ok := gamepadButtonOptionByID(buttonID); !ok {
		return ""
	}
	bindings := make(map[string]string, len(controllerControlOrder))
	for _, binding := range gamepadBindingsForProfile(*profile) {
		bindings[binding.Control] = binding.ID
	}
	previousID := bindings[control]
	swapped := ""
	for _, other := range controllerControlOrder {
		if other == control || bindings[other] != buttonID {
			continue
		}
		bindings[other] = previousID
		swapped = other
		break
	}
	bindings[control] = buttonID
	profile.GamepadLayout = "custom"
	profile.GamepadBindings = bindings
	return swapped
}

func containsControl(controls []string, control string) bool {
	for _, candidate := range controls {
		if candidate == control {
			return true
		}
	}
	return false
}

func (s *Shell) keyboardBindingLabel(control string) string {
	for _, binding := range keyboardBindingsForProfile(s.controllerProfile()) {
		if binding.Control == control {
			return binding.Label
		}
	}
	return "Unassigned"
}

func (s *Shell) invalidateSettingsPanel() {
	if s.interfaceUI != nil {
		s.interfaceUI.panelSignature = ""
		// Captured Enter or Space must not also submit the previously focused
		// EbitenUI button in the same update.
		if s.interfaceUI.ui != nil {
			s.interfaceUI.ui.ClearFocus()
		}
	}
}

func captureMatches(
	capture *bindingCapture,
	device bindingDevice,
	control string,
) bool {
	return capture != nil && capture.Device == device && capture.Control == control
}

func bindingCaptureSignature(capture *bindingCapture) string {
	if capture == nil {
		return ""
	}
	return string(capture.Device) + ":" + capture.Control
}
