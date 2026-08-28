package frontend

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWriteDebugBundleIncludesOptionalArtifacts(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	snapshot := debugBundleSnapshot{
		CreatedAt:   time.Now().UTC(),
		AudioTrace:  []byte("[audio-trace] events=1 omitted=0\nt+0.000s start\n"),
		CPUProfile:  []byte{0x1f, 0x8b, 0x08, 0x00},
		CrashReport: []byte("panic: boom\n\ngoroutine 1 [running]:\n"),
	}
	path, _, err := writeDebugBundle(snapshot, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := readDebugZIP(t, path)
	for _, name := range []string{
		"goroutines.txt",
		"audio-trace.log",
		"cpu.pprof",
		"crash.txt",
	} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("bundle is missing %s (entries %v)", name, sortedMapKeys(entries))
		}
	}
	if !strings.Contains(string(entries["audio-trace.log"]), "audio-trace") {
		t.Fatalf("audio-trace.log = %q", entries["audio-trace.log"])
	}
	if !strings.Contains(string(entries["crash.txt"]), "panic: boom") {
		t.Fatalf("crash.txt = %q", entries["crash.txt"])
	}
}

func TestAudioTraceRecordsRingAndRenders(t *testing.T) {
	trace := newAudioTrace("host_rate=44100Hz")
	telemetry := AudioQueueTelemetry{
		Underruns:     3,
		Overruns:      1,
		DroppedFrames: 2,
		StaleFrames:   4,
		FillFrames:    100,
	}
	trace.record("start", "", telemetry)
	trace.record("flush", "generation 1->2", telemetry)

	rendered := string(trace.render())
	if !strings.Contains(rendered, "host_rate=44100Hz") ||
		!strings.Contains(rendered, "start") ||
		!strings.Contains(rendered, "generation 1->2") ||
		!strings.Contains(rendered, "under=3") {
		t.Fatalf("rendered = %q", rendered)
	}

	for i := 0; i < audioTraceCapacity+10; i++ {
		trace.record("sample", "", telemetry)
	}
	if len(trace.entries) != audioTraceCapacity {
		t.Fatalf("entries = %d, want %d", len(trace.entries), audioTraceCapacity)
	}
	if trace.omitted < 10 {
		t.Fatalf("omitted = %d, want >= 10", trace.omitted)
	}
}

type stubCrashPrompter struct {
	confirm   bool
	available bool
}

func (s stubCrashPrompter) confirmCrashReport(string) (bool, bool) {
	return s.confirm, s.available
}

type stubIssueRelay struct {
	submission issueRelaySubmission
	report     issueRelayReport
	err        error
	calls      int
}

func (s *stubIssueRelay) Submit(
	_ context.Context,
	submission issueRelaySubmission,
) (issueRelayReport, error) {
	s.calls++
	s.submission = submission
	return s.report, s.err
}

func (s *stubIssueRelay) AddComment(
	context.Context,
	issueRelayReport,
	string,
	string,
) (string, error) {
	return "", nil
}

func TestHandleShellCrashSubmitsAndExits(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	shell := &Shell{backend: NullBackend{}, settings: defaultSettings()}
	relay := &stubIssueRelay{report: issueRelayReport{
		ReportID:   "11111111-1111-4111-8111-111111111111",
		IssueURL:   "https://github.com/mirusu400/aram-frontend/issues/7",
		Capability: "aram_rpt_" + strings.Repeat("z", 43),
	}}
	opened := ""
	exitCode := -1
	env := crashEnvironment{
		prompter: stubCrashPrompter{confirm: true, available: true},
		relay:    relay,
		openURL:  func(url string) error { opened = url; return nil },
		exit:     func(code int) { exitCode = code },
	}

	handleShellCrash(shell, env, "boom", []byte("goroutine 1 [running]:\nmain.crash()"))

	if relay.calls != 1 {
		t.Fatalf("relay calls = %d, want 1", relay.calls)
	}
	if !strings.Contains(relay.submission.Draft.Situation, "Automatic crash report") ||
		!strings.Contains(relay.submission.Draft.Situation, "boom") {
		t.Fatalf("crash draft = %+v", relay.submission.Draft)
	}
	if relay.submission.Draft.Repository != "aram-frontend" {
		t.Fatalf("crash repository = %q", relay.submission.Draft.Repository)
	}
	if relay.submission.BundlePath == "" {
		t.Fatal("crash submission carries no bundle path")
	}
	if opened != "https://github.com/mirusu400/aram-frontend/issues/7" {
		t.Fatalf("opened URL = %q", opened)
	}
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}

	entries := readDebugZIP(t, relay.submission.BundlePath)
	if !strings.Contains(string(entries["crash.txt"]), "panic: boom") ||
		!strings.Contains(string(entries["crash.txt"]), "goroutine 1") {
		t.Fatalf("crash.txt = %q", entries["crash.txt"])
	}
	if _, ok := entries["screenshot.png"]; ok {
		t.Fatal("crash bundle should not carry a screenshot")
	}
}

func TestHandleShellCrashDeclinedDoesNotSubmit(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	shell := &Shell{backend: NullBackend{}, settings: defaultSettings()}
	relay := &stubIssueRelay{}
	exitCode := -1
	env := crashEnvironment{
		prompter: stubCrashPrompter{confirm: false, available: true},
		relay:    relay,
		openURL:  func(string) error { return nil },
		exit:     func(code int) { exitCode = code },
	}

	handleShellCrash(shell, env, "nope", []byte("stack"))

	if relay.calls != 0 {
		t.Fatalf("declined crash still submitted (%d calls)", relay.calls)
	}
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
}

func TestSnapshotCPUProfileRoundTrips(t *testing.T) {
	shell := &Shell{backend: NullBackend{}, settings: defaultSettings()}
	if got := shell.snapshotCPUProfile(); got != nil {
		t.Fatalf("profile before enabling = %v", got)
	}
	if !shell.setCPUProfiling(true) {
		t.Skip("CPU profiling is unavailable on this platform")
	}
	busy := 0
	for i := 0; i < 1_000_000; i++ {
		busy += i
	}
	_ = busy
	data := shell.snapshotCPUProfile()
	shell.setCPUProfiling(false)
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatalf("cpu profile is not a gzip pprof (len=%d)", len(data))
	}
}
