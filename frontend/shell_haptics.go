package frontend

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// gamepadRumbleHold is re-issued each active tick. XInput rumble is state-based
// and holds until changed, so a hold slightly longer than one 60 Hz frame keeps
// the motor alive between ticks without a visible gap.
const gamepadRumbleHold = 100 * time.Millisecond

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
	magnitude, active := hapticMagnitude(source.Haptics())
	if !active {
		s.stopHapticsIfActive()
		return
	}
	rising := !s.hapticActive
	s.driveGamepadRumble(magnitude)
	if rising {
		state := source.Haptics()
		ebiten.Vibrate(&ebiten.VibrateOptions{
			Duration:  state.Duration,
			Magnitude: magnitude,
		})
	}
	s.hapticActive = true
}

func (s *Shell) stopHapticsIfActive() {
	if !s.hapticActive {
		return
	}
	s.driveGamepadRumble(0)
	s.hapticActive = false
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
