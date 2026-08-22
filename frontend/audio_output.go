package frontend

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const hostAudioSampleRate = 44_100

// pcmQueue is an infinite PCM stream from the player's point of view. An
// underrun produces silence, while an overrun drops the oldest complete stereo
// frames so audio stays synchronized with current guest time.
type pcmQueue struct {
	mu       sync.Mutex
	data     []byte
	offset   int
	maxBytes int
	closed   bool
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
	copy(destination, q.data[q.offset:q.offset+count])
	clear(destination[count:])
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
		if excess >= len(q.data) {
			q.data = q.data[:0]
		} else {
			copy(q.data, q.data[excess:])
			q.data = q.data[:len(q.data)-excess]
		}
	}
}

func (q *pcmQueue) availableBytes() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data) - q.offset
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
	q.offset += drop
}

func (q *pcmQueue) flush() {
	q.mu.Lock()
	q.data = q.data[:0]
	q.offset = 0
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
	softenEnabled  bool
	lp             [2]float64 // per-channel one-pole low-pass state
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
	player, err := context.NewPlayer(queue)
	if err != nil {
		queue.close()
		return nil, err
	}
	output := &audioOutput{queue: queue, player: player}
	output.configure(settings)
	return output, nil
}

func (o *audioOutput) configure(settings AudioSettings) {
	if o == nil || o.player == nil {
		return
	}
	latency := settings.Latency
	if latency < 20*time.Millisecond {
		latency = 20 * time.Millisecond
	}
	if latency > 250*time.Millisecond {
		latency = 250 * time.Millisecond
	}
	o.queue.setMaxBytes(audioQueueBytes(latency))
	// The queue's steady-state fill sits at the prebuffer level, so that is the
	// real stall tolerance: if the emulation goroutine stalls (a heavy "flash"
	// frame) for longer than this, the queue underruns and the sound stutters.
	// Buffer twice the requested latency so a spike up to ~2x latency is
	// absorbed, with a floor so a low latency setting still tolerates a frame or
	// two. audioQueueBytes keeps the ceiling above this.
	prebuffer := 2 * latency
	if prebuffer < 150*time.Millisecond {
		prebuffer = 150 * time.Millisecond
	}
	o.prebufferBytes = alignStereoFrame(int(
		int64(hostAudioSampleRate*4) * int64(prebuffer) / int64(time.Second),
	))
	o.prebufferWait = latency
	o.player.SetBufferSize(latency)
	o.softenEnabled = settings.Soften
	volume := float64(settings.Volume) / 100
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	if settings.Muted {
		volume = 0
	}
	o.player.SetVolume(volume)
}

func (o *audioOutput) enqueue(chunk AudioChunk) error {
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
}

func (o *audioOutput) flush() {
	if o == nil || o.queue == nil {
		return
	}
	if o.player != nil && o.player.IsPlaying() {
		o.player.Pause()
	}
	o.queue.flush()
	o.lastEnqueue = time.Time{}
	o.started = false
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
	if latency < 20*time.Millisecond {
		latency = 20 * time.Millisecond
	}
	// Keep enough producer headroom for scheduling jitter, but never enough to
	// let stale sound trail the video for a noticeable amount of time.
	queueDuration := latency * 3
	// The ceiling must stay above the prebuffer level (2x latency, floored at
	// 100ms) so the queue can actually hold the steady-state buffer without
	// immediately dropping it as overrun. The floor gives a heavy-frame spike
	// room to be absorbed; the cap keeps stale sound from trailing the video.
	if queueDuration < 220*time.Millisecond {
		queueDuration = 220 * time.Millisecond
	}
	if queueDuration > 700*time.Millisecond {
		queueDuration = 700 * time.Millisecond
	}
	return int(int64(hostAudioSampleRate*4) * int64(queueDuration) / int64(time.Second))
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
