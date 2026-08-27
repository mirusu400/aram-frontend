package frontend

import (
	"testing"
	"time"
)

// ktfQuantum is the awkward one: it does not divide the sixty-hertz tick
// interval evenly, which is exactly the case a fixed calls-per-tick driver
// gets wrong.
const ktfQuantum = (time.Second + 30) / 60

func newPacingShell(quantum time.Duration) (*Shell, func(time.Duration)) {
	shell := &Shell{}
	shell.settings.Speed = 1
	clock := time.Unix(0, 0)
	shell.nowFunc = func() time.Time { return clock }
	return shell, func(step time.Duration) { clock = clock.Add(step) }
}

// runPacing drives ticks of the given interval and reports how much guest time
// the shell handed out, mimicking a display loop that always keeps up.
func runPacing(
	shell *Shell,
	advance func(time.Duration),
	quantum, tick, total time.Duration,
) time.Duration {
	var issued int
	for elapsed := time.Duration(0); elapsed < total; elapsed += tick {
		advance(tick)
		shell.accumulateFramePacing(shell.now(), quantum)
		issued += shell.takeFrameQuanta(quantum)
	}
	return time.Duration(issued) * quantum
}

// TestPacingTracksRealTimeAcrossQuantumAndTickMismatch is the property the
// whole mechanism exists for: whatever the quantum, one minute of real time
// hands the guest one minute of guest time.
func TestPacingTracksRealTimeAcrossQuantumAndTickMismatch(t *testing.T) {
	tests := []struct {
		name    string
		quantum time.Duration
		tick    time.Duration
	}{
		{name: "KTF quantum on a 60Hz tick", quantum: ktfQuantum, tick: time.Second / 60},
		{name: "16ms quantum on a 60Hz tick", quantum: 16 * time.Millisecond, tick: time.Second / 60},
		{name: "KTF quantum on a 144Hz tick", quantum: ktfQuantum, tick: time.Second / 144},
		{name: "16ms quantum on a 30Hz tick", quantum: 16 * time.Millisecond, tick: time.Second / 30},
	}
	const total = time.Minute
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shell, advance := newPacingShell(test.quantum)
			advanced := runPacing(shell, advance, test.quantum, test.tick, total)
			drift := advanced - total
			if drift < 0 {
				drift = -drift
			}
			// One quantum of slack covers the partial quantum still banked.
			if drift > test.quantum+test.tick {
				t.Fatalf(
					"advanced %s over %s of real time, drifting %s",
					advanced,
					total,
					advanced-total,
				)
			}
		})
	}
}

// TestPacingRecoversFromAStall pins the regression that made titles run slow:
// a tick the host could not service must stay owed, not be dropped.
func TestPacingRecoversFromAStall(t *testing.T) {
	const quantum = ktfQuantum
	shell, advance := newPacingShell(quantum)

	// Seed, then consume the immediate first quantum.
	shell.accumulateFramePacing(shell.now(), quantum)
	if got := shell.takeFrameQuanta(quantum); got != 1 {
		t.Fatalf("first tick issued %d quanta, want 1", got)
	}

	// Five ticks pass while a batch is in flight, so nothing is consumed.
	for range 5 {
		advance(quantum)
		shell.accumulateFramePacing(shell.now(), quantum)
	}
	if got := shell.takeFrameQuanta(quantum); got != 5 {
		t.Fatalf("after a five tick stall the shell issued %d quanta, want 5", got)
	}
	if shell.frameAccumulator >= quantum {
		t.Fatalf("backlog %s remained after catching up", shell.frameAccumulator)
	}
}

// TestPacingDropsRatherThanBanksALongGap keeps a suspended window, a
// breakpoint, or a closed lid from fast-forwarding the title on resume.
func TestPacingDropsRatherThanBanksALongGap(t *testing.T) {
	const quantum = ktfQuantum
	shell, advance := newPacingShell(quantum)
	shell.accumulateFramePacing(shell.now(), quantum)
	shell.takeFrameQuanta(quantum)

	advance(10 * time.Minute)
	shell.accumulateFramePacing(shell.now(), quantum)
	if got := shell.takeFrameQuanta(quantum); got > framePacingQuantaPerTick {
		t.Fatalf("a ten minute gap issued %d quanta at once", got)
	}
	if shell.frameAccumulator >= quantum {
		t.Fatalf("a ten minute gap banked %s of guest time", shell.frameAccumulator)
	}
}

