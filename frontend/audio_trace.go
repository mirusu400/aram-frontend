package frontend

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// audioTraceCapacity bounds the ring so a long session cannot grow the trace
// without limit. The oldest event is dropped once it is full and the omitted
// count keeps the record honest.
const audioTraceCapacity = 512

// audioTraceEntry is one moment in the audio pipeline. Every entry snapshots the
// cumulative queue counters so a reader sees underruns and drops climbing
// between control events even when no control event of its own fired.
type audioTraceEntry struct {
	millis    int64
	kind      string
	detail    string
	underruns uint64
	overruns  uint64
	dropped   uint64
	stale     uint64
	fill      int
}

// audioTrace is a bounded, timestamped history of audio pipeline events. Sound
// problems are timing and synchronization problems, so a point-in-time counter
// cannot explain them; this ring is where the sequence lives.
type audioTrace struct {
	mu      sync.Mutex
	start   time.Time
	config  string
	entries []audioTraceEntry
	omitted int
}

func newAudioTrace(config string) *audioTrace {
	return &audioTrace{start: time.Now(), config: config}
}

func (t *audioTrace) record(
	kind, detail string,
	telemetry AudioQueueTelemetry,
) {
	if t == nil {
		return
	}
	entry := audioTraceEntry{
		millis:    time.Since(t.start).Milliseconds(),
		kind:      kind,
		detail:    detail,
		underruns: telemetry.Underruns,
		overruns:  telemetry.Overruns,
		dropped:   telemetry.DroppedFrames,
		stale:     telemetry.StaleFrames,
		fill:      telemetry.FillFrames,
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.entries) >= audioTraceCapacity {
		copy(t.entries, t.entries[1:])
		t.entries[len(t.entries)-1] = entry
		t.omitted++
		return
	}
	t.entries = append(t.entries, entry)
}

// render returns the human-readable trace for the debug bundle. It carries no
// host paths, so it needs no redaction.
func (t *audioTrace) render() []byte {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var output strings.Builder
	fmt.Fprintf(
		&output,
		"[audio-trace] events=%d omitted=%d\n%s\n",
		len(t.entries),
		t.omitted,
		t.config,
	)
	for _, entry := range t.entries {
		fmt.Fprintf(
			&output,
			"t+%d.%03ds %-6s under=%d over=%d drop=%d stale=%d fill=%d %s\n",
			entry.millis/1000,
			entry.millis%1000,
			entry.kind,
			entry.underruns,
			entry.overruns,
			entry.dropped,
			entry.stale,
			entry.fill,
			strings.TrimSpace(entry.detail),
		)
	}
	return []byte(output.String())
}
