package frontend

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const hostAudioSampleRate = 44_100

// AudioQueueTelemetry exposes host-playback buffering separately from guest
// audio generation. Counts are cumulative for the life of the output; fill,
// target, and capacity are current host-rate stereo frames.
type AudioQueueTelemetry struct {
	Underruns      uint64 `json:"underruns"`
	MissingSamples uint64 `json:"missing_samples"`
	Overruns       uint64 `json:"overruns"`
	DroppedFrames  uint64 `json:"dropped_frames"`
	StaleFrames    uint64 `json:"stale_frames"`
	FillFrames     int    `json:"fill_frames"`
	TargetFrames   int    `json:"target_frames"`
	CapacityFrames int    `json:"capacity_frames"`
	Started        bool   `json:"started"`
}

// pcmQueue is an infinite PCM stream from the player's point of view. An
// underrun produces silence, while an overrun drops the oldest complete stereo
// frames so audio stays synchronized with current guest time.
type pcmQueue struct {
	mu       sync.Mutex
	data     []byte
	offset   int
	maxBytes int
	// targetBytes is the depth the queue settles at. maxBytes alone only
	// bounds how bad the lag can get; without a target the queue drifts up to
	// the cap and stays there, so the player hears everything that far behind.
	// See trimToTarget.
	targetBytes   int
	closed        bool
	underruns     uint64
	missingBytes  uint64
	overruns      uint64
	droppedFrames uint64
	// Declick state. lastOut is the last sample handed to the player, residual
	// is the step left over at a seam, and the two flags say where the next
	// seam is: silent tracks whether the previous frame was fill, spliceNext is
	// set when the queue itself discarded audio.
	lastOut    [2]int32
	residual   [2]int32
	silent     bool
	spliceNext bool
}

func newPCMQueue(maxBytes int) *pcmQueue {
	return &pcmQueue{maxBytes: alignStereoFrame(maxBytes)}
}

func (q *pcmQueue) Read(destination []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed && q.offset >= len(q.data) {
		return 0, io.EOF
	}
	available := len(q.data) - q.offset
	count := min(len(destination), available)
	// Only whole stereo frames are handed over, so a seam always falls on a
	// frame boundary and the declick below can reason in frames.
	count -= count % 4
	copy(destination, q.data[q.offset:q.offset+count])
	clear(destination[count:])
	if missing := len(destination) - count; missing > 0 {
		q.underruns++
		q.missingBytes += uint64(missing)
	}
	q.declick(destination, count)
	q.offset += count
	if q.offset == len(q.data) {
		q.data = q.data[:0]
		q.offset = 0
	} else if q.offset >= 64*1024 && q.offset*2 >= len(q.data) {
		q.data = append(q.data[:0], q.data[q.offset:]...)
		q.offset = 0
	}
	return len(destination), nil
}

// declick smooths every seam the player is about to hear: the frame where the
// producer ran out, the frame where it came back, and any frame where the queue
// itself discarded buffered sound.
//
// A seam is a step in the waveform, and a step is exactly the click that reads
// as crackle - the ear is far more sensitive to the discontinuity than to the
// missing milliseconds around it. Carrying the step forward as an offset that
// decays over a fraction of a millisecond turns it into a slope, so a short
// starvation sounds like a dip rather than a tick. realBytes is how much of
// destination is genuine audio; anything past it is fill.
func (q *pcmQueue) declick(destination []byte, realBytes int) {
	frames := len(destination) / 4
	realFrames := realBytes / 4
	if frames == 0 {
		return
	}
	if q.settled(frames, realFrames) {
		last := (min(realFrames, frames) - 1) * 4
		if last < 0 {
			return
		}
		q.lastOut[0] = int32(int16(binary.LittleEndian.Uint16(destination[last:])))
		q.lastOut[1] = int32(int16(binary.LittleEndian.Uint16(destination[last+2:])))
		return
	}
	for frame := 0; frame < frames; frame++ {
		silent := frame >= realFrames
		seam := silent != q.silent || (!silent && q.spliceNext)
		q.silent = silent
		if !silent {
			q.spliceNext = false
		}
		base := frame * 4
		for channel := 0; channel < 2; channel++ {
			at := base + channel*2
			sample := int32(int16(binary.LittleEndian.Uint16(destination[at:])))
			if seam {
				q.residual[channel] = q.lastOut[channel] - sample
			}
			value := clampPCM16(sample + q.residual[channel])
			binary.LittleEndian.PutUint16(destination[at:], uint16(value))
			q.lastOut[channel] = int32(value)
			q.residual[channel] = q.residual[channel] *
				declickDecayNumerator / declickDecayDenominator
		}
	}
}

