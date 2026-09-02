package frontend

import (
	"image"
	"math"
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
			if binding.ID == "" {
				continue
			}
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
	if s.guestInputAllowed() && s.activeMenu < 0 && !s.touchLayoutEditing {
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
		s.collectHostControlState(next)
	}
	s.dropUnavailableControls(next)
	s.queueInputTransitions(backend, next)
}

// sideKeysAvailable reports whether the guest owns the handset side keys. Only
// whole-phone firmware reads the volume rocker; an application title is started
// by a phone that keeps those keys for itself, so the keypad hides them rather
// than offering a button whose press goes nowhere.
func (s *Shell) sideKeysAvailable() bool {
	return s.firmwareSession
}

// sideKeyControls are the handset side keys a keyboard or gamepad binding can
// still name while an application title is loaded.
var sideKeyControls = [...]string{"volume-up", "volume-down"}

// dropUnavailableControls removes controls the current session cannot deliver
// before the transition pass runs, so a control held when the session changed
// is released rather than left stuck pressed in the backend.
func (s *Shell) dropUnavailableControls(next map[string]bool) {
	if s.sideKeysAvailable() {
		return
	}
	for _, control := range sideKeyControls {
		delete(next, control)
	}
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
	// The circular pad tracks its own touch: a live direction while the thumb
	// steers, and a brief OK pulse after a center tap lifts.
	if s.padDir != "" {
		state[s.padDir] = true
	}
	if s.padOKPulse > 0 {
		state["ok"] = true
	}
}

// SetHostControl records one control the native host holds or releases, such
// as a button on a second physical panel driven by its own Activity. The host
// calls this from its UI thread while handleMappedInput samples it on the game
// loop, so the held set is mutex-guarded. The control names are the same ones
// the on-screen deck uses (dpad, soft keys, num0-9, ...), so a host press runs
// through the identical transition and availability path.
func (s *Shell) SetHostControl(control string, pressed bool) {
	if control == "" {
		return
	}
	s.hostControlMu.Lock()
	defer s.hostControlMu.Unlock()
	if s.hostControls == nil {
		s.hostControls = make(map[string]bool)
	}
	if pressed {
		s.hostControls[control] = true
	} else {
		delete(s.hostControls, control)
	}
}

func (s *Shell) collectHostControlState(state map[string]bool) {
	s.hostControlMu.Lock()
	defer s.hostControlMu.Unlock()
	for control := range s.hostControls {
		state[control] = true
	}
}

// SetSecondaryKeypadActive tells the shell a second physical panel is showing
// the keypad as its own surface. While active the on-screen control deck and
// keypad are suppressed so the game panel is unobstructed, and input arrives
// through SetHostControl. Turning it off releases any control the host still
// held so nothing is left stuck pressed.
func (s *Shell) SetSecondaryKeypadActive(active bool) {
	s.secondaryKeypad.Store(active)
	if !active {
		s.hostControlMu.Lock()
		s.hostControls = nil
		s.hostControlMu.Unlock()
	}
}

// SetControllerConnected tells the shell whether the host sees a physical
// controller. On a touch layout the built-in on-screen controls step aside for
// it (see onScreenControlsHidden), because the player already has real buttons.
func (s *Shell) SetControllerConnected(connected bool) {
	s.controllerConnected.Store(connected)
}

// onScreenControlsHidden reports whether the built-in touch deck and keypad
// should give way. A second physical panel takes them over entirely; on a
// single screen a connected controller replaces them unless the player has
// asked to keep them. It only applies to touch-layout platforms - a desktop
// window keeps its own deck regardless of what a host declares.
func (s *Shell) onScreenControlsHidden() bool {
	if !platformUsesTouchLayout() {
		return false
	}
	if s.secondaryKeypad.Load() {
		return true
	}
	return s.controllerConnected.Load() && !s.settings.ShowControlsWithPad
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
			if binding.ID == "" {
				continue
			}
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
	if s.touchLayoutEditing {
		s.handleTouchLayoutEditTouches()
		return
	}
	// Age any pending OK pulse before this tick can top it up on release, so a
	// tap that lands this frame still reads OK this frame.
	if s.padOKPulse > 0 {
		s.padOKPulse--
	}
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		if s.padTouchActive && id == s.padTouchID {
			s.releaseCircularPad()
			continue
		}
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
				if control, ok := focusControlAtScaled(x, y, width, height, s.touchScale()); ok {
					s.touchControls[id] = control
				}
			}
			continue
		}
		if s.touchChromeToggleAvailable() {
			width, _ := s.viewportSize()
			bounds := touchChromeToggleBounds(width, s.touchChromeHiddenActive())
			if pointInRect(x, y, bounds) {
				s.toggleTouchChrome()
				continue
			}
		}
		if s.guestInputAllowed() && s.activeMenu < 0 &&
			!s.onScreenControlsHidden() {
			if s.touchDpadCircular() && !s.padTouchActive &&
				s.circularPadContains(x, y) {
				s.padTouchActive = true
				s.padTouchID = id
				s.padTouchMoved = false
				s.padDir = ""
				s.padKnob = image.Point{}
				continue
			}
			if control, ok := s.touchControlAt(x, y); ok {
				s.touchControls[id] = control
				continue
			}
		}
		if s.touchChromeHiddenActive() {
			// The chrome is hidden; stray taps must not reach it.
			continue
		}
		if s.interfaceUI == nil {
			s.handlePointerPress(x, y)
		}
	}
	s.sampleCircularPad()
}

