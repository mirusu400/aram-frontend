package frontend

import (
	"encoding/binary"
	"io"
	"testing"
	"time"
)

func TestEncodeHostPCMConvertsMonoAndSampleRate(t *testing.T) {
	encoded, err := encodeHostPCM(AudioChunk{
		SampleRate: 22_050,
		Channels:   1,
		PCM16:      []int16{100, -200},
	})
	if err != nil {
		t.Fatalf("encodeHostPCM: %v", err)
	}
	if len(encoded) != 4*4 {
		t.Fatalf("encoded bytes = %d, want 16", len(encoded))
	}
	want := []int16{100, 100, 100, 100, -200, -200, -200, -200}
	for index, expected := range want {
		got := int16(binary.LittleEndian.Uint16(encoded[index*2:]))
		if got != expected {
			t.Fatalf("sample %d = %d, want %d", index, got, expected)
		}
	}
}

func TestPCMQueuePadsUnderrunWithSilence(t *testing.T) {
	queue := newPCMQueue(32)
	queue.enqueue([]byte{1, 2, 3, 4})
	destination := make([]byte, 8)
	count, err := queue.Read(destination)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if count != len(destination) {
		t.Fatalf("Read count = %d, want %d", count, len(destination))
	}
	want := []byte{1, 2, 3, 4, 0, 0, 0, 0}
	for index := range want {
		if destination[index] != want[index] {
			t.Fatalf("byte %d = %d, want %d", index, destination[index], want[index])
		}
	}
	telemetry := queue.telemetry()
	if telemetry.Underruns != 1 || telemetry.MissingSamples != 2 {
		t.Fatalf("underrun telemetry = %+v, want 1 event and 2 samples", telemetry)
	}
}

func TestPCMQueueDropsOldestStereoFrames(t *testing.T) {
	queue := newPCMQueue(8)
	queue.enqueue([]byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3})
	if got := queue.availableBytes(); got != 8 {
		t.Fatalf("available bytes = %d, want 8", got)
	}
	telemetry := queue.telemetry()
	if telemetry.Overruns != 1 || telemetry.DroppedFrames != 1 {
		t.Fatalf("overrun telemetry = %+v, want 1 event and 1 stereo frame", telemetry)
	}
	if telemetry.FillFrames != 2 || telemetry.CapacityFrames != 2 {
		t.Fatalf("queue fill telemetry = %+v, want 2/2 frames", telemetry)
	}
	destination := make([]byte, 8)
	if _, err := queue.Read(destination); err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := []byte{2, 2, 2, 2, 3, 3, 3, 3}
	for index := range want {
		if destination[index] != want[index] {
			t.Fatalf("byte %d = %d, want %d", index, destination[index], want[index])
		}
	}
	if got := queue.availableBytes(); got != 0 {
		t.Fatalf("available bytes after read = %d, want 0", got)
	}
	queue.close()
	if _, err := queue.Read(make([]byte, 4)); err != io.EOF {
		t.Fatalf("closed empty queue error = %v, want EOF", err)
	}
}

func TestAudioQueueBytesBoundsLatency(t *testing.T) {
	low := audioQueueBytes(20 * time.Millisecond)
	high := audioQueueBytes(250 * time.Millisecond)
	if low != hostAudioSampleRate*4*220/1000 {
		t.Fatalf("low-latency queue = %d", low)
	}
	if high != hostAudioSampleRate*4*700/1000 {
		t.Fatalf("high-latency queue = %d", high)
	}
}

func TestConfiguredLatencyMatchesAudioStartThreshold(t *testing.T) {
	for _, latency := range []time.Duration{
		20 * time.Millisecond,
		60 * time.Millisecond,
		250 * time.Millisecond,
	} {
		target := audioPrebufferBytes(latency)
		want := alignStereoFrame(int(
			int64(hostAudioSampleRate*4) * int64(latency) / int64(time.Second),
		))
		if target != want {
			t.Fatalf("%s start threshold = %d bytes, want %d", latency, target, want)
		}
		if capacity := audioQueueBytes(latency); capacity <= target {
			t.Fatalf("%s queue capacity = %d, want more than threshold %d", latency, capacity, target)
		}
	}
}

