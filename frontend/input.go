package frontend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type keyBinding struct {
	Control string
	Key     ebiten.Key
	Label   string
}

type gamepadBinding struct {
	Control string
	ID      string
	Button  ebiten.StandardGamepadButton
	Label   string
}

type gamepadButtonOption struct {
	ID     string
	Button ebiten.StandardGamepadButton
	Label  string
}

var gamepadButtonOptions = []gamepadButtonOption{
	{ID: "dpad-up", Button: ebiten.StandardGamepadButtonLeftTop, Label: "D-pad Up"},
	{ID: "dpad-down", Button: ebiten.StandardGamepadButtonLeftBottom, Label: "D-pad Down"},
	{ID: "dpad-left", Button: ebiten.StandardGamepadButtonLeftLeft, Label: "D-pad Left"},
	{ID: "dpad-right", Button: ebiten.StandardGamepadButtonLeftRight, Label: "D-pad Right"},
	{ID: "face-south", Button: ebiten.StandardGamepadButtonRightBottom, Label: "South face"},
	{ID: "face-east", Button: ebiten.StandardGamepadButtonRightRight, Label: "East face"},
	{ID: "face-west", Button: ebiten.StandardGamepadButtonRightLeft, Label: "West face"},
	{ID: "face-north", Button: ebiten.StandardGamepadButtonRightTop, Label: "North face"},
	{ID: "shoulder-left", Button: ebiten.StandardGamepadButtonFrontTopLeft, Label: "Left shoulder"},
	{ID: "shoulder-right", Button: ebiten.StandardGamepadButtonFrontTopRight, Label: "Right shoulder"},
	{ID: "trigger-left", Button: ebiten.StandardGamepadButtonFrontBottomLeft, Label: "Left trigger"},
	{ID: "trigger-right", Button: ebiten.StandardGamepadButtonFrontBottomRight, Label: "Right trigger"},
	{ID: "select", Button: ebiten.StandardGamepadButtonCenterLeft, Label: "Select / Back"},
	{ID: "start", Button: ebiten.StandardGamepadButtonCenterRight, Label: "Menu / Start"},
	{ID: "stick-left", Button: ebiten.StandardGamepadButtonLeftStick, Label: "Left stick click"},
	{ID: "stick-right", Button: ebiten.StandardGamepadButtonRightStick, Label: "Right stick click"},
	{ID: "guide", Button: ebiten.StandardGamepadButtonCenterCenter, Label: "Guide"},
}

var controllerControlOrder = []string{
	"up", "down", "left", "right",
	"ok", "back", "soft-left", "soft-right", "menu", "star", "hash",
}

var directionControlOrder = []string{"up", "down", "left", "right"}

var keyboardControlOrder = append(
	append([]string(nil), controllerControlOrder...),
	"num0", "num1", "num2", "num3", "num4",
	"num5", "num6", "num7", "num8", "num9",
)

func keyboardBindings(profile string) []keyBinding {
	directions := []keyBinding{
		{Control: "up", Key: ebiten.KeyArrowUp, Label: "Arrow Up"},
		{Control: "down", Key: ebiten.KeyArrowDown, Label: "Arrow Down"},
		{Control: "left", Key: ebiten.KeyArrowLeft, Label: "Arrow Left"},
		{Control: "right", Key: ebiten.KeyArrowRight, Label: "Arrow Right"},
	}
	if profile == "wasd" {
		directions = []keyBinding{
			{Control: "up", Key: ebiten.KeyW, Label: "W"},
			{Control: "down", Key: ebiten.KeyS, Label: "S"},
			{Control: "left", Key: ebiten.KeyA, Label: "A"},
			{Control: "right", Key: ebiten.KeyD, Label: "D"},
		}
	}
	bindings := append(directions,
		keyBinding{Control: "ok", Key: ebiten.KeyEnter, Label: "Enter"},
		keyBinding{Control: "back", Key: ebiten.KeyBackspace, Label: "Backspace"},
		keyBinding{Control: "soft-left", Key: ebiten.KeyQ, Label: "Q"},
		keyBinding{Control: "soft-right", Key: ebiten.KeyE, Label: "E"},
		keyBinding{Control: "menu", Key: ebiten.KeySpace, Label: "Space"},
		keyBinding{Control: "star", Key: ebiten.KeyComma, Label: ","},
		keyBinding{Control: "hash", Key: ebiten.KeyPeriod, Label: "."},
	)
	for number := 0; number <= 9; number++ {
		bindings = append(bindings, keyBinding{
			Control: fmt.Sprintf("num%d", number),
			Key:     ebiten.KeyDigit0 + ebiten.Key(number),
			Label:   fmt.Sprintf("%d", number),
		})
	}
	return bindings
}