// settled reports that this buffer holds no seam and carries no correction, so
// the samples can go to the player untouched. Whole seconds of playback are
// this case, and the ear only ever hears the exceptions.
func (q *pcmQueue) settled(frames, realFrames int) bool {
	return q.residual == [2]int32{} &&
		!q.spliceNext &&
		q.silent == (realFrames == 0) &&
		(realFrames == 0 || realFrames >= frames)
}

// declickDecay shapes how fast a seam correction fades. 15/16 per frame is
// about a third of a millisecond at 44.1 kHz: long enough to turn a step into a
// slope the ear reads as continuous, short enough that it displaces no audible
// content.
const (
	declickDecayNumerator   = 15
	declickDecayDenominator = 16
)

func clampPCM16(value int32) int16 {
	if value > 32767 {
		return 32767
	}
	if value < -32768 {
		return -32768
	}
	return int16(value)
}

func (q *pcmQueue) enqueue(data []byte) {
	if len(data) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	if q.offset != 0 {
		q.data = append(q.data[:0], q.data[q.offset:]...)
		q.offset = 0
	}
	q.data = append(q.data, data...)
	if excess := len(q.data) - q.maxBytes; excess > 0 {
		excess = alignStereoFrameUp(excess)
		dropped := excess
		if excess >= len(q.data) {
			dropped = len(q.data)
			q.data = q.data[:0]
		} else {
			copy(q.data, q.data[excess:])
			q.data = q.data[:len(q.data)-excess]
		}
		q.overruns++
		q.droppedFrames += uint64(dropped / 4)
		q.spliceNext = true
	}
	q.trimToTarget()
}

// trimToTarget pulls a queue that has drifted above its target back down.
//
// The producer generates a frame's worth of audio at a time and the player
// drains continuously, so the depth wanders upward whenever the guest runs
// even slightly ahead. Capping it is not enough on its own: the cap is three
// times the latency the player asked for, so the queue settles near the cap
// and every sound arrives that late. 리듬스타1 (#148) is where this is
// audible - its notes are on screen well before the beat is heard - and the
// report bundle shows the depth sitting at a median of 5 880 frames and
// peaking at 9 555, against a 90 ms (3 969 frame) target.
//
// The correction is deliberately small. Trimming at most a millisecond per
// enqueue converges over roughly a second, which the ear reads as the sound
// tightening up rather than as a jump, and the seam goes through the same
// declick as any other discard. Slack keeps ordinary jitter from trimming at
// all, so a well-behaved stream is never touched.
func (q *pcmQueue) trimToTarget() {
	if q.targetBytes <= 0 {
		return
	}
	available := len(q.data) - q.offset
	ceiling := q.targetBytes + q.targetBytes/2
	if available <= ceiling {
		return
	}
	trim := alignStereoFrameUp(available - q.targetBytes)
	if limit := alignStereoFrameUp(hostAudioSampleRate * 4 / 1000); trim > limit {
		trim = limit
	}
	if trim <= 0 || trim >= available {
		return
	}
	q.offset += trim
	q.droppedFrames += uint64(trim / 4)
	q.spliceNext = true
}