// TestPacingHonoursTheSpeedSetting checks the speed control actually scales
// guest time; it was previously shown in the interface but wired to nothing.
func TestPacingHonoursTheSpeedSetting(t *testing.T) {
	const quantum = ktfQuantum
	for _, speed := range []float64{0.5, 1, 2} {
		shell, advance := newPacingShell(quantum)
		shell.settings.Speed = speed
		// Discard the immediate startup quantum so only paced time is measured.
		shell.accumulateFramePacing(shell.now(), quantum)
		shell.takeFrameQuanta(quantum)

		const total = 4 * time.Second
		var issued int
		for elapsed := time.Duration(0); elapsed < total; elapsed += quantum {
			advance(quantum)
			shell.accumulateFramePacing(shell.now(), quantum)
			issued += shell.takeFrameQuanta(quantum)
		}
		advanced := time.Duration(issued) * quantum
		want := time.Duration(float64(total) * speed)
		drift := advanced - want
		if drift < 0 {
			drift = -drift
		}
		if drift > 2*quantum {
			t.Fatalf(
				"at %gx speed %s of real time advanced %s, want about %s",
				speed,
				total,
				advanced,
				want,
			)
		}
	}
}

// TestPacingResetClearsBankedTime keeps a pause from becoming a burst.
func TestPacingResetClearsBankedTime(t *testing.T) {
	const quantum = ktfQuantum
	shell, advance := newPacingShell(quantum)
	shell.accumulateFramePacing(shell.now(), quantum)
	advance(5 * quantum)
	shell.accumulateFramePacing(shell.now(), quantum)

	shell.resetFramePacing()
	if shell.frameAccumulator != 0 {
		t.Fatalf("reset left %s banked", shell.frameAccumulator)
	}
	// The next tick after a reset starts immediately rather than idling.
	shell.accumulateFramePacing(shell.now(), quantum)
	if got := shell.takeFrameQuanta(quantum); got != 1 {
		t.Fatalf("first tick after a reset issued %d quanta, want 1", got)
	}
}

// TestMeasuredSpeedReportsTheAchievedRatio covers the readout used to confirm
// a title is running at handset speed.
func TestMeasuredSpeedReportsTheAchievedRatio(t *testing.T) {
	const quantum = ktfQuantum
	shell, advance := newPacingShell(quantum)
	if shell.MeasuredSpeed() != 0 {
		t.Fatal("speed was reported before a sample window completed")
	}
	for elapsed := time.Duration(0); elapsed < 2*framePacingSampleWindow; elapsed += quantum {
		startedAt := shell.now()
		advance(quantum)
		shell.accumulateFramePacing(shell.now(), quantum)
		if owed := shell.takeFrameQuanta(quantum); owed > 0 {
			shell.recordPacingSample(
				startedAt,
				shell.now(),
				time.Duration(owed)*quantum,
			)
		}
	}
	if speed := shell.MeasuredSpeed(); speed < 0.95 || speed > 1.05 {
		t.Fatalf("measured speed = %g, want about 1", speed)
	}
}

