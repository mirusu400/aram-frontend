package frontend

import "strings"

func (s *Shell) cycleKeyboardProfile() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		if profile.KeyboardProfile == "default" {
			profile.KeyboardProfile = "wasd"
		} else {
			profile.KeyboardProfile = "default"
		}
		profile.KeyboardBindings = nil
	}, "Keyboard profile updated")
}

func (s *Shell) toggleVirtualKeypad() {
	s.settings.ShowVirtualKeypad = !s.settings.ShowVirtualKeypad
	s.saveControllerSettings(s.trf(
		"Virtual keypad: %s",
		s.tr(onOff(s.settings.ShowVirtualKeypad)),
	))
}

// toggleShowControlsWithPad decides whether the on-screen touch controls stay
// up while a physical controller is connected. Off - the default - hides them
// so the handset's real buttons play without the deck in the way.
func (s *Shell) toggleShowControlsWithPad() {
	s.settings.ShowControlsWithPad = !s.settings.ShowControlsWithPad
	s.saveControllerSettings(s.trf(
		"On-screen controls with a controller: %s",
		s.tr(onOff(s.settings.ShowControlsWithPad)),
	))
}

func (s *Shell) toggleVibration() {
	s.settings.VibrationEnabled = !s.settings.VibrationEnabled
	if !s.settings.VibrationEnabled {
		s.stopHapticsIfActive()
	}
	s.saveControllerSettings(s.trf(
		"Vibration: %s",
		s.tr(onOff(s.settings.VibrationEnabled)),
	))
}

func (s *Shell) toggleGamepadEnabled() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadEnabled = !profile.GamepadEnabled
	}, "Gamepad input updated")
}

func (s *Shell) cycleGamepadLayout() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		if profile.GamepadLayout == "standard" {
			profile.GamepadLayout = "swapped"
		} else {
			profile.GamepadLayout = "standard"
		}
		delete(profile.GamepadBindings, "ok")
		delete(profile.GamepadBindings, "back")
	}, "Gamepad confirm/back layout updated")
}

func (s *Shell) toggleGamepadAnalog() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadAnalog = !profile.GamepadAnalog
	}, "Analog directions updated")
}

func (s *Shell) cycleGamepadDeadzone() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadDeadzone += 5
		if profile.GamepadDeadzone > 50 {
			profile.GamepadDeadzone = 15
		}
	}, "Gamepad dead zone updated")
}

func (s *Shell) resetControllerBindings() {
	s.bindingCapture = nil
	key := s.controllerProfileKey()
	if key != "" {
		delete(s.settings.TitleControllers, key)
	} else {
		s.settings.setGlobalControllerProfile(defaultSettings().globalControllerProfile())
	}
	s.saveControllerSettings("Controller bindings reset")
}

func (s *Shell) reloadGamepadMappings() {
	applied, err := loadCustomGamepadMappings()
	if err != nil {
		s.setStatus(s.tr("Controller database: ") + err.Error())
		return
	}
	s.gamepadMappingsLoaded = applied
	if !applied {
		path, pathErr := customGamepadMappingsPath()
		if pathErr != nil {
			s.setStatus(s.tr("Controller database: ") + pathErr.Error())
			return
		}
		s.setStatus(s.trf(
			"Controller database: no mapping file at %s",
			path,
		))
		return
	}
	s.setStatus(s.tr("Custom controller database reloaded"))
}

func (s *Shell) togglePerTitleControls() {
	if s.settings.PerTitleControls {
		s.settings.PerTitleControls = false
		s.saveControllerSettings("Controller profile scope: global")
		return
	}
	key := s.titleControllerKey()
	if key == "" {
		s.setStatus(s.tr("Controller profile: open an identified title first"))
		return
	}
	s.settings.PerTitleControls = true
	if _, ok := s.settings.TitleControllers[key]; !ok {
		s.settings.TitleControllers[key] = s.settings.globalControllerProfile()
	}
	s.saveControllerSettings("Controller profile scope: this title")
}

func (s *Shell) controllerProfile() ControllerProfile {
	global := s.settings.globalControllerProfile()
	key := s.controllerProfileKey()
	if key == "" {
		return global
	}
	profile, ok := s.settings.TitleControllers[key]
	if !ok {
		return global
	}
	profile.normalize()
	return profile
}

func (s *Shell) updateControllerProfile(update func(*ControllerProfile), status string) {
	profile := s.controllerProfile()
	update(&profile)
	profile.normalize()
	if key := s.controllerProfileKey(); key != "" {
		if s.settings.TitleControllers == nil {
			s.settings.TitleControllers = make(map[string]ControllerProfile)
		}
		s.settings.TitleControllers[key] = profile
	} else {
		s.settings.setGlobalControllerProfile(profile)
	}
	s.saveControllerSettings(status)
}

func (s *Shell) titleControllerKey() string {
	return titleSettingsKey(s.input)
}

func (s *Shell) controllerProfileKey() string {
	if !s.settings.PerTitleControls {
		return ""
	}
	return s.titleControllerKey()
}

func (s *Shell) controllerProfileScopeLabel() string {
	if s.controllerProfileKey() != "" {
		return "This title"
	}
	return "Global"
}

func (s *Shell) gamepadBindingLabel(control string) string {
	for _, binding := range gamepadBindingsForProfile(s.controllerProfile()) {
		if binding.Control == control {
			if binding.Label == "" {
				return "Unassigned"
			}
			return binding.Label
		}
	}
	return "Unassigned"
}

func (s *Shell) saveControllerSettings(status string) {
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Controller settings: ") + err.Error())
		return
	}
	s.setStatus(s.tr(status))
}

func gamepadLayoutLabel(layout string) string {
	switch layout {
	case "swapped":
		return "East confirm"
	case "custom":
		return "Custom"
	default:
		return "South confirm"
	}
}

func keyboardProfileLabel(profile string) string {
	switch profile {
	case "wasd":
		return "WASD"
	case "custom":
		return "Custom"
	default:
		return "Arrow keys"
	}
}

func controlDisplayName(control string) string {
	switch control {
	case "up":
		return "Up"
	case "down":
		return "Down"
	case "left":
		return "Left"
	case "right":
		return "Right"
	case "ok":
		return "Confirm"
	case "back":
		return "C / Clear"
	case "send":
		return "Call"
	case "end":
		return "End call"
	case "soft-left":
		return "Soft key left"
	case "soft-right":
		return "Right soft key / Cancel"
	case "volume-up":
		return "Volume up"
	case "volume-down":
		return "Volume down"
	case "menu":
		return "Menu"
	case "star":
		return "Star"
	case "hash":
		return "Hash"
	default:
		if strings.HasPrefix(control, "num") && len(control) == 4 {
			return "Number " + strings.TrimPrefix(control, "num")
		}
		return control
	}
}