// enqueueSilence appends a pause, easing out of whatever is already queued so
// the stream slopes into the gap instead of stepping into it.
func (q *pcmQueue) enqueueSilence(frames int) {
	if frames <= 0 {
		return
	}
	silence := make([]byte, frames*4)
	q.mu.Lock()
	tail := len(q.data) - 4
	if tail >= q.offset {
		for channel := 0; channel < 2; channel++ {
			value := int32(int16(binary.LittleEndian.Uint16(
				q.data[tail+channel*2:],
			)))
			for frame := 0; frame < frames && value != 0; frame++ {
				value = value * declickDecayNumerator / declickDecayDenominator
				binary.LittleEndian.PutUint16(
					silence[frame*4+channel*2:],
					uint16(int16(value)),
				)
			}
		}
	}
	q.mu.Unlock()
	q.enqueue(silence)
}

func (q *pcmQueue) availableBytes() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data) - q.offset
}

func (q *pcmQueue) setTargetBytes(targetBytes int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.targetBytes = alignStereoFrame(targetBytes)
}

func (q *pcmQueue) setMaxBytes(maxBytes int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.maxBytes = alignStereoFrame(maxBytes)
	available := len(q.data) - q.offset
	if available <= q.maxBytes {
		return
	}
	drop := alignStereoFrameUp(available - q.maxBytes)
	if drop > available {
		drop = available
	}
	q.offset += drop
	q.overruns++
	q.droppedFrames += uint64(drop / 4)
	q.spliceNext = true
}

func (q *pcmQueue) telemetry() AudioQueueTelemetry {
	q.mu.Lock()
	defer q.mu.Unlock()
	return AudioQueueTelemetry{
		Underruns:      q.underruns,
		MissingSamples: q.missingBytes / 2,
		Overruns:       q.overruns,
		DroppedFrames:  q.droppedFrames,
		FillFrames:     (len(q.data) - q.offset) / 4,
		CapacityFrames: q.maxBytes / 4,
	}
}

func (q *pcmQueue) flush() {
	q.mu.Lock()
	q.data = q.data[:0]
	q.offset = 0
	// Whatever plays next follows a discarded buffer, so it is a seam. lastOut
	// is deliberately kept: gliding from the sample the player actually heard is
	// the whole point.
	q.spliceNext = true
	q.mu.Unlock()
}

func (q *pcmQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.data = nil
	q.offset = 0
	q.mu.Unlock()
}

func alignStereoFrame(size int) int {
	if size < 4 {
		return 4
	}
	return size - size%4
}

func alignStereoFrameUp(size int) int {
	if remainder := size % 4; remainder != 0 {
		return size + 4 - remainder
	}
	return size
}

type audioOutput struct {
	queue          *pcmQueue
	player         *audio.Player
	prebufferBytes int
	prebufferWait  time.Duration
	lastEnqueue    time.Time
	started        bool
	generation     uint64
	nextSample     uint64
	staleFrames    uint64
	softenEnabled  bool
	lp             [2]float64 // per-channel one-pole low-pass state
	trace          *audioTrace
	lastSample     time.Time
}

// traceEvent records one pipeline event, snapshotting the current queue
// counters so the trace shows health at the moment of the event.
func (o *audioOutput) traceEvent(kind, detail string) {
	if o == nil || o.trace == nil {
		return
	}
	o.trace.record(kind, detail, o.telemetry())
}

// maybeSample records a health snapshot at most once per second so a steady
// problem (a queue underrunning with no control events) still leaves a trail.
func (o *audioOutput) maybeSample(now time.Time) {
	if o == nil || o.trace == nil {
		return
	}
	if !o.lastSample.IsZero() && now.Sub(o.lastSample) < time.Second {
		return
	}
	o.lastSample = now
	o.traceEvent("sample", "")
}

// softenCoefficient is the one-pole low-pass factor applied when Soften is on:
// y += a*(x-y). ~0.5 puts the corner near 5 kHz at 44.1 kHz, rounding off the
// FM synth's harsh top without muffling the melody.
const softenCoefficient = 0.5

// soften applies the low-pass in place per channel when enabled. The chunk is a
// fresh copy handed over by the backend, so mutating it is safe; the filter
// state persists across chunks to stay continuous.
func (o *audioOutput) soften(chunk *AudioChunk) {
	if !o.softenEnabled || len(chunk.PCM16) == 0 {
		return
	}
	channels := chunk.Channels
	if channels < 1 {
		channels = 1
	}
	for c := 0; c < channels && c < len(o.lp); c++ {
		y := o.lp[c]
		for i := c; i < len(chunk.PCM16); i += channels {
			y += softenCoefficient * (float64(chunk.PCM16[i]) - y)
			chunk.PCM16[i] = int16(y)
		}
		o.lp[c] = y
	}
}

