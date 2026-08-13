package frontend

import (
	"context"
	"fmt"
	"time"
)

const (
	// framePacingFallbackQuantum is used when a backend cannot report how much
	// guest time one RunFrame advances.
	framePacingFallbackQuantum = time.Second / 60
	// framePacingCatchUpLimit bounds how much real time a single Update may
	// hand to the guest. Dragging the window, hitting a breakpoint, or closing
	// a laptop lid otherwise banks that whole gap and the title fast-forwards
	// through it on resume.
	framePacingCatchUpLimit = 250 * time.Millisecond
	// framePacingQuantaPerTick bounds how many quanta one Update may issue and
	// how deep the backlog may grow. When the host genuinely cannot keep up the
	// guest skips ahead the way a handset drops frames, rather than building a
	// backlog it later works off as a fast-forward.
	framePacingQuantaPerTick = 8
	// framePacingSampleWindow is how long the measured speed is averaged over.
	framePacingSampleWindow = time.Second
)

// FrameQuantumBackend reports how much guest time one RunFrame advances.
//
// The core's virtual clock never reads host wall time, so the guest only runs
// at handset speed if the shell issues quanta at this rate. The rate is not
// the same for every runtime, which is why it cannot be assumed to match the
// display refresh rate.
type FrameQuantumBackend interface {
	FrameQuantum() time.Duration
}

func (s *Shell) frameQuantum() time.Duration {
	backend, ok := s.backend.(FrameQuantumBackend)
	if !ok {
		return framePacingFallbackQuantum
	}
	if quantum := backend.FrameQuantum(); quantum > 0 {
		return quantum
	}
	return framePacingFallbackQuantum
}

// framePacingSpeed is the requested ratio of guest time to real time.
func (s *Shell) framePacingSpeed() float64 {
	if s.settings.Speed > 0 {
		return s.settings.Speed
	}
	return 1
}

// now reads the shell's clock. Tests replace it to drive pacing exactly.
func (s *Shell) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// resetFramePacing drops banked time. It runs whenever the guest is not
// executing so a pause does not become a burst of catch-up on resume.
//
// The last measured speed is deliberately kept. This runs on every idle tick,
// including while the settings panel is open - which is exactly where the
// achieved-speed readout is shown. Zeroing it here blanked the value the
// moment the user opened settings to read it, so a title running below handset
// speed looked identical to one running at full speed.
func (s *Shell) resetFramePacing() {
	s.frameAccumulator = 0
	s.lastFramePacingAt = time.Time{}
	s.pacingQuantaIssued = 0
	s.pacingSampleStartedAt = time.Time{}
}

// clearMeasuredSpeed forgets the achieved-speed readout. It runs when the
// sample no longer applies - a different title is loaded or the machine is
// fully unloaded - rather than on every transient pause.
func (s *Shell) clearMeasuredSpeed() {
	s.measuredSpeed = 0
}

// accumulateFramePacing folds elapsed real time into the guest's time budget.
//
// It runs on every tick, including ticks that cannot start work because a
// batch is still in flight. That is the whole point: a frame the host was too
// busy to run stays owed instead of being dropped, which is what used to make
// a title fall permanently behind after a single slow moment.
func (s *Shell) accumulateFramePacing(now time.Time, quantum time.Duration) {
	if quantum <= 0 {
		return
	}
	if s.lastFramePacingAt.IsZero() {
		// Nothing has elapsed yet, but a freshly started or resumed machine
		// should draw immediately rather than after a tick of dead air.
		s.lastFramePacingAt = now
		s.frameAccumulator = quantum
		return
	}
	delta := now.Sub(s.lastFramePacingAt)
	s.lastFramePacingAt = now
	if delta <= 0 {
		return
	}
	if delta > framePacingCatchUpLimit {
		delta = framePacingCatchUpLimit
	}
	s.frameAccumulator += time.Duration(float64(delta) * s.framePacingSpeed())
	if ceiling := framePacingQuantaPerTick * quantum; s.frameAccumulator > ceiling {
		s.frameAccumulator = ceiling
	}
}

// takeFrameQuanta consumes whole quanta from the budget. The remainder stays
// banked: a quantum rarely divides the tick interval evenly, so discarding it
// would drift the guest clock away from real time.
func (s *Shell) takeFrameQuanta(quantum time.Duration) int {
	if quantum <= 0 || s.frameAccumulator < quantum {
		return 0
	}
	owed := int(s.frameAccumulator / quantum)
	if owed > framePacingQuantaPerTick {
		owed = framePacingQuantaPerTick
	}
	s.frameAccumulator -= time.Duration(owed) * quantum
	return owed
}

// recordPacingSample tracks how much guest time was actually issued against
// real time, so the achieved speed can be shown and asserted rather than
// assumed.
func (s *Shell) recordPacingSample(
	now time.Time,
	quanta int,
	quantum time.Duration,
) {
	if quanta <= 0 {
		return
	}
	if s.pacingSampleStartedAt.IsZero() {
		s.pacingSampleStartedAt = now
		s.pacingQuantaIssued = 0
	}
	s.pacingQuantaIssued += quanta
	elapsed := now.Sub(s.pacingSampleStartedAt)
	if elapsed < framePacingSampleWindow {
		return
	}
	advanced := time.Duration(s.pacingQuantaIssued) * quantum
	s.measuredSpeed = float64(advanced) / float64(elapsed)
	s.pacingSampleStartedAt = now
	s.pacingQuantaIssued = 0
}

// MeasuredSpeed reports the ratio of guest time to real time observed over the
// last completed sample window, or zero before one has completed. At the
// default setting a healthy machine reports approximately 1.
func (s *Shell) MeasuredSpeed() float64 {
	return s.measuredSpeed
}

func (s *Shell) scheduleRunningFrame() {
	if !s.hostActive ||
		s.hostPaused ||
		s.loading ||
		s.dialogOpen ||
		s.problem != nil ||
		s.input == nil ||
		len(s.busyCommands) != 0 ||
		s.backend.State() != StateRunning {
		s.resetFramePacing()
		return
	}
	backend, ok := s.backend.(FrameBackend)
	if !ok {
		s.resetFramePacing()
		return
	}
	quantum := s.frameQuantum()
	now := s.now()
	s.accumulateFramePacing(now, quantum)
	if s.frameRunPending {
		return
	}
	owed := s.takeFrameQuanta(quantum)
	if owed == 0 {
		return
	}
	s.recordPacingSample(now, owed, quantum)
	generation := s.frameGeneration
	s.frameRunPending = true
	go func() {
		var err error
		for range owed {
			if err = backend.RunFrame(context.Background()); err != nil {
				break
			}
		}
		s.frameRunResults <- frameRunResult{
			generation: generation,
			err:        err,
		}
	}()
}

// speedSettingValue labels the speed control with the achieved ratio beside the
// requested one, so a machine that cannot keep up is visible rather than being
// mistaken for the title running slowly by design.
func (s *Shell) speedSettingValue() string {
	requested := fmt.Sprintf("%gx", s.framePacingSpeed())
	measured := s.MeasuredSpeed()
	if measured <= 0 {
		return requested
	}
	return fmt.Sprintf("%s (%.0f%%)", requested, measured/s.framePacingSpeed()*100)
}
