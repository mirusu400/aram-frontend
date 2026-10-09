package frontend

import (
	"strings"
	"testing"
	"time"
)

func TestHostAudioBufferPlanBudgetsQueueAndPlayer(t *testing.T) {
	for _, test := range []struct {
		name       string
		properties HostAudioProperties
		latency    time.Duration
	}{
		{"48kHz burst", HostAudioProperties{48_000, 192}, 60 * time.Millisecond},
		{"44.1kHz burst", HostAudioProperties{44_100, 256}, 60 * time.Millisecond},
		{"minimum latency", HostAudioProperties{48_000, 192}, 20 * time.Millisecond},
		{"large burst", HostAudioProperties{48_000, 960}, 60 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := planAudioBuffers(test.latency, test.properties)
			burst := test.properties.burst()
			total := plan.queueTarget + plan.player
			if total < test.latency || total >= test.latency+2*burst {
				t.Fatalf("total %s for budget %s burst %s", total, test.latency, burst)
			}
			if plan.queueTarget%burst != 0 || plan.player%burst != 0 {
				t.Fatalf("buffers %+v are not aligned to %s", plan, burst)
			}
			if plan.pump < 2*time.Millisecond || plan.pump > 8*time.Millisecond {
				t.Fatalf("unbounded pump interval %s", plan.pump)
			}
		})
	}
}

func TestAudioTraceCorrelatesFrameWorkWithoutFlooding(t *testing.T) {
	output := newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	result := frameRunResult{workFrames: 1, workTotal: time.Millisecond, workMax: time.Millisecond}
	now := time.Unix(10, 0)
	for frame := range 60 {
		if frame == 30 {
			output.sampleFrameWork(frameRunResult{workFrames: 1, workTotal: 80 * time.Millisecond, workMax: 80 * time.Millisecond}, now.Add(500*time.Millisecond))
		}
		output.sampleFrameWork(result, now.Add(time.Duration(frame)*time.Second/60))
	}
	if len(output.trace.entries) != 0 {
		t.Fatalf("frame work was logged before the complete window: %d", len(output.trace.entries))
	}
	output.sampleFrameWork(result, now.Add(time.Second))
	if len(output.trace.entries) != 1 {
		t.Fatalf("frame events flooded the trace: %d", len(output.trace.entries))
	}
	detail := output.trace.entries[0].detail
	for _, want := range []string{"work_frames=62", "total=141ms", "max=80ms"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing %s in aggregate: %s", want, detail)
		}
	}
	output.sampleFrameWork(result, now.Add(1100*time.Millisecond))
	output.sampleFrameWork(result, now.Add(2100*time.Millisecond))
	if len(output.trace.entries) != 2 || !strings.Contains(output.trace.entries[1].detail, "max=1ms") {
		t.Fatalf("the previous window's slow frame leaked into the next aggregate")
	}
}

func TestAudioTraceFlushKeepsIncompleteFrameWindow(t *testing.T) {
	output := newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	now := time.Now().Add(-750 * time.Millisecond)
	output.sampleFrameWork(frameRunResult{workFrames: 1, workTotal: time.Millisecond, workMax: time.Millisecond}, now)
	output.sampleFrameWork(frameRunResult{workFrames: 1, workTotal: 80 * time.Millisecond, workMax: 80 * time.Millisecond}, now.Add(500*time.Millisecond))
	output.flush()
	trace := string(output.trace.render())
	if !strings.Contains(trace, "work_frames=2") || !strings.Contains(trace, "total=81ms") || !strings.Contains(trace, "max=80ms") {
		t.Fatalf("flushing discarded the incomplete frame window: %s", trace)
	}
}

func TestAudioTraceExportKeepsIncompleteFrameWindow(t *testing.T) {
	output := newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	output.sampleFrameWork(frameRunResult{workFrames: 1, workTotal: 80 * time.Millisecond, workMax: 80 * time.Millisecond}, time.Now())
	shell := &Shell{audioOutput: output}
	first := string(shell.audioTraceRender())
	second := string(shell.audioTraceRender())
	if !strings.Contains(first, "max=80ms") || first != second {
		t.Fatalf("export discarded or duplicated pending frame work: %s / %s", first, second)
	}
}

func TestHostAudioUnavailablePropertiesKeepPortableBuffers(t *testing.T) {
	for _, p := range []HostAudioProperties{{}, {0, 192}, {48_000, 0}, {48_000, -1}, {48_000, 1 << 30}, {1 << 30, 192}} {
		plan := planAudioBuffers(60*time.Millisecond, p)
		if plan.queueTarget != 60*time.Millisecond || plan.player != 60*time.Millisecond || plan.pump != 4*time.Millisecond {
			t.Fatalf("invalid properties %+v changed fallback: %+v", p, plan)
		}
	}
}

func TestHostAudioOutputUsesBudgetAndKeepsPlayerLag(t *testing.T) {
	previous := currentHostAudioProperties()
	SetHostAudioProperties(48_000, 192)
	t.Cleanup(func() { SetHostAudioProperties(previous.SampleRate, previous.FramesPerBuffer) })
	output := newAudioOutputWithFactory(AudioSettings{Latency: 60 * time.Millisecond}, nil)
	player := &bufferedTestPlayer{}
	output.player = player
	output.configure(output.settings)
	if player.latency != 32*time.Millisecond || output.prebufferWait != 32*time.Millisecond {
		t.Fatalf("player %s queue %s, want 32ms each", player.latency, output.prebufferWait)
	}
	// PCM 40ms behind the displayed frame still falls inside the combined
	// 64ms budget. Using just the 32ms queue target incorrectly discarded it.
	chunk := AudioChunk{SampleRate: 44_100, Channels: 1, Generation: 1,
		StartGuestNS: int64(60 * time.Millisecond), PCM16: make([]int16, 441)}
	if !output.synchronizeChunk(&chunk, int64(100*time.Millisecond), 1) || len(chunk.PCM16) != 441 {
		t.Fatalf("valid player-buffered audio was trimmed: %d samples", len(chunk.PCM16))
	}
	telemetry := output.telemetry()
	if telemetry.ReportedDeviceRate != 48_000 || telemetry.ReportedBurstFrames != 192 || telemetry.PlayerBufferMS != 32 {
		t.Fatalf("missing host buffer telemetry: %+v", telemetry)
	}
}