func newAudioOutput(settings AudioSettings) (*audioOutput, error) {
	context := audio.CurrentContext()
	if context == nil {
		context = audio.NewContext(hostAudioSampleRate)
	}
	if context.SampleRate() != hostAudioSampleRate {
		return nil, errors.New("the existing host audio context does not use 44.1 kHz")
	}
	queue := newPCMQueue(audioQueueBytes(settings.Latency))
	queue.setTargetBytes(audioPrebufferBytes(settings.Latency))
	player, err := context.NewPlayer(queue)
	if err != nil {
		queue.close()
		return nil, err
	}
	output := &audioOutput{queue: queue, player: player}
	output.configure(settings)
	output.trace = newAudioTrace(fmt.Sprintf(
		"host_rate=%dHz latency=%s soften=%t volume=%d muted=%t mix=%t",
		hostAudioSampleRate,
		normalizedAudioLatency(settings.Latency),
		settings.Soften,
		settings.Volume,
		settings.Muted,
		settings.MixMode,
	))
	return output, nil
}

func (o *audioOutput) configure(settings AudioSettings) {
	if o == nil || o.player == nil {
		return
	}
	latency := normalizedAudioLatency(settings.Latency)
	o.queue.setMaxBytes(audioQueueBytes(latency))
	o.queue.setTargetBytes(audioPrebufferBytes(latency))
	// Requested latency is the actual steady-state start target. Capacity stays
	// larger so a producer spike can be absorbed without structurally delaying
	// every sound by a hidden 150 ms floor.
	o.prebufferBytes = audioPrebufferBytes(latency)
	o.prebufferWait = latency
	o.player.SetBufferSize(latency)
	o.softenEnabled = settings.Soften
	// Volume can exceed 100% for boosted output. oto multiplies samples by the
	// gain and the driver clips anything past full scale, so amplified audio may
	// distort but never panics.
	volume := float64(settings.Volume) / 100
	if volume < 0 {
		volume = 0
	}
	if volume > 2 {
		volume = 2
	}
	if settings.Muted {
		volume = 0
	}
	o.player.SetVolume(volume)
}

func (o *audioOutput) enqueue(
	chunk AudioChunk,
	videoGuestNS int64,
	videoGeneration uint64,
) error {
	if !o.synchronizeChunk(&chunk, videoGuestNS, videoGeneration) {
		return nil
	}
	o.soften(&chunk)
	encoded, err := encodeHostPCM(chunk)
	if err != nil {
		return err
	}
	o.queue.enqueue(encoded)
	if len(encoded) != 0 {
		o.lastEnqueue = time.Now()
	}
	if !o.started && o.queue.availableBytes() >= o.prebufferBytes {
		o.start()
	}
	return nil
}

