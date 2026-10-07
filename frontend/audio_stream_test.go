package frontend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
	"time"
)

type bufferedTestPlayer struct {
	source   io.Reader
	held     []byte
	playing  bool
	stopped  bool
	closed   bool
	latency  time.Duration
	volume   float64
	position time.Duration
	closeErr error
}

func (p *bufferedTestPlayer) Play()                         { p.playing = true }
func (p *bufferedTestPlayer) PauseAndStopReading()          { p.playing = false; p.stopped = true }
func (p *bufferedTestPlayer) IsPlaying() bool               { return p.playing }
func (p *bufferedTestPlayer) SetBufferSize(d time.Duration) { p.latency = d }
func (p *bufferedTestPlayer) SetVolume(v float64)           { p.volume = v }
func (p *bufferedTestPlayer) Position() time.Duration       { return p.position }
func (p *bufferedTestPlayer) Close() error                  { p.closed = true; p.held = nil; return p.closeErr }
func (p *bufferedTestPlayer) readAhead(frames int) {
	p.held = make([]byte, frames*4)
	_, _ = p.source.Read(p.held)
}

func TestAudioGenerationReplacesPlayerAndItsReadAhead(t *testing.T) {
	settings := AudioSettings{Latency: 20 * time.Millisecond, Volume: 75}
	var players []*bufferedTestPlayer
	output := newAudioOutputWithFactory(settings, func(source io.Reader) (hostAudioPlayer, error) {
		player := &bufferedTestPlayer{source: source}
		players = append(players, player)
		return player, nil
	})
	chunk := func(generation uint64, sample int16) AudioChunk {
		pcm := make([]int16, hostAudioSampleRate/50)
		for i := range pcm {
			pcm[i] = sample
		}
		return AudioChunk{SampleRate: hostAudioSampleRate, Channels: 1, PCM16: pcm, Generation: generation}
	}
	if err := output.enqueue(chunk(1, 10000), 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 {
		t.Fatalf("players=%d", len(players))
	}
	old := players[0]
	old.readAhead(441)
	if output.telemetry().PlayerBufferedFrames != 441 {
		t.Fatalf("player buffering=%+v", output.telemetry())
	}
	oldQueue := output.queue
	if err := output.enqueue(chunk(2, -10000), 0, 0); err != nil {
		t.Fatal(err)
	}
	if !old.stopped || !old.closed || len(old.held) != 0 {
		t.Fatal("old read-ahead player survived generation change")
	}
	if _, err := oldQueue.Read(make([]byte, 4)); err != io.EOF {
		t.Fatalf("retired queue remains readable: %v", err)
	}
	if len(players) != 2 || players[1].volume != 0.75 || players[1].latency != settings.Latency {
		t.Fatalf("replacement did not inherit settings: %+v", players)
	}
	players[1].readAhead(1)
	if sample := int16(binary.LittleEndian.Uint16(players[1].held)); sample != -10000 {
		t.Fatalf("old PCM crossed generation: %d", sample)
	}
	output.flush()
	if len(players) != 2 || output.player != nil {
		t.Fatal("flush created an idle player that can read ahead")
	}
	settings.Muted = true
	output.configure(settings)
	if err := output.enqueue(chunk(3, 20000), 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(players) != 3 || players[2].volume != 0 {
		t.Fatal("muted settings lost after recreation")
	}
	if err := output.close(); err != nil {
		t.Fatal(err)
	}
}

func TestAudioPlayerCreationFailureIsReportedAndRetryable(t *testing.T) {
	expected := errors.New("synthetic device unavailable")
	calls := 0
	output := newAudioOutputWithFactory(AudioSettings{Latency: 20 * time.Millisecond}, func(source io.Reader) (hostAudioPlayer, error) {
		calls++
		if calls == 1 {
			return nil, expected
		}
		return &bufferedTestPlayer{source: source}, nil
	})
	chunk := AudioChunk{SampleRate: hostAudioSampleRate, Channels: 1, PCM16: make([]int16, 882)}
	if err := output.enqueue(chunk, 0, 0); !errors.Is(err, expected) {
		t.Fatalf("creation error=%v", err)
	}
	output.startIfReady(time.Now())
	if !output.started || calls != 2 {
		t.Fatalf("creation not retried: calls=%d started=%t", calls, output.started)
	}
}

func TestStreamingResamplingIsIndependentOfFragmentSize(t *testing.T) {
	for _, rate := range []int{8000, 22050, 42336, 44100, 48000, 192000} {
		for _, channels := range []int{1, 2} {
			data := make([]int16, 1001*channels)
			for i := range data {
				data[i] = int16(math.Sin(float64(i)*0.31) * 12000)
			}
			var whole hostPCMEncoder
			want, err := whole.encode(AudioChunk{SampleRate: rate, Channels: channels, PCM16: data})
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []int{1, 20, 441, 706} {
				var encoder hostPCMEncoder
				var got []byte
				for first := 0; first < 1001; first += fragment {
					last := min(first+fragment, 1001)
					part, err := encoder.encode(AudioChunk{SampleRate: rate, Channels: channels, PCM16: data[first*channels : last*channels]})
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, part...)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("rate=%d channels=%d fragment=%d: %d != %d frames or changed samples", rate, channels, fragment, len(got)/4, len(want)/4)
				}
			}
			expected := (1001*hostAudioSampleRate + rate - 1) / rate
			if len(want)/4 != expected {
				t.Fatalf("rate=%d output frames=%d want=%d", rate, len(want)/4, expected)
			}
		}
	}
}

func TestPCMBacklogRecoveryBudgetDoesNotDependOnChunkCount(t *testing.T) {
	run := func(fragment int) (int, uint64) {
		queue := newPCMQueue(hostAudioSampleRate * 4)
		queue.setTargetBytes(441 * 4)
		queue.data = make([]byte, 4410*4)
		queue.trimming = true
		for first := 0; first < 3528; first += fragment {
			frames := min(fragment, 3528-first)
			queue.enqueue(make([]byte, frames*4))
			if _, err := queue.Read(make([]byte, frames*4)); err != nil {
				t.Fatal(err)
			}
		}
		return queue.availableBytes(), queue.telemetry().DroppedFrames
	}
	wantFill, wantDropped := run(3528)
	for _, fragment := range []int{1, 20, 441, 706} {
		fill, dropped := run(fragment)
		if fill != wantFill || dropped != wantDropped {
			t.Fatalf("fragment=%d fill=%d dropped=%d; whole fill=%d dropped=%d", fragment, fill, dropped, wantFill, wantDropped)
		}
	}
}

func TestAudioFocusDoesNotOverrideBackgroundLifecycle(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.SetHostActive(false)
	shell.SetAudioFocus(false)
	shell.SetAudioFocus(true)
	shell.syncHostLifecycle()
	if shell.hostActive || !shell.audioSuspended {
		t.Fatal("focus gain resumed background audio")
	}
	shell.SetHostActive(true)
	shell.SetAudioFocus(false)
	shell.syncHostLifecycle()
	if shell.hostActive {
		t.Fatal("foreground gain overrode focus loss")
	}
	shell.SetAudioFocus(true)
	shell.syncHostLifecycle()
	if !shell.hostActive {
		t.Fatal("both gates active did not restore host activity")
	}
}

func TestFocusLossStopsAudioBeforeTheNextUpdate(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	var player *bufferedTestPlayer
	shell.audioOutput = newAudioOutputWithFactory(AudioSettings{Latency: 20 * time.Millisecond}, func(source io.Reader) (hostAudioPlayer, error) {
		player = &bufferedTestPlayer{source: source}
		return player, nil
	})
	if err := shell.audioOutput.enqueue(AudioChunk{SampleRate: hostAudioSampleRate, Channels: 1, PCM16: make([]int16, 882)}, 0, 0); err != nil {
		t.Fatal(err)
	}
	if player == nil || !player.playing {
		t.Fatal("fixture did not start playback")
	}
	shell.SetAudioFocus(false)
	if !shell.audioSuspended || !player.stopped || !player.closed || shell.audioOutput.player != nil {
		t.Fatal("native focus loss waited for the suspended game loop to flush audio")
	}
}

func TestLatencyPresetsKeepTheCustomSlider(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	ui := &shellUI{settingsSection: "Audio"}
	var preset *settingsDropdownModel
	custom := false
	for _, row := range ui.settingsRowModels(shell) {
		if row.label == "Latency preset" {
			preset = row.dropdown
		}
		if row.label == "Requested latency" && (row.dropdown != nil || row.slider != nil) {
			custom = true
		}
		if row.label == "Music and effects" && (row.action != nil || row.value != shell.tr("Automatic")) {
			t.Fatal("automatic mixing has a misleading toggle")
		}

	}
	if preset == nil || !custom {
		t.Fatal("presets replaced the custom latency control")
	}
	for index, want := range []int{20, 60, 120} {
		preset.apply(index)
		if shell.settings.AudioLatencyMS != want || preset.value() != index {
			t.Fatalf("preset %d=%dms want=%d", index, shell.settings.AudioLatencyMS, want)
		}
	}
	shell.setAudioLatency(80)
	if preset.value() != 3 {
		t.Fatal("custom latency not represented")
	}
}
