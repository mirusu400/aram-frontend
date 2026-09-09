package frontend

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// gamepadRumbleHold is re-issued each active tick. XInput rumble is state-based
// and holds until changed, so a hold slightly longer than one 60 Hz frame keeps
// the motor alive between ticks without a visible gap.
const gamepadRumbleHold = 100 * time.Millisecond

// hapticPreview* describe the confirmation buzz played when vibration is
// switched on in settings. It is short and mid-strength: long enough to feel on
// a phone, weak enough not to read as a title's own rumble.
const (
	hapticPreviewDuration  = 120 * time.Millisecond
	hapticPreviewMagnitude = 0.6
)

// updateHaptics polls the backend's vibration request once per tick and drives
// the host rumble motors and phone vibrator. Gamepad rumble is state-based and
// re-issued while active; the phone vibrator runs a whole pulse on its own, so
// it is triggered only on the rising edge.
func (s *Shell) updateHaptics() {
	if !s.settings.VibrationEnabled || s.backend.State() != StateRunning {
		s.stopHapticsIfActive()
		return
	}
	source, ok := s.backend.(HapticsBackend)
	if !ok {
		s.stopHapticsIfActive()
		return
	}
	state := source.Haptics()
	magnitude, active := hapticMagnitude(state)
	if !active {
		s.stopHapticsIfActive()
		return
	}
	newPulse := startsHapticPulse(s.hapticActive, s.hapticRemaining, state.Duration)
	s.driveGamepadRumble(magnitude)
	if newPulse {
		ebiten.Vibrate(&ebiten.VibrateOptions{
			Duration:  state.Duration,
			Magnitude: magnitude,
		})
	}
	s.hapticActive = true
	s.hapticRemaining = state.Duration
}

func (s *Shell) stopHapticsIfActive() {
	if !s.hapticActive {
		return
	}
	s.driveGamepadRumble(0)
	s.hapticActive = false
	s.hapticRemaining = 0
}

// driveGamepadRumble sets both motors on every connected standard-layout pad. A
// zero magnitude stops them and is always delivered; a non-zero magnitude
// respects the profile's gamepad toggle so a controller disabled for input does
// not buzz.
func (s *Shell) driveGamepadRumble(magnitude float64) {
	stopping := magnitude <= 0
	if !stopping && !s.controllerProfile().GamepadEnabled {
		return
	}
	duration := gamepadRumbleHold
	if stopping {
		duration = 0
	}
	vibrateGamepads(magnitude, duration)
}

// previewHaptics plays one confirmation pulse so switching vibration on proves
// the motor works without waiting for a title to request one. It ignores the
// guest request and the machine state; the settings toggle is the trigger.
func (s *Shell) previewHaptics() {
	magnitude, duration, ok := hapticPreviewPulse(s.settings.VibrationEnabled)
	if !ok {
		return
	}
	ebiten.Vibrate(&ebiten.VibrateOptions{
		Duration:  duration,
		Magnitude: magnitude,
	})
	if !s.controllerProfile().GamepadEnabled {
		return
	}
	vibrateGamepads(magnitude, duration)
}

// hapticPreviewPulse reports the confirmation pulse for a toggle result. It is
// pure so the "only when switched on" rule can be tested without motors.
func hapticPreviewPulse(enabled bool) (float64, time.Duration, bool) {
	if !enabled {
		return 0, 0, false
	}
	return hapticPreviewMagnitude, hapticPreviewDuration, true
}

// vibrateGamepads sets both motors on every connected standard-layout pad.
func vibrateGamepads(magnitude float64, duration time.Duration) {
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		ebiten.VibrateGamepad(id, &ebiten.VibrateGamepadOptions{
			Duration:        duration,
			StrongMagnitude: magnitude,
			WeakMagnitude:   magnitude,
		})
	}
}

// startsHapticPulse identifies a fresh guest request even when it immediately
// follows the previous request without an idle frame. Remaining time normally
// falls every tick, so an increase means the guest restarted or extended the
// pulse and the phone vibrator must be triggered again.
func startsHapticPulse(active bool, previousRemaining, remaining time.Duration) bool {
	return !active || remaining > previousRemaining
}

// hapticMagnitude maps a vibration request to a 0..1 motor magnitude and
// whether it should actuate. It is pure so the mapping can be tested without a
// running game or host motor.
func hapticMagnitude(state HapticsState) (float64, bool) {
	if state.Level == 0 || state.Duration <= 0 {
		return 0, false
	}
	magnitude := float64(state.Level) / 100
	if magnitude > 1 {
		magnitude = 1
	}
	return magnitude, true
}