func TestAudioOutputFlushDropsAStaleImpulse(t *testing.T) {
	output := &audioOutput{
		queue:   newPCMQueue(64),
		started: true,
		lp:      [2]float64{100, -100},
	}
	output.queue.enqueue([]byte{0xff, 0x7f, 0x00, 0x80})

	output.flush()
	if got := output.queue.availableBytes(); got != 0 {
		t.Fatalf("flush left %d stale bytes", got)
	}
	if output.started {
		t.Fatal("flush left playback marked as started")
	}
	if output.lp != [2]float64{} {
		t.Fatalf("flush retained low-pass history %+v", output.lp)
	}

	output.queue.enqueue(make([]byte, 4))
	destination := make([]byte, 4)
	if _, err := output.queue.Read(destination); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(destination); got != 0 {
		t.Fatalf("stale impulse survived flush: %#04x", got)
	}
}

func TestAudioDiscontinuityCommandsAreExplicit(t *testing.T) {
	discontinuous := []BackendCommand{
		CommandStart,
		CommandPauseResume,
		CommandStop,
		CommandReset,
		CommandFastForward,
		CommandLoadState,
		CommandRewind,
	}
	for _, command := range discontinuous {
		if !isAudioDiscontinuityCommand(command) {
			t.Errorf("%s was not classified as an audio discontinuity", command)
		}
	}
	for _, command := range []BackendCommand{CommandFrame, CommandSaveState} {
		if isAudioDiscontinuityCommand(command) {
			t.Errorf("%s unnecessarily flushes continuous audio", command)
		}
	}
}

func TestAudioDiscontinuitySuspendsDrainUntilRunning(t *testing.T) {
	output := &audioOutput{queue: newPCMQueue(64), started: true}
	output.queue.enqueue([]byte{0xff, 0x7f, 0x00, 0x80})
	shell := &Shell{audioOutput: output}

	shell.beginAudioDiscontinuity()
	if !shell.audioSuspended {
		t.Fatal("discontinuity did not suspend audio drain")
	}
	if got := output.queue.availableBytes(); got != 0 {
		t.Fatalf("discontinuity left %d stale bytes", got)
	}

	output.queue.enqueue([]byte{1, 2, 3, 4})
	shell.finishAudioDiscontinuity(StatePaused)
	if !shell.audioSuspended || output.queue.availableBytes() != 0 {
		t.Fatal("paused discontinuity resumed or retained audio")
	}
	shell.finishAudioDiscontinuity(StateRunning)
	if shell.audioSuspended {
		t.Fatal("running state did not resume audio drain")
	}
}

func TestAudioOutputDropsStaleTimestampedPCM(t *testing.T) {
	const latency = 60 * time.Millisecond
	output := &audioOutput{
		queue:          newPCMQueue(audioQueueBytes(latency)),
		prebufferBytes: audioPrebufferBytes(latency),
		prebufferWait:  latency,
	}
	frames := func(duration time.Duration) []int16 {
		count := int(int64(hostAudioSampleRate) * int64(duration) / int64(time.Second))
		return make([]int16, count*2)
	}

	// At guest 200 ms the 60 ms target starts at 140 ms. This first chunk
	// ended at 100 ms and must never become a late backlog.
	if err := output.enqueue(AudioChunk{
		SampleRate:   hostAudioSampleRate,
		Channels:     2,
		PCM16:        frames(100 * time.Millisecond),
		StartGuestNS: 0,
		StartSample:  0,
		Generation:   4,
	}, int64(200*time.Millisecond), 4); err != nil {
		t.Fatal(err)
	}
	if got := output.queue.availableBytes(); got != 0 {
		t.Fatalf("fully stale chunk queued %d bytes", got)
	}

	// A chunk crossing the target keeps only the portion at or after 140 ms.
	if err := output.enqueue(AudioChunk{
		SampleRate:   hostAudioSampleRate,
		Channels:     2,
		PCM16:        frames(40 * time.Millisecond),
		StartGuestNS: int64(120 * time.Millisecond),
		StartSample:  uint64(hostAudioSampleRate * 120 / 1000),
		Generation:   4,
	}, int64(200*time.Millisecond), 4); err != nil {
		t.Fatal(err)
	}
	telemetry := output.telemetry()
	if telemetry.FillFrames != hostAudioSampleRate*20/1000 {
		t.Fatalf("trimmed fill = %+v, want 20 ms", telemetry)
	}
	if telemetry.StaleFrames != hostAudioSampleRate*120/1000 {
		t.Fatalf("stale-frame telemetry = %+v, want 120 ms", telemetry)
	}
}