// circularPadContains reports whether a press falls on the round pad. The whole
// bounding square of the directional cluster counts, not just the inscribed
// circle, so a thumb that lands near a corner still steers rather than leaking
// to whatever sat under the cross.
func (s *Shell) circularPadContains(x, y int) bool {
	width, height := s.viewportSize()
	metrics := touchDeckMetricsFor(width, height, s.touchLayoutOptions())
	center, radius := circularPadCircle(metrics)
	return pointInRect(x, y, rectAt(
		center.X-radius, center.Y-radius, radius*2, radius*2,
	))
}

// sampleCircularPad reads the live position of the pad's touch each tick and
// resolves it to a held direction, tracking whether the thumb ever left the
// deadzone so a still tap can be told from a drag on release. A touch that
// ended without a release event (a dropped finger) is released here.
func (s *Shell) sampleCircularPad() {
	if !s.padTouchActive {
		return
	}
	if !touchIDActive(s.padTouchID) {
		s.releaseCircularPad()
		return
	}
	x, y := ebiten.TouchPosition(s.padTouchID)
	width, height := s.viewportSize()
	metrics := touchDeckMetricsFor(width, height, s.touchLayoutOptions())
	center, radius := circularPadCircle(metrics)
	dx := float64(x - center.X)
	dy := float64(y - center.Y)
	dist := math.Hypot(dx, dy)
	if dist < float64(radius)*circularPadDeadzoneRatio {
		// Resting at center: no direction, and still a candidate for an OK tap.
		s.padDir = ""
		s.padKnob = image.Point{}
		return
	}
	s.padTouchMoved = true
	s.padDir = resolvePadDirection4(dx, dy)
	// Keep the drawn knob inside the well however far the thumb travels.
	limit := float64(radius) * (1 - circularPadKnobRatio)
	if dist > limit {
		dx *= limit / dist
		dy *= limit / dist
	}
	s.padKnob = image.Pt(int(dx), int(dy))
}

// releaseCircularPad ends the pad touch. A release that never left the
// deadzone was a center tap, so it arms a short OK pulse; a drag just stops.
func (s *Shell) releaseCircularPad() {
	if s.padTouchActive && !s.padTouchMoved {
		s.padOKPulse = circularPadOKPulseFrames
	}
	s.padTouchActive = false
	s.padTouchMoved = false
	s.padDir = ""
	s.padKnob = image.Point{}
}

// touchIDActive reports whether a touch is still down this tick.
func touchIDActive(id ebiten.TouchID) bool {
	for _, active := range ebiten.AppendTouchIDs(nil) {
		if active == id {
			return true
		}
	}
	return false
}

func (s *Shell) touchControlAt(x, y int) (string, bool) {
	width, height := s.viewportSize()
	button, ok := touchButtonAtWithOptions(x, y, width, height, s.touchLayoutOptions())
	if !ok {
		return "", false
	}
	return button.Control, true
}

func (s *Shell) collectVirtualKeypadState(state map[string]bool) {
	if !s.virtualKeypadVisible() {
		return
	}
	width, height := s.viewportSize()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if control, ok := virtualKeypadControlAtSize(
			x, y, width, height, s.sideKeysAvailable(),
		); ok {
			state[control] = true
		}
	}
	for _, id := range ebiten.AppendTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if control, ok := virtualKeypadControlAtSize(
			x, y, width, height, s.sideKeysAvailable(),
		); ok {
			state[control] = true
		}
	}
}
