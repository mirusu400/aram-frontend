package frontend

import (
	"context"
	"errors"
	"testing"
	"time"
)

type batchContractBackend struct {
	calls  int
	failAt int
	err    error
}

func (b *batchContractBackend) RunFrame(context.Context) error {
	b.calls++
	if b.calls == b.failAt {
		return b.err
	}
	return nil
}

func TestFrameWorkerReportsOnlyCompletedGuestQuanta(t *testing.T) {
	for _, test := range []struct {
		name      string
		owed      int
		failAt    int
		completed int
		calls     int
	}{
		{"complete single", 1, 0, 1, 1},
		{"complete maximum batch", framePacingQuantaPerTick, 0, framePacingQuantaPerTick, framePacingQuantaPerTick},
		{"first frame fails", framePacingQuantaPerTick, 1, 0, 1},
		{"later frame fails", framePacingQuantaPerTick, 4, 3, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			const quantum = time.Second / 60
			const generation = 7
			start := time.Unix(100, 0)
			failure := errors.New("synthetic frame failure")
			backend := &batchContractBackend{failAt: test.failAt, err: failure}
			shell := &Shell{
				frameRunRequests: make(chan frameRunRequest, 1),
				frameRunResults:  make(chan frameRunResult, 1),
				nowFunc:          func() time.Time { return start.Add(time.Second) },
			}
			shell.frameRunRequests <- frameRunRequest{
				backend: backend, owed: test.owed, quantum: quantum,
				generation: generation, startedAt: start,
			}
			close(shell.frameRunRequests)
			done := make(chan struct{})
			go func() {
				shell.runFrameWorker()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("frame worker did not finish the bounded request")
			}
			result := <-shell.frameRunResults
			if backend.calls != test.calls || result.completedQuanta != test.completed || result.guestAdvanced != time.Duration(test.completed)*quantum {
				t.Fatalf("calls=%d, completed=%d, advanced=%s; want %d, %d, %s", backend.calls, result.completedQuanta, result.guestAdvanced, test.calls, test.completed, time.Duration(test.completed)*quantum)
			}
			if result.generation != generation || result.startedAt != start || result.completedAt != start.Add(time.Second) {
				t.Fatalf("frame result lost its timeline identity: %+v", result)
			}
			if test.failAt > 0 && !errors.Is(result.err, failure) || test.failAt == 0 && result.err != nil {
				t.Fatalf("frame failure was changed: %v", result.err)
			}
		})
	}
}