func keyboardBindingsForProfile(profile ControllerProfile) []keyBinding {
	bindings := keyboardBindings(profile.KeyboardProfile)
	for index, binding := range bindings {
		id := profile.KeyboardBindings[binding.Control]
		key, ok := keyboardKeyByID(id)
		if !ok {
			continue
		}
		bindings[index].Key = key
		bindings[index].Label = keyboardKeyLabel(key)
	}
	return bindings
}

func keyboardBindingIDsForProfile(profile ControllerProfile) map[string]string {
	bindings := keyboardBindingsForProfile(profile)
	result := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		result[binding.Control] = binding.Key.String()
	}
	return result
}

func keyboardKeyByID(id string) (ebiten.Key, bool) {
	if id == "" {
		return 0, false
	}
	for key := ebiten.Key(0); key <= ebiten.KeyMax; key++ {
		if key.String() == id && isBindableKeyboardKey(key) {
			return key, true
		}
	}
	return 0, false
}

func keyboardKeyLabel(key ebiten.Key) string {
	switch key {
	case ebiten.KeyArrowUp:
		return "Arrow Up"
	case ebiten.KeyArrowDown:
		return "Arrow Down"
	case ebiten.KeyArrowLeft:
		return "Arrow Left"
	case ebiten.KeyArrowRight:
		return "Arrow Right"
	case ebiten.KeyControlLeft:
		return "Left Ctrl"
	case ebiten.KeyControlRight:
		return "Right Ctrl"
	case ebiten.KeyShiftLeft:
		return "Left Shift"
	case ebiten.KeyShiftRight:
		return "Right Shift"
	case ebiten.KeyAltLeft:
		return "Left Alt"
	case ebiten.KeyAltRight:
		return "Right Alt"
	case ebiten.KeyMetaLeft:
		return "Left Meta"
	case ebiten.KeyMetaRight:
		return "Right Meta"
	}
	return key.String()
}

func isBindableKeyboardKey(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyEscape,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
		ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
		ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight,
		ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8,
		ebiten.KeyF9, ebiten.KeyF10, ebiten.KeyF11:
		return false
	default:
		return key >= 0 && key <= ebiten.KeyMax && key.String() != ""
	}
}

