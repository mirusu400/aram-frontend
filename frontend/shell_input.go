package frontend

import (
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (s *Shell) gamepadActivityLabel() string {
	profile := s.controllerProfile()
	if !profile.GamepadEnabled {
		return s.tr("Disabled")
	}
	var active []string
	seen := make(map[string]bool)
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for _, binding := range gamepadBindingsForProfile(profile) {
			if ebiten.IsStandardGamepadButtonPressed(id, binding.Button) && !seen[binding.Control] {
				seen[binding.Control] = true
				active = append(active, s.tr(controlDisplayName(binding.Control)))
			}
		}
		if profile.GamepadAnalog {
			state := make(map[string]bool)
			applyAnalogDirections(
				state,
				ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal),
				ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical),
				float64(profile.GamepadDeadzone)/100,
			)
			for _, control := range []string{"up", "down", "left", "right"} {
				if state[control] && !seen[control] {
					seen[control] = true
					active = append(active, s.tr(controlDisplayName(control)))
				}
			}
		}
	}
	if len(active) == 0 {
		return s.tr("Idle")
	}
	return strings.Join(active, " + ")
}

func (s *Shell) handleMappedInput() {
	backend, ok := s.backend.(InputBackend)
	if !ok || s.input == nil {
		return
	}

	next := make(map[string]bool)
	if s.guestInputAllowed() && s.activeMenu < 0 {
		profile := s.controllerProfile()
		modifierPressed := ebiten.IsKeyPressed(ebiten.KeyControl) ||
			ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyControlRight) ||
			ebiten.IsKeyPressed(ebiten.KeyAlt) ||
			ebiten.IsKeyPressed(ebiten.KeyAltLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyAltRight)
		if !modifierPressed {
			for _, binding := range keyboardBindingsForProfile(profile) {
				if ebiten.IsKeyPressed(binding.Key) {
					next[binding.Control] = true
				}
			}
		}
		s.collectGamepadState(next, profile)
		s.collectTouchState(next)
		s.collectVirtualKeypadState(next)
	}
	s.queueInputTransitions(backend, next)
}

func (s *Shell) queueInputTransitions(backend InputBackend, next map[string]bool) {
	next = s.atomicDirectionalState(next)
	if s.controlState == nil {
		s.controlState = make(map[string]bool)
	}

	allControls := make(map[string]bool, len(next)+len(s.controlState))
	for control := range next {
		allControls[control] = true
	}
	for control := range s.controlState {
		allControls[control] = true
	}
	controls := make([]string, 0, len(allControls))
	for control := range allControls {
		controls = append(controls, control)
	}
	sort.Strings(controls)

	// Deliver releases first so changing direction can never leave the old
	// and new direction pressed together in the backend, even momentarily.
	for _, pressed := range []bool{false, true} {
		for _, control := range controls {
			if next[control] != pressed || s.controlState[control] == pressed {
				continue
			}
			if pressed && isDirectionControl(control) && s.hasPressedDirection() {
				// A failed release is retried on the next update. Do not press its
				// replacement until the backend has accepted that release.
				continue
			}
			if err := backend.QueueInput(InputEvent{
				Control: control,
				Pressed: pressed,
				// Host input is sampled independently from the emulated clock. A
				// zero timestamp asks the backend to anchor the transition at its
				// current guest time instead of scheduling it in the future.
				At: 0,
			}); err != nil {
				s.setStatus(s.trf(
					"Input %s: %s",
					s.tr(controlDisplayName(control)),
					err.Error(),
				))
				continue
			}
			if pressed {
				s.controlState[control] = true
			} else {
				delete(s.controlState, control)
			}
		}
	}
}

func (s *Shell) atomicDirectionalState(next map[string]bool) map[string]bool {
	result := make(map[string]bool, len(next))
	for control, pressed := range next {
		if !isDirectionControl(control) {
			result[control] = pressed
		}
	}

	held := make(map[string]bool, len(s.directionPressOrder))
	order := make([]string, 0, len(directionControlOrder))
	for _, control := range s.directionPressOrder {
		if next[control] {
			order = append(order, control)
			held[control] = true
		}
	}
	for _, control := range directionControlOrder {
		if next[control] && !held[control] {
			order = append(order, control)
		}
	}
	s.directionPressOrder = order
	if len(order) != 0 {
		result[order[len(order)-1]] = true
	}
	return result
}

func (s *Shell) hasPressedDirection() bool {
	for _, control := range directionControlOrder {
		if s.controlState[control] {
			return true
		}
	}
	return false
}

func (s *Shell) collectTouchState(state map[string]bool) {
	for _, control := range s.touchControls {
		state[control] = true
	}
}

func (s *Shell) collectGamepadState(state map[string]bool, profile ControllerProfile) {
	if !profile.GamepadEnabled {
		return
	}
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for _, binding := range gamepadBindingsForProfile(profile) {
			if ebiten.IsStandardGamepadButtonPressed(id, binding.Button) {
				state[binding.Control] = true
			}
		}
		if profile.GamepadAnalog {
			applyAnalogDirections(
				state,
				ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal),
				ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical),
				float64(profile.GamepadDeadzone)/100,
			)
		}
	}
}

func (s *Shell) handleTouch() {
	if !platformUsesTouchLayout() {
		return
	}
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		delete(s.touchControls, id)
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if s.focusMode {
			width, height := s.viewportSize()
			if pointInRect(x, y, focusExitBoundsFor(width, height)) {
				s.toggleFocusMode()
				continue
			}
			if s.guestInputAllowed() {
				if control, ok := focusControlAtSize(x, y, width, height); ok {
					s.touchControls[id] = control
				}
			}
			continue
		}
		if s.guestInputAllowed() && s.activeMenu < 0 {
			if control, ok := s.touchControlAt(x, y); ok {
				s.touchControls[id] = control
				continue
			}
		}
		if s.panel == nil {
			if index, ok := s.touchNavigationAt(x, y); ok {
				if s.activeMenu == index {
					s.activeMenu = -1
				} else {
					s.activeMenu = index
				}
				continue
			}
		}
		if s.interfaceUI == nil {
			s.handlePointerPress(x, y)
		}
	}
}

func (s *Shell) touchNavigationAt(x, y int) (int, bool) {
	width, height := s.viewportSize()
	return touchNavigationAtSize(x, y, width, height)
}

func (s *Shell) touchControlAt(x, y int) (string, bool) {
	width, height := s.viewportSize()
	return touchControlAtSize(x, y, width, height)
}

func (s *Shell) collectVirtualKeypadState(state map[string]bool) {
	if !s.virtualKeypadVisible() {
		return
	}
	width, height := s.viewportSize()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if control, ok := virtualKeypadControlAtSize(x, y, width, height); ok {
			state[control] = true
		}
	}
	for _, id := range ebiten.AppendTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if control, ok := virtualKeypadControlAtSize(x, y, width, height); ok {
			state[control] = true
		}
	}
}
