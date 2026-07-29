package frontend

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type keyBinding struct {
	Control string
	Key     ebiten.Key
	Label   string
}

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

func (s *Shell) handleMappedInput() {
	backend, ok := s.backend.(InputBackend)
	if !ok || s.input == nil {
		return
	}

	next := make(map[string]bool)
	if s.panel == nil && s.activeMenu < 0 {
		modifierPressed := ebiten.IsKeyPressed(ebiten.KeyControl) ||
			ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyControlRight) ||
			ebiten.IsKeyPressed(ebiten.KeyAlt) ||
			ebiten.IsKeyPressed(ebiten.KeyAltLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyAltRight)
		if !modifierPressed {
			for _, binding := range keyboardBindings(s.settings.KeyboardProfile) {
				if ebiten.IsKeyPressed(binding.Key) {
					next[binding.Control] = true
				}
			}
		}
		s.collectGamepadState(next)
		s.collectTouchState(next)
	}

	allControls := make(map[string]bool, len(next)+len(s.controlState))
	for control := range next {
		allControls[control] = true
	}
	for control := range s.controlState {
		allControls[control] = true
	}
	for control := range allControls {
		pressed := next[control]
		if s.controlState[control] == pressed {
			continue
		}
		if err := backend.QueueInput(InputEvent{
			Control: control,
			Pressed: pressed,
			At:      timeSince(s.startedAt),
		}); err != nil {
			s.setStatus("Input " + control + ": " + err.Error())
		}
	}
	s.controlState = next
}

func (s *Shell) collectTouchState(state map[string]bool) {
	for _, control := range s.touchControls {
		state[control] = true
	}
}

func (s *Shell) collectGamepadState(state map[string]bool) {
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		gamepadBindings := map[string]ebiten.StandardGamepadButton{
			"up":         ebiten.StandardGamepadButtonLeftTop,
			"down":       ebiten.StandardGamepadButtonLeftBottom,
			"left":       ebiten.StandardGamepadButtonLeftLeft,
			"right":      ebiten.StandardGamepadButtonLeftRight,
			"ok":         ebiten.StandardGamepadButtonRightBottom,
			"back":       ebiten.StandardGamepadButtonRightRight,
			"soft-left":  ebiten.StandardGamepadButtonFrontTopLeft,
			"soft-right": ebiten.StandardGamepadButtonFrontTopRight,
			"menu":       ebiten.StandardGamepadButtonCenterRight,
		}
		for control, button := range gamepadBindings {
			if ebiten.IsStandardGamepadButtonPressed(id, button) {
				state[control] = true
			}
		}
	}
}

func timeSince(startedAt time.Time) time.Duration {
	elapsed := time.Since(startedAt)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}
