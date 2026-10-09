package frontend

import (
	"testing"
	"time"
)

func TestInputFrameBorrowIsRepaidAndCannotRepeat(t *testing.T) {
	const quantum = 16 * time.Millisecond
	shell, advance := newPacingShell(quantum)
	shell.accumulateFramePacing(shell.now(), quantum)
	shell.takeScheduledFrameQuanta(quantum)
	advance(4 * time.Millisecond)
	shell.accumulateFramePacing(shell.now(), quantum)
	shell.requestInputFrame()
	if got := shell.takeScheduledFrameQuanta(quantum); got != 1 {
		t.Fatalf("accepted input waited for the next quantum: %d", got)
	}
	if shell.frameAccumulator != -12*time.Millisecond {
		t.Fatalf("borrowed quantum was not debited: %s", shell.frameAccumulator)
	}
	for range 50 {
		shell.requestInputFrame()
		if got := shell.takeScheduledFrameQuanta(quantum); got != 0 {
			t.Fatalf("repeated input accelerated guest by %d quanta", got)
		}
	}
	advance(12 * time.Millisecond)
	shell.accumulateFramePacing(shell.now(), quantum)
	if shell.frameAccumulator != 0 {
		t.Fatalf("borrowed time was not repaid: %s", shell.frameAccumulator)
	}
}

func TestRapidInputMaintainsRequestedGuestSpeed(t *testing.T) {
	const quantum = 16 * time.Millisecond
	shell, advance := newPacingShell(quantum)
	var issued int
	const total = 10 * time.Second
	for elapsed := time.Duration(0); elapsed < total; elapsed += time.Millisecond {
		advance(time.Millisecond)
		shell.accumulateFramePacing(shell.now(), quantum)
		shell.requestInputFrame()
		issued += shell.takeScheduledFrameQuanta(quantum)
	}
	guest := time.Duration(issued) * quantum
	if guest > total+2*quantum || guest < total-quantum {
		t.Fatalf("rapid input advanced %s in %s", guest, total)
	}
}

func TestInputWakeIsCoalescedAndPauseClearsRequest(t *testing.T) {
	shell := &Shell{frameWorkerWake: make(chan struct{}, 1)}
	for range 50 {
		shell.requestInputFrame()
	}
	if len(shell.frameWorkerWake) != 1 || !shell.inputFramePending {
		t.Fatalf("input wake was not coalesced")
	}
	shell.waitFrameRest()
	if len(shell.frameWorkerWake) != 0 {
		t.Fatalf("worker did not consume input wake")
	}
	shell.resetFramePacing()
	if shell.inputFramePending {
		t.Fatalf("paused input request leaked into resume")
	}
}

func TestFramePerformanceTargetFollowsSpeed(t *testing.T) {
	for _, test := range []struct {
		speed float64
		want  time.Duration
	}{{1, 16 * time.Millisecond}, {2, 8 * time.Millisecond}, {.5, 32 * time.Millisecond}, {100, time.Millisecond}, {.01, 250 * time.Millisecond}} {
		if got := hostFrameWorkTarget(16*time.Millisecond, test.speed); got != test.want {
			t.Fatalf("speed %g target %s, want %s", test.speed, got, test.want)
		}
	}
}
