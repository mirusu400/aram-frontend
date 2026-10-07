package frontend

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

type audioControlBackend struct {
	NullBackend
	chunks []AudioChunk
}

func (b *audioControlBackend) DrainAudio() AudioChunk {
	if len(b.chunks) == 0 {
		return AudioChunk{}
	}
	chunk := b.chunks[0]
	b.chunks = b.chunks[1:]
	return chunk
}

func activeAudioShell(backend *audioControlBackend, output *audioOutput) *Shell {
	shell := &Shell{backend: backend, audioOutput: output, audioSpeed: 1}
	shell.hostActiveRequest.Store(true)
	shell.audioFocusRequest.Store(true)
	return shell
}

func TestPlayerRetirementErrorRemainsInTraceAfterRecreation(t *testing.T) {
	failure := errors.New("synthetic retirement failure")
	output := newAudioOutputWithFactory(AudioSettings{Latency: 20 * time.Millisecond}, func(source io.Reader) (hostAudioPlayer, error) {
		return &bufferedTestPlayer{source: source, closeErr: failure}, nil
	})
	chunk := AudioChunk{SampleRate: 44100, Channels: 1, PCM16: make([]int16, 882)}
	if err := output.enqueue(chunk, 0, 0); err != nil {
		t.Fatal(err)
	}
	output.flush()
	if err := output.enqueue(chunk, 0, 0); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.trace.render(), []byte(failure.Error())) {
		t.Fatal("recreation erased the player retirement failure")
	}
	_ = output.close()
}

func TestMainAudioDrainerAppliesCurrentDisplaySpeedBeforeEncoding(t *testing.T) {
	output := newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	backend := &audioControlBackend{chunks: []AudioChunk{{SampleRate: 44100, Channels: 1, PCM16: make([]int16, 705)}}}
	shell := activeAudioShell(backend, output)
	shell.audioSpeed = 0.96
	shell.drainAudioOnce(true)
	if got := output.telemetry().FillFrames; got != 735 {
		t.Fatalf("first display-synchronized drain = %d frames, want 735", got)
	}
}

func TestEmptyGenerationMarkerRetiresTransferredPlayerPCM(t *testing.T) {
	var player *bufferedTestPlayer
	output := newAudioOutputWithFactory(AudioSettings{Latency: 20 * time.Millisecond}, func(source io.Reader) (hostAudioPlayer, error) {
		player = &bufferedTestPlayer{source: source}
		return player, nil
	})
	backend := &audioControlBackend{chunks: []AudioChunk{{SampleRate: 44100, Channels: 1, PCM16: make([]int16, 882), Generation: 1}}}
	shell := activeAudioShell(backend, output)
	shell.drainAudioOnce(true)
	player.readAhead(441)
	backend.chunks = []AudioChunk{{SampleRate: 44100, Channels: 1, Generation: 2, StartGuestNS: 20_000_000}}
	shell.drainAudioOnce(false)
	if !player.closed || !player.stopped || len(player.held) != 0 || output.queue.availableBytes() != 0 || output.player != nil {
		t.Fatal("guest mute marker left transferred PCM or a reading player alive")
	}
	if output.generation != 2 {
		t.Fatalf("generation = %d, want 2", output.generation)
	}
	// An inactive control marker must not initialize a device or a new player.
	empty := activeAudioShell(&audioControlBackend{chunks: []AudioChunk{{Generation: 3}}}, nil)
	empty.drainAudioOnce(true)
	if empty.audioOutput != nil {
		t.Fatal("empty generation marker initialized an audio device")
	}
}
