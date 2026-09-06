package frontend

import (
	"math"
	"time"
)

const (
	// displaySyncTolerance is how far the synchronized guest rate may sit
	// from the requested speed before the plan is abandoned for wall-clock
	// pacing. A 62.5 Hz title on a 60 Hz display runs 4% slow under sync,
	// which is the case this exists for; a 144 Hz display would need 15% and
	// is left to real-time pacing instead.
	displaySyncTolerance = 0.05
	// displaySyncWindow is how long host ticks are counted before the tick
	// rate is (re)estimated.
	displaySyncWindow = time.Second
	// displaySyncStallLimit is the tick gap beyond which the gap is a stall
	// (a dragged window, a breakpoint, a lid) rather than the display's
	// rhythm. Counting one 200 ms hitch would read as a 20% slower display,
	// abandon the plan for the next second, and turn the hitch into the very
	// burst sync exists to avoid.
	displaySyncStallLimit = 100 * time.Millisecond
)

// displaySyncState tracks the host tick rate and the current sync plan.
//
// Wall-clock pacing hands the guest exactly as much time as passed, which is
// right on average but, when the guest frame rate and the display refresh do
// not divide, produces a periodic tick that issues two quanta - a visible
// stutter a few times a second. Display sync instead issues a fixed budget per
// host tick so that guest frames land on display frames, accepting a small,
// constant speed error the way a console emulator does.
type displaySyncState struct {
	tickCount   int
	windowStart time.Time
	lastTick    time.Time
	// tickRate is the measured host ticks per second, zero until the first
	// window has closed.
	tickRate float64
	// active reports whether the last accumulate used the sync plan.
	active bool
	// budget is the guest time issued per host tick while active.
	budget time.Duration
	// speed is the guest-to-real time ratio the plan achieves while active.
	speed float64
}

// recordHostTick counts one Update call. It runs on every tick, guest or no
// guest, so the rate is already known when a title starts.
func (s *Shell) recordHostTick(now time.Time) {
	d := &s.displaySync
	if d.windowStart.IsZero() {
		d.windowStart = now
		d.lastTick = now
		d.tickCount = 0
		return
	}
	gap := now.Sub(d.lastTick)
	d.lastTick = now
	if gap > displaySyncStallLimit {
		// Excise the stall from the window instead of letting it dilute the
		// rate: the ticks either side of it were still one per refresh.
		d.windowStart = d.windowStart.Add(gap)
		return
	}
	d.tickCount++
	elapsed := now.Sub(d.windowStart)
	if elapsed < displaySyncWindow {
		return
	}
	d.tickRate = float64(d.tickCount) / elapsed.Seconds()
	d.windowStart = now
	d.tickCount = 0
}

// displaySyncEnabled reports whether the user asked for sync and the host
// loop can honour it. Without vsync the tick rate has nothing to do with the
// display, so the plan would only add a random speed error.
func (s *Shell) displaySyncEnabled() bool {
	return s.settings.DisplaySync && !s.settings.VsyncDisabled
}

// displaySyncBudget returns the guest time to issue this tick under the sync
// plan, or false when wall-clock pacing must be used instead. It also records
// the plan so the readout and the audio path can follow it.
func (s *Shell) displaySyncBudget(quantum time.Duration) (time.Duration, bool) {
	d := &s.displaySync
	budget, speed, ok := displaySyncPlan(
		d.tickRate, quantum, s.framePacingSpeed(), s.displaySyncEnabled(),
	)
	d.active, d.budget, d.speed = ok, budget, speed
	return budget, ok
}

// displaySyncPlan works out how much guest time each host tick should carry
// so that guest frames fall on a whole number of ticks (or ticks on a whole
// number of guest frames), and what guest-to-real speed that produces.
//
// With a guest frame every 16 ms and 60 ticks per second the plan is one
// quantum per tick at 0.96x; at 120 ticks it is half a quantum per tick,
// again 0.96x; at 240 it is a quarter. Displays whose rate lands too far from
// a whole ratio (144 Hz, 50 Hz, 75 Hz) are refused so the title keeps real
// time there.
func displaySyncPlan(
	tickRate float64,
	quantum time.Duration,
	speed float64,
	enabled bool,
) (budget time.Duration, achieved float64, ok bool) {
	if !enabled || tickRate <= 0 || quantum <= 0 || speed <= 0 {
		return 0, 0, false
	}
	// target is the guest frame rate the speed setting asks for.
	target := speed / quantum.Seconds()
	if target >= tickRate {
		perTick := math.Round(target / tickRate)
		budget = time.Duration(perTick) * quantum
		achieved = perTick * tickRate * quantum.Seconds()
	} else {
		ticksPerFrame := math.Max(1, math.Round(tickRate/target))
		// Round up so an odd quantum still completes on its scheduled tick
		// rather than one nanosecond short of it.
		budget = time.Duration(math.Ceil(float64(quantum) / ticksPerFrame))
		achieved = tickRate * quantum.Seconds() / ticksPerFrame
	}
	if math.Abs(achieved/speed-1) > displaySyncTolerance {
		return 0, 0, false
	}
	return budget, achieved, true
}

// effectivePacingSpeed is the guest-to-real ratio the pacer is actually
// aiming for: the sync plan's rate while it is engaged, otherwise the speed
// setting. The pacer copies it into Shell.audioSpeed on every tick, and the
// main-thread audio drain pushes that to the output, which stretches guest
// PCM by the ratio so a synchronized title does not underrun its host queue
// by the same few percent every second.
func (s *Shell) effectivePacingSpeed() float64 {
	if s.displaySync.active && s.displaySync.speed > 0 {
		return s.displaySync.speed
	}
	return s.framePacingSpeed()
}

// applyHostLoopSettings pushes the vsync preference to ebiten. It runs once
// at construction and again whenever the preference changes.
func (s *Shell) applyHostLoopSettings() {
	applyHostLoop(s.settings.VsyncDisabled)
}

// toggleDisplaySync flips the persisted display-sync preference. The pacer
// re-plans on its next tick, so no reset is needed.
func (s *Shell) toggleDisplaySync() {
	s.settings.DisplaySync = !s.settings.DisplaySync
	_ = s.settings.save()
	s.setStatus(s.trf("Display sync: %s", s.tr(onOff(s.settings.DisplaySync))))
}

// toggleVsync flips the persisted vsync preference and applies it at once.
func (s *Shell) toggleVsync() {
	s.settings.VsyncDisabled = !s.settings.VsyncDisabled
	_ = s.settings.save()
	s.applyHostLoopSettings()
	s.setStatus(s.trf("Vsync: %s", s.tr(onOff(!s.settings.VsyncDisabled))))
}