func (o *audioOutput) synchronizeChunk(
	chunk *AudioChunk,
	videoGuestNS int64,
	videoGeneration uint64,
) bool {
	if chunk == nil || chunk.SampleRate <= 0 || chunk.Channels <= 0 ||
		len(chunk.PCM16) == 0 || len(chunk.PCM16)%chunk.Channels != 0 {
		return chunk != nil
	}
	frames := len(chunk.PCM16) / chunk.Channels
	if chunk.Generation == 0 {
		return true
	}
	if chunk.Generation != o.generation {
		o.flushInternal(fmt.Sprintf(
			"generation %d->%d", o.generation, chunk.Generation,
		))
		o.generation = chunk.Generation
		o.nextSample = chunk.StartSample
	}

	if videoGeneration == chunk.Generation {
		targetDuration := time.Duration(
			int64(o.prebufferBytes/4) * int64(time.Second) / hostAudioSampleRate,
		)
		targetGuestNS := videoGuestNS - int64(targetDuration)
		chunkEndNS := chunk.StartGuestNS + int64(
			time.Duration(int64(frames)*int64(time.Second)/int64(chunk.SampleRate)),
		)
		if chunkEndNS <= targetGuestNS {
			o.staleFrames += uint64(frames)
			o.nextSample = max(o.nextSample, chunk.StartSample+uint64(frames))
			o.traceEvent("stale", fmt.Sprintf("late-drop %d", frames))
			return false
		}
		if chunk.StartGuestNS < targetGuestNS {
			dropFrames := framesForDurationCeil(
				time.Duration(targetGuestNS-chunk.StartGuestNS),
				chunk.SampleRate,
			)
			if dropFrames > frames {
				dropFrames = frames
			}
			o.trimChunkPrefix(chunk, dropFrames)
			o.staleFrames += uint64(dropFrames)
			frames -= dropFrames
			// This gap is one we just made on purpose. The audio already
			// queued in front of it is contiguous - only as late - so move the
			// cursor with the trim, or the sample-continuity check below reads
			// the trim as the producer skipping forward and flushes a healthy
			// queue: a 5 ms trim would cost the whole buffer and a restart.
			o.nextSample = max(o.nextSample, chunk.StartSample)
		}
	}
	if frames == 0 {
		return false
	}

	if chunk.StartSample < o.nextSample {
		overlap := o.nextSample - chunk.StartSample
		if overlap >= uint64(frames) {
			o.staleFrames += uint64(frames)
			o.traceEvent("stale", fmt.Sprintf("overlap-drop %d", frames))
			return false
		}
		o.trimChunkPrefix(chunk, int(overlap))
		o.staleFrames += overlap
		frames -= int(overlap)
	} else if chunk.StartSample > o.nextSample {
		o.bridgeGap(chunk.StartSample-o.nextSample, chunk.SampleRate)
	}
	o.nextSample = chunk.StartSample + uint64(frames)
	return true
}

// bridgeGap absorbs a pause the producer left in the stream.
//
// Restarting host playback for this was the wrong reflex. A flush threw away
// every millisecond already buffered ahead of the gap and then waited for a
// fresh prebuffer, so a pause the guest meant to be short became a much longer
// hole with a click at each end - and the mixer's own rounding used to open a
// one-sample "gap" tens of times a second, which is what made continuous music
// break up. Anything the queue can hold is filled with silence so playback
// keeps its timeline; a longer pause is simply left to drain, which sounds the
// same and costs nothing, because the player consumes in real time and there is
// nothing else to give it.
func (o *audioOutput) bridgeGap(frames uint64, sampleRate int) {
	if frames == 0 || sampleRate <= 0 {
		return
	}
	hostFrames := frames * hostAudioSampleRate / uint64(sampleRate)
	if hostFrames == 0 || hostFrames*4 > uint64(o.queue.maxBytes) {
		return
	}
	o.queue.enqueueSilence(int(hostFrames))
	o.traceEvent("bridge", fmt.Sprintf("%d frames", hostFrames))
}

func (o *audioOutput) trimChunkPrefix(chunk *AudioChunk, frames int) {
	if frames <= 0 {
		return
	}
	chunk.PCM16 = chunk.PCM16[frames*chunk.Channels:]
	chunk.StartSample += uint64(frames)
	chunk.StartGuestNS += int64(
		time.Duration(int64(frames) * int64(time.Second) / int64(chunk.SampleRate)),
	)
}

func framesForDurationCeil(duration time.Duration, sampleRate int) int {
	if duration <= 0 || sampleRate <= 0 {
		return 0
	}
	seconds := int64(duration / time.Second)
	remainder := int64(duration % time.Second)
	frames := seconds * int64(sampleRate)
	frames += (remainder*int64(sampleRate) + int64(time.Second) - 1) / int64(time.Second)
	return int(frames)
}

func (o *audioOutput) startIfReady(now time.Time) {
	if o == nil || o.started || o.queue.availableBytes() == 0 {
		return
	}
	if o.queue.availableBytes() >= o.prebufferBytes ||
		!o.lastEnqueue.IsZero() && now.Sub(o.lastEnqueue) >= o.prebufferWait {
		o.start()
	}
}