// TestMeasuredSpeedSurvivesPacingReset keeps the achieved-speed readout legible
// while the guest is paused. The settings panel - the only place the readout is
// shown - pauses the guest, so resetFramePacing must not blank the last sample.
func TestMeasuredSpeedSurvivesPacingReset(t *testing.T) {
	const quantum = ktfQuantum
	shell, advance := newPacingShell(quantum)
	for elapsed := time.Duration(0); elapsed < 2*framePacingSampleWindow; elapsed += quantum {
		startedAt := shell.now()
		advance(quantum)
		shell.accumulateFramePacing(shell.now(), quantum)
		if owed := shell.takeFrameQuanta(quantum); owed > 0 {
			shell.recordPacingSample(
				startedAt,
				shell.now(),
				time.Duration(owed)*quantum,
			)
		}
	}
	measured := shell.MeasuredSpeed()
	if measured <= 0 {
		t.Fatalf("no speed sampled before reset: %g", measured)
	}
	shell.resetFramePacing()
	if got := shell.MeasuredSpeed(); got != measured {
		t.Fatalf("reset changed the readout to %g, want %g preserved", got, measured)
	}
	shell.clearMeasuredSpeed()
	if got := shell.MeasuredSpeed(); got != 0 {
		t.Fatalf("clear left readout at %g, want 0", got)
	}
}

func TestMeasuredSpeedUsesCompletedGuestTime(t *testing.T) {
	shell := &Shell{}
	startedAt := time.Unix(10, 0)
	shell.recordPacingSample(
		startedAt,
		startedAt.Add(2*time.Second),
		time.Second,
	)
	if got := shell.MeasuredSpeed(); got != 0.5 {
		t.Fatalf("measured speed = %g, want 0.5", got)
	}
}

// A batch that was merely issued has advanced no guest time yet. A backend
// taking two seconds to finish one quantum must report its completed ratio,
// not the requested 1x rate recorded before RunFrame returned.
func TestScheduledFrameSpeedIsRecordedAfterCompletion(t *testing.T) {
	backend := &schedulingBackend{
		started: make(chan struct{}, 1),
		release: make(chan struct{}, 1),
		state:   StateRunning,
	}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	shell.panel = nil
	shell.settings.Speed = 1
	clock := time.Unix(20, 0)
	shell.nowFunc = func() time.Time { return clock }

	shell.scheduleRunningFrame()
	waitSignal(t, backend.started, "slow frame")
	if got := shell.MeasuredSpeed(); got != 0 {
		t.Fatalf("issued frame reported speed %g before completion", got)
	}
	clock = clock.Add(2 * time.Second)
	backend.release <- struct{}{}
	waitFrameCompletion(t, shell)

	want := float64(shell.frameQuantum()) / float64(2*time.Second)
	if got := shell.MeasuredSpeed(); got != want {
		t.Fatalf("completed-frame speed = %g, want %g", got, want)
	}
}

// TestAnOpenPanelPausesTheGuest covers the settings and issue-report panels:
// their host input goes to the panel, so the title behind them must not keep
// running and taking hits while the user reads or types. A panel that opted
// into guest input keeps the machine running.
func TestAnOpenPanelPausesTheGuest(t *testing.T) {
	backend := &schedulingBackend{
		started: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
		state:   StateRunning,
	}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	shell.settings.Speed = 1
	clock := time.Now()
	shell.nowFunc = func() time.Time { return clock }
	advance := func() { clock = clock.Add(shell.frameQuantum()) }

	frameCalls := func() int {
		backend.mu.Lock()
		defer backend.mu.Unlock()
		return backend.calls
	}

	for _, panel := range []*Panel{
		{Kind: "settings", Title: "Configure ARAM"},
		{Kind: "issue-report", Title: "Report an issue"},
	} {
		shell.panel = panel
		advance()
		shell.scheduleRunningFrame()
		if calls := frameCalls(); calls != 0 {
			t.Fatalf("%s panel advanced the guest; calls = %d", panel.Kind, calls)
		}
	}

	// A cheat panel that keeps host input on the guest keeps it running.
	shell.panel.AllowGuestInput = true
	advance()
	shell.scheduleRunningFrame()
	waitSignal(t, backend.started, "frame behind a guest-input panel")
	backend.release <- struct{}{}
	waitFrameCompletion(t, shell)

	shell.panel = nil
	advance()
	shell.scheduleRunningFrame()
	waitSignal(t, backend.started, "frame after the panel closed")
	backend.release <- struct{}{}
	waitFrameCompletion(t, shell)
}