func TestAudioOutputFlushesWhenGenerationChanges(t *testing.T) {
	output := &audioOutput{
		queue:          newPCMQueue(64 * 1024),
		prebufferBytes: 4,
	}
	first := AudioChunk{
		SampleRate: 44_100,
		Channels:   2,
		PCM16:      make([]int16, 20),
		Generation: 8,
	}
	if err := output.enqueue(first, 0, 8); err != nil {
		t.Fatal(err)
	}
	if err := output.enqueue(AudioChunk{
		SampleRate:   44_100,
		Channels:     2,
		PCM16:        make([]int16, 4),
		StartGuestNS: int64(time.Second),
		Generation:   9,
	}, int64(time.Second), 9); err != nil {
		t.Fatal(err)
	}
	if got := output.queue.availableBytes(); got != 8 {
		t.Fatalf("generation change retained old PCM: %d bytes", got)
	}
	if output.generation != 9 {
		t.Fatalf("output generation = %d, want 9", output.generation)
	}
}

// A stale prefix trim is a gap the output makes on purpose. The audio already
// queued in front of it is contiguous, only as late, so the trim must not be
// mistaken for the producer skipping forward: that flushed the whole buffer and
// restarted playback, turning a 5 ms trim into a latency-sized dropout.
func TestTrimmingAStalePrefixKeepsTheHealthyQueue(t *testing.T) {
	const latency = 60 * time.Millisecond
	output := &audioOutput{
		queue:          newPCMQueue(audioQueueBytes(latency)),
		prebufferBytes: audioPrebufferBytes(latency),
		prebufferWait:  latency,
	}
	frames := func(duration time.Duration) []int16 {
		count := int(int64(hostAudioSampleRate) * int64(duration) / int64(time.Second))
		return make([]int16, count*2)
	}
	sample := func(at time.Duration) uint64 {
		return uint64(int64(hostAudioSampleRate) * int64(at) / int64(time.Second))
	}

	// Steady state: a latency-sized buffer of guest audio contiguous to 200 ms.
	if err := output.enqueue(AudioChunk{
		SampleRate:   hostAudioSampleRate,
		Channels:     2,
		PCM16:        frames(60 * time.Millisecond),
		StartGuestNS: int64(140 * time.Millisecond),
		StartSample:  sample(140 * time.Millisecond),
		Generation:   4,
	}, int64(200*time.Millisecond), 4); err != nil {
		t.Fatal(err)
	}
	healthy := output.queue.availableBytes()
	if healthy == 0 {
		t.Fatal("steady-state chunk was not queued")
	}
	// The first chunk established the generation, which resets playback.
	output.started = true

	// The producer hiccups: only a 5 ms prefix of the next chunk is stale.
	if err := output.enqueue(AudioChunk{
		SampleRate:   hostAudioSampleRate,
		Channels:     2,
		PCM16:        frames(20 * time.Millisecond),
		StartGuestNS: int64(200 * time.Millisecond),
		StartSample:  sample(200 * time.Millisecond),
		Generation:   4,
	}, int64(265*time.Millisecond), 4); err != nil {
		t.Fatal(err)
	}
	if got := output.queue.availableBytes(); got < healthy {
		t.Fatalf("a 5 ms stale trim dropped the queue: %d bytes left of %d", got, healthy)
	}
	if !output.started {
		t.Fatal("a 5 ms stale trim restarted playback")
	}
}