func (o *audioOutput) start() {
	if o == nil || o.player == nil || o.started {
		return
	}
	o.started = true
	o.player.Play()
	o.traceEvent("start", "")
}

func (o *audioOutput) flush() {
	o.flushInternal("external")
}

func (o *audioOutput) flushInternal(reason string) {
	if o == nil || o.queue == nil {
		return
	}
	if o.player != nil && o.player.IsPlaying() {
		o.player.Pause()
	}
	o.queue.flush()
	o.lastEnqueue = time.Time{}
	o.started = false
	o.lp = [2]float64{}
	o.generation = 0
	o.nextSample = 0
	o.traceEvent("flush", reason)
}

func (o *audioOutput) telemetry() AudioQueueTelemetry {
	if o == nil || o.queue == nil {
		return AudioQueueTelemetry{}
	}
	telemetry := o.queue.telemetry()
	telemetry.TargetFrames = o.prebufferBytes / 4
	telemetry.Started = o.started
	telemetry.StaleFrames = o.staleFrames
	return telemetry
}

func (o *audioOutput) close() error {
	if o == nil {
		return nil
	}
	o.queue.close()
	if o.player == nil {
		return nil
	}
	return o.player.Close()
}

func audioQueueBytes(latency time.Duration) int {
	latency = normalizedAudioLatency(latency)
	// Keep enough producer headroom for scheduling jitter, but never enough to
	// let stale sound trail the video for a noticeable amount of time.
	queueDuration := latency * 3
	// Keep the capacity above the latency-sized start target so a producer spike
	// has room to be absorbed. The cap keeps stale sound from trailing the video.
	if queueDuration < 220*time.Millisecond {
		queueDuration = 220 * time.Millisecond
	}
	if queueDuration > 700*time.Millisecond {
		queueDuration = 700 * time.Millisecond
	}
	return int(int64(hostAudioSampleRate*4) * int64(queueDuration) / int64(time.Second))
}

func normalizedAudioLatency(latency time.Duration) time.Duration {
	if latency < 20*time.Millisecond {
		return 20 * time.Millisecond
	}
	if latency > 250*time.Millisecond {
		return 250 * time.Millisecond
	}
	return latency
}

func audioPrebufferBytes(latency time.Duration) int {
	latency = normalizedAudioLatency(latency)
	return alignStereoFrame(int(
		int64(hostAudioSampleRate*4) * int64(latency) / int64(time.Second),
	))
}

func encodeHostPCM(chunk AudioChunk) ([]byte, error) {
	if len(chunk.PCM16) == 0 {
		return nil, nil
	}
	if chunk.SampleRate < 8_000 || chunk.SampleRate > 192_000 {
		return nil, errors.New("backend returned an invalid audio sample rate")
	}
	if chunk.Channels != 1 && chunk.Channels != 2 {
		return nil, errors.New("backend returned an unsupported audio channel count")
	}
	if len(chunk.PCM16)%chunk.Channels != 0 {
		return nil, errors.New("backend returned an incomplete PCM frame")
	}
	sourceFrames := len(chunk.PCM16) / chunk.Channels
	outputFrames := int((int64(sourceFrames)*hostAudioSampleRate +
		int64(chunk.SampleRate)/2) / int64(chunk.SampleRate))
	if outputFrames == 0 {
		return nil, nil
	}
	result := make([]byte, outputFrames*4)
	for outputFrame := 0; outputFrame < outputFrames; outputFrame++ {
		sourceFrame := int(int64(outputFrame) * int64(chunk.SampleRate) /
			hostAudioSampleRate)
		if sourceFrame >= sourceFrames {
			sourceFrame = sourceFrames - 1
		}
		source := sourceFrame * chunk.Channels
		left := chunk.PCM16[source]
		right := left
		if chunk.Channels == 2 {
			right = chunk.PCM16[source+1]
		}
		destination := outputFrame * 4
		binary.LittleEndian.PutUint16(result[destination:], uint16(left))
		binary.LittleEndian.PutUint16(result[destination+2:], uint16(right))
	}
	return result, nil
}
