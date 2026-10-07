package frontend

import (
	"sync"
	"testing"
	"time"
)

type blockedLifecycleBackend struct {
	NullBackend
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockedLifecycleBackend) State() BackendState {
	b.once.Do(func() { close(b.entered); <-b.release })
	return StateRunning
}

func TestNativeAudioGateLossDuringLifecycleSync(t *testing.T) {
	for _, gate := range []string{"focus", "foreground"} {
		t.Run(gate, func(t *testing.T) {
			backend := &blockedLifecycleBackend{entered: make(chan struct{}), release: make(chan struct{})}
			shell := &Shell{backend: backend, busyCommands: make(map[BackendCommand]bool)}
			shell.hostActiveRequest.Store(true)
			shell.audioFocusRequest.Store(true)
			done := make(chan struct{})
			go func() { shell.syncHostLifecycle(); close(done) }()
			select {
			case <-backend.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("lifecycle sync did not reach state read")
			}
			if gate == "focus" {
				shell.SetAudioFocus(false)
			} else {
				shell.SetHostActive(false)
			}
			close(backend.release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("lifecycle sync did not finish")
			}
			if !shell.audioSuspended || shell.hostActive {
				t.Fatal("stale lifecycle sync reopened a native audio gate")
			}
		})
	}
}