func normalizeKeyboardBindingIDs(bindings map[string]string) map[string]string {
	if len(bindings) == 0 {
		return nil
	}
	validControls := make(map[string]bool, len(keyboardControlOrder))
	for _, control := range keyboardControlOrder {
		validControls[control] = true
	}
	result := make(map[string]string)
	for control, id := range bindings {
		if !validControls[control] {
			continue
		}
		if _, ok := keyboardKeyByID(id); ok {
			result[control] = id
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func gamepadBindings(layout string) []gamepadBinding {
	ids := defaultGamepadBindingIDs(layout)
	bindings := make([]gamepadBinding, 0, len(controllerControlOrder))
	for _, control := range controllerControlOrder {
		option, _ := gamepadButtonOptionByID(ids[control])
		bindings = append(bindings, gamepadBinding{
			Control: control,
			ID:      option.ID,
			Button:  option.Button,
			Label:   option.Label,
		})
	}
	return bindings
}

func defaultGamepadBindingIDs(layout string) map[string]string {
	ok := "face-south"
	back := "face-east"
	if layout == "swapped" {
		ok, back = back, ok
	}
	return map[string]string{
		"up":         "dpad-up",
		"down":       "dpad-down",
		"left":       "dpad-left",
		"right":      "dpad-right",
		"ok":         ok,
		"back":       back,
		"soft-left":  "shoulder-left",
		"soft-right": "shoulder-right",
		"menu":       "start",
		"star":       "face-west",
		"hash":       "face-north",
	}
}

func gamepadBindingsForProfile(profile ControllerProfile) []gamepadBinding {
	defaults := defaultGamepadBindingIDs(profile.GamepadLayout)
	bindings := make([]gamepadBinding, 0, len(controllerControlOrder))
	for _, control := range controllerControlOrder {
		id := defaults[control]
		if configured := profile.GamepadBindings[control]; configured != "" {
			id = configured
		}
		option, ok := gamepadButtonOptionByID(id)
		if !ok {
			option, _ = gamepadButtonOptionByID(defaults[control])
		}
		bindings = append(bindings, gamepadBinding{
			Control: control,
			ID:      option.ID,
			Button:  option.Button,
			Label:   option.Label,
		})
	}
	return bindings
}

func gamepadButtonOptionByID(id string) (gamepadButtonOption, bool) {
	for _, option := range gamepadButtonOptions {
		if option.ID == id {
			return option, true
		}
	}
	return gamepadButtonOption{}, false
}

func gamepadButtonOptionByButton(button ebiten.StandardGamepadButton) (gamepadButtonOption, bool) {
	for _, option := range gamepadButtonOptions {
		if option.Button == button {
			return option, true
		}
	}
	return gamepadButtonOption{}, false
}

func normalizeGamepadBindingIDs(bindings map[string]string) map[string]string {
	if len(bindings) == 0 {
		return nil
	}
	validControls := make(map[string]bool, len(controllerControlOrder))
	for _, control := range controllerControlOrder {
		validControls[control] = true
	}
	result := make(map[string]string)
	for control, id := range bindings {
		if !validControls[control] {
			continue
		}
		if _, ok := gamepadButtonOptionByID(id); ok {
			result[control] = id
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func controllerProfileSignature(profile ControllerProfile) string {
	parts := []string{
		profile.KeyboardProfile,
		fmt.Sprintf("%t", profile.GamepadEnabled),
		profile.GamepadLayout,
		fmt.Sprintf("%t", profile.GamepadAnalog),
		fmt.Sprintf("%d", profile.GamepadDeadzone),
	}
	for _, control := range keyboardControlOrder {
		parts = append(parts, "key:"+control+"="+profile.KeyboardBindings[control])
	}
	for _, control := range controllerControlOrder {
		parts = append(parts, control+"="+profile.GamepadBindings[control])
	}
	return strings.Join(parts, "|")
}

func gamepadConnectionSignature() string {
	var devices []string
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		devices = append(devices, fmt.Sprintf(
			"%d:%s:%t",
			id,
			ebiten.GamepadName(id),
			ebiten.IsStandardGamepadLayoutAvailable(id),
		))
	}
	if len(devices) == 0 {
		return "none"
	}
	return strings.Join(devices, "|")
}

func gamepadConnectionLabel(languages ...Language) string {
	language := LanguageEnglish
	if len(languages) > 0 {
		language = languages[0]
	}
	ids := ebiten.AppendGamepadIDs(nil)
	if len(ids) == 0 {
		return translate(language, "None")
	}
	standard := 0
	for _, id := range ids {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			standard++
		}
	}
	if len(ids) == 1 {
		name := ebiten.GamepadName(ids[0])
		if name == "" {
			name = translate(language, "Controller")
		}
		if standard == 0 {
			return translatef(
				language,
				"%s (unsupported)",
				shorten(name, 18),
			)
		}
		return shorten(name, 22)
	}
	return translatef(
		language,
		"%d connected / %d standard",
		len(ids),
		standard,
	)
}

func customGamepadMappingsPath() (string, error) {
	path, err := settingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "gamecontrollerdb.txt"), nil
}

func loadCustomGamepadMappings() (bool, error) {
	path, err := customGamepadMappingsPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, errors.New("gamecontrollerdb.txt is empty")
	}
	return ebiten.UpdateStandardGamepadLayoutMappings(string(data))
}

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

func isDirectionControl(control string) bool {
	for _, direction := range directionControlOrder {
		if control == direction {
			return true
		}
	}
	return false
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

func applyAnalogDirections(state map[string]bool, horizontal, vertical, deadzone float64) {
	if horizontal <= -deadzone {
		state["left"] = true
	}
	if horizontal >= deadzone {
		state["right"] = true
	}
	if vertical <= -deadzone {
		state["up"] = true
	}
	if vertical >= deadzone {
		state["down"] = true
	}
}
