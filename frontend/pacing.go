package frontend

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"
)

// debugPacing logs, on change, why the guest frame pump does or does not run.
// It is a temporary diagnostic for headless/handheld bring-up.
var debugPacing = os.Getenv("ARAM_DEBUG_PACING") != ""
var lastPacingReason string
var pendingStuck, owedZero int

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
	s.pacingGuestAdvanced = 0
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

// recordPacingSample tracks completed guest time against wall time. Recording
// issued work made a slow backend look healthy while its batch was still
// running; completion timestamps expose the actual achieved ratio.
func (s *Shell) recordPacingSample(
	startedAt time.Time,
	completedAt time.Time,
	guestAdvanced time.Duration,
) {
	if guestAdvanced <= 0 || completedAt.Before(startedAt) {
		return
	}
	if s.pacingSampleStartedAt.IsZero() {
		s.pacingSampleStartedAt = startedAt
		s.pacingGuestAdvanced = 0
	}
	s.pacingGuestAdvanced += guestAdvanced
	elapsed := completedAt.Sub(s.pacingSampleStartedAt)
	if elapsed < framePacingSampleWindow {
		return
	}
	s.measuredSpeed = float64(s.pacingGuestAdvanced) / float64(elapsed)
	s.pacingSampleStartedAt = completedAt
	s.pacingGuestAdvanced = 0
}

// MeasuredSpeed reports the ratio of guest time to real time observed over the
// last completed sample window, or zero before one has completed. At the
// default setting a healthy machine reports approximately 1.
func (s *Shell) MeasuredSpeed() float64 {
	return s.measuredSpeed
}

// scheduleRunningFrame hands the backend the guest time real time has earned.
// A panel that captures host input also stops the guest: settings, the issue
// report form and every other modal panel used to leave the title running
// behind them, so a machine kept playing - and kept taking damage - while its
// controls went to the panel. Panels that opted into guest input (a cheat
// toggled mid-fight) keep the title running exactly as before.
func (s *Shell) scheduleRunningFrame() {
	if debugPacing {
		reason := "OK"
		switch {
		case !s.hostActive:
			reason = "hostInactive"
		case s.hostPaused:
			reason = "hostPaused"
		case s.loading:
			reason = "loading"
		case s.dialogOpen:
			reason = "dialogOpen"
		case !s.guestInputAllowed():
			reason = "panelOpen"
		case s.problem != nil:
			reason = "problem"
		case s.input == nil:
			reason = "nilInput"
		case len(s.busyCommands) != 0:
			reason = "busyCommands"
		case s.backend.State() != StateRunning:
			reason = fmt.Sprintf("state=%v", s.backend.State())
		}
		if reason != lastPacingReason {
			fmt.Fprintf(os.Stderr, "pacing: %s\n", reason)
			lastPacingReason = reason
		}
	}
	if !s.hostActive ||
		s.hostPaused ||
		s.loading ||
		s.dialogOpen ||
		!s.guestInputAllowed() ||
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
		if debugPacing {
			pendingStuck++
			if pendingStuck%120 == 1 {
				fmt.Fprintf(os.Stderr, "sched: frameRunPending stuck (n=%d)\n", pendingStuck)
			}
		}
		return
	}
	owed := s.takeFrameQuanta(quantum)
	if owed == 0 {
		if debugPacing {
			owedZero++
			if owedZero%240 == 1 {
				fmt.Fprintf(os.Stderr, "sched: owed=0 (n=%d) quantum=%v\n", owedZero, quantum)
			}
		}
		return
	}
	s.startFrameWorker()
	s.frameRunPending = true
	s.frameRunRequests <- frameRunRequest{
		backend:    backend,
		owed:       owed,
		quantum:    quantum,
		generation: s.frameGeneration,
		startedAt:  now,
		uiPriority: s.settings.UIPriority,
	}
}

// uiPriorityFrameRest is the pause the worker takes after each guest frame when
// UI priority is on. Lowering the worker thread's priority already lets the
// interface preempt the guest; this hands back a slice of the physical core as
// well, so even a single- or dual-core host stays responsive while a heavy
// title runs a little slower.
const uiPriorityFrameRest = 3 * time.Millisecond

// startFrameWorker launches the guest frame worker once, on first use. The
// worker owns a single de-prioritised OS thread for the shell's lifetime, so a
// heavy title yields CPU to the interface instead of stalling it.
func (s *Shell) startFrameWorker() {
	s.frameWorkerOnce.Do(func() {
		go s.runFrameWorker()
	})
}

// runFrameWorker executes batched guest quanta off the interface goroutine. It
// pins itself to one OS thread and lowers that thread's priority so the guest
// never starves the ebiten update/draw thread of CPU. It processes one batch
// at a time; scheduleRunningFrame only enqueues while no batch is in flight.
func (s *Shell) runFrameWorker() {
	runtime.LockOSThread()
	lowerCurrentThreadPriority()
	for request := range s.frameRunRequests {
		var (
			completed int
			err       error
		)
		if debugPacing {
			fmt.Fprintf(os.Stderr, "worker: batch owed=%d\n", request.owed)
		}
		for range request.owed {
			if err = request.backend.RunFrame(context.Background()); err != nil {
				break
			}
			completed++
			if request.uiPriority {
				time.Sleep(uiPriorityFrameRest)
			}
		}
		if debugPacing {
			fmt.Fprintf(os.Stderr, "worker: done completed=%d err=%v\n", completed, err)
		}
		s.frameRunResults <- frameRunResult{
			generation:      request.generation,
			completedQuanta: completed,
			guestAdvanced:   time.Duration(completed) * request.quantum,
			startedAt:       request.startedAt,
			completedAt:     s.now(),
			err:             err,
		}
	}
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

// debugPacingReport captures what the frame pacer achieved, for the debug
// bundle. Reading it costs nothing and it is the number that separates "the
// host cannot keep up" from "the title runs at this speed on a handset too".
func (s *Shell) debugPacingReport() debugPacingReport {
	requested := s.framePacingSpeed()
	measured := s.MeasuredSpeed()
	achieved := float64(0)
	if requested > 0 && measured > 0 {
		achieved = measured / requested * 100
	}
	return debugPacingReport{
		RequestedSpeed:  requested,
		MeasuredSpeed:   measured,
		AchievedPercent: achieved,
		UIPriority:      s.settings.UIPriority,
	}
}
