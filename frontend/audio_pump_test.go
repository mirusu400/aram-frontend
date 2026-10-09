package frontend

import (
	"testing"
	"time"
)

type audioPumpBackend struct {
	videoBackend
	state BackendState
}

func (b *audioPumpBackend) State() BackendState { return b.state }

func newAudioPumpShell(t *testing.T) (*Shell, *audioPumpBackend) {
	t.Helper()
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	previous := currentHostAudioProperties()
	SetHostAudioProperties(48_000, 192)
	t.Cleanup(func() { SetHostAudioProperties(previous.SampleRate, previous.FramesPerBuffer) })
	backend := &audioPumpBackend{state: StateRunning}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic"}
	return shell, backend
}

func TestAudioPumpWaitsWhileOutputIsIdle(t *testing.T) {
	shell, _ := newAudioPumpShell(t)
	if got := shell.audioPumpInterval(); got != 0 {
		t.Fatalf("pump without an output wakes every %s", got)
	}
	shell.audioOutput = newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	if got := shell.audioPumpInterval(); got != 2*time.Millisecond {
		t.Fatalf("active pump interval %s, want 2ms", got)
	}
	for _, state := range []BackendState{StatePaused, StateStopped, StateReady, StateFaulted} {
		shell.finishAudioDiscontinuity(state)
		if got := shell.audioPumpInterval(); got != 0 {
			t.Fatalf("pump in %s wakes every %s", state, got)
		}
	}
	shell.finishAudioDiscontinuity(StateRunning)
	if got := shell.audioPumpInterval(); got != 2*time.Millisecond {
		t.Fatalf("resumed pump interval %s, want 2ms", got)
	}
	if err := shell.releaseCurrentInput(false); err != nil {
		t.Fatal(err)
	}
	if got := shell.audioPumpInterval(); got != 0 {
		t.Fatalf("pump after closing a title wakes every %s", got)
	}
}

func TestAudioPumpResumeWakesWithoutLifecycleChange(t *testing.T) {
	shell, _ := newAudioPumpShell(t)
	shell.audioOutput = newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	shell.beginAudioDiscontinuity()
	select {
	case <-shell.audioPumpWake:
	default:
		t.Fatal("suspending playback did not wake the pump to stop its timer")
	}
	shell.finishAudioDiscontinuity(StateRunning)
	if len(shell.audioPumpWake) != 1 {
		t.Fatal("resuming playback did not wake the idle pump")
	}
	for range 50 {
		shell.wakeAudioPump()
	}
	if len(shell.audioPumpWake) != 1 {
		t.Fatal("pump wake requests were not coalesced")
	}
	if got := shell.audioPumpInterval(); got != 2*time.Millisecond {
		t.Fatalf("resumed pump interval %s, want 2ms", got)
	}
}

func TestAudioPumpWaitsBehindNativeGates(t *testing.T) {
	for _, gate := range []string{"focus", "foreground"} {
		t.Run(gate, func(t *testing.T) {
			shell, _ := newAudioPumpShell(t)
			shell.audioOutput = newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
			if gate == "focus" {
				shell.SetAudioFocus(false)
			} else {
				shell.SetHostActive(false)
			}
			if got := shell.audioPumpInterval(); got != 0 {
				t.Fatalf("pump with lost %s wakes every %s", gate, got)
			}
			if gate == "focus" {
				shell.SetAudioFocus(true)
			} else {
				shell.SetHostActive(true)
			}
			shell.syncHostLifecycle()
			if got := shell.audioPumpInterval(); got != 2*time.Millisecond {
				t.Fatalf("pump after %s resume interval %s", gate, got)
			}
			if len(shell.audioPumpWake) != 1 {
				t.Fatal("native resume did not wake the idle pump")
			}
		})
	}
}

func TestAudioPumpRetainsFinalPCMAfterGuestStops(t *testing.T) {
	shell, backend := newAudioPumpShell(t)
	shell.audioOutput = newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	backend.state = StateStopped
	backend.chunks = []AudioChunk{{SampleRate: hostAudioSampleRate, Channels: 1, PCM16: make([]int16, 441)}}
	shell.updateAudio()
	if got := shell.audioOutput.queue.availableBytes(); got == 0 {
		t.Fatal("the guest's final PCM was not drained before idling")
	}
	if got := shell.audioPumpInterval(); got != 0 {
		t.Fatalf("pump after guest exit wakes every %s", got)
	}
}
