package frontend

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
)

// Exercise the actual event property mapping without sending test data to the
// live project. Close can be blocked to simulate a slow/offline network.
type analyticsTestClient struct {
	posthog.Client
	mu           sync.Mutex
	events       []posthog.Capture
	closeCalls   atomic.Int32
	closeStarted chan struct{}
	closeDone    chan struct{}
	closeRelease <-chan struct{}
}

func newAnalyticsTestClient() *analyticsTestClient {
	return &analyticsTestClient{closeStarted: make(chan struct{}), closeDone: make(chan struct{})}
}

func (c *analyticsTestClient) Enqueue(message posthog.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, message.(posthog.Capture))
	return nil
}

func (c *analyticsTestClient) Close() error {
	if c.closeCalls.Add(1) != 1 {
		return errors.New("closed twice")
	}
	close(c.closeStarted)
	if c.closeRelease != nil {
		<-c.closeRelease
	}
	close(c.closeDone)
	return nil
}

func (c *analyticsTestClient) captured(event string) []posthog.Capture {
	c.mu.Lock()
	defer c.mu.Unlock()
	var events []posthog.Capture
	for _, capture := range c.events {
		if capture.Event == event {
			events = append(events, capture)
		}
	}
	return events
}

func newTestAnalytics(enabled bool, now time.Time) (*analyticsReporter, *[]*analyticsTestClient) {
	clients := []*analyticsTestClient{}
	a := newAnalyticsReporterWithFactory(enabled, func() analyticsService {
		client := newAnalyticsTestClient()
		clients = append(clients, client)
		return &postHogAnalyticsService{client: client, distinctID: "synthetic-install"}
	}, now)
	return a, &clients
}

func assertAnalyticsProperties(t *testing.T, capture posthog.Capture, want posthog.Properties) {
	t.Helper()
	for key, value := range want {
		if got := capture.Properties[key]; got != value {
			t.Errorf("%s.%s = %v (%T), want %v (%T)", capture.Event, key, got, got, value, value)
		}
	}
}

type analyticsOpenBackend struct {
	NullBackend
	err error
}

func (b *analyticsOpenBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	if b.err != nil {
		return InputInfo{}, b.err
	}
	return InputInfo{
		DisplayName: "Embedded guest title", SHA256: strings.Repeat("A", 64),
		ImageSHA256: strings.Repeat("b", 64), ProfileID: "synthetic-profile", Format: "synthetic",
	}, nil
}

func (*analyticsOpenBackend) State() BackendState { return StateReady }

func TestAnalyticsOpenKeepsOriginalFilenameAndSeparatesTitleRuns(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	a, clients := newTestAnalytics(true, time.Now())
	t.Cleanup(func() { a.shutdown(time.Now(), AudioQueueTelemetry{}) })
	backend := &analyticsOpenBackend{}
	shell := &Shell{analytics: a, backend: backend, settings: defaultSettings(), backendResults: make(chan backendResult, 1)}
	var runID string
	for i, request := range []OpenRequest{
		{Path: `C:\private\games\한글게임.dat`},
		{Path: "/private/cache/drop-123.dat", DisplayName: "Original game.dat"},
		{Path: "content://documents/secret-document-id", DisplayName: "Mobile game.dat"},
	} {
		shell.openRequest(request)
		select {
		case result := <-shell.backendResults:
			shell.consumeBackendResult(result)
		case <-time.After(5 * time.Second):
			t.Fatal("open did not complete")
		}
		for range 5 {
			shell.reportAnalyticsMilestone(FrontendReady, *shell.input)
		}
		events := (*clients)[0].captured("title_milestone")
		if len(events) != 2*(i+1) {
			t.Fatalf("duplicate or missing milestones: %d", len(events))
		}
		ready := events[len(events)-1]
		name := []string{"한글게임.dat", "Original game.dat", "Mobile game.dat"}[i]
		assertAnalyticsProperties(t, ready, posthog.Properties{
			"file_name": name, "sha256": strings.Repeat("a", 64), "image_sha256": strings.Repeat("b", 64),
			"profile_id": "synthetic-profile", "milestone": "ready", "mode": "application",
		})
		if ready.Properties["title_run_id"] == runID {
			t.Fatal("different opens shared a title run")
		}
		runID = ready.Properties["title_run_id"].(string)
		for key, value := range ready.Properties {
			if text, ok := value.(string); ok && (strings.Contains(text, "private") || strings.Contains(text, "secret-document-id")) {
				t.Fatalf("private path leaked through %s: %s", key, text)
			}
		}
	}
	backend.err = errors.New("synthetic malformed input")
	shell.openRequest(OpenRequest{DisplayName: "broken.dat", Data: []byte("synthetic")})
	select {
	case result := <-shell.backendResults:
		shell.consumeBackendResult(result)
	case <-time.After(5 * time.Second):
		t.Fatal("failed open did not complete")
	}
	events := (*clients)[0].captured("title_milestone")
	assertAnalyticsProperties(t, events[len(events)-1], posthog.Properties{"file_name": "broken.dat", "sha256": "", "milestone": "guest-faulted"})
}

func TestAnalyticsPerformanceIntervalAndAndroidForegroundSessions(t *testing.T) {
	now := time.Unix(1000, 0)
	a, clients := newTestAnalytics(true, now)
	settings := analyticsPerformanceSettings{CPUChoice: "fastest", RequestedSpeed: 2, EffectiveSpeed: 2}
	info := InputInfo{SHA256: strings.Repeat("a", 64), ProfileID: "synthetic"}
	a.beginTitle(OpenRequest{DisplayName: "Synthetic.dat"}, now, AudioQueueTelemetry{Underruns: 10, DroppedFrames: 100})
	a.milestone(FrontendRunning, info, now, AudioQueueTelemetry{Underruns: 10, DroppedFrames: 100})
	a.setRunning(true, settings, now, AudioQueueTelemetry{Underruns: 10, DroppedFrames: 100})
	a.recordFrame(frameRunResult{workFrames: 2, workTotal: 12 * time.Millisecond, workMax: 8 * time.Millisecond, guestAdvanced: 45 * time.Second}, now.Add(30*time.Second))
	a.sampleAudio(now.Add(30*time.Second), AudioQueueTelemetry{Underruns: 12, DroppedFrames: 102})
	first := (*clients)[0]
	if len(first.captured("performance_summary")) != 0 {
		t.Fatal("sent a summary before the interval")
	}
	a.recordFrame(frameRunResult{workFrames: 3, workTotal: 9 * time.Millisecond, workMax: 4 * time.Millisecond, guestAdvanced: 45 * time.Second}, now.Add(60*time.Second))
	a.sampleAudio(now.Add(60*time.Second), AudioQueueTelemetry{Underruns: 14, DroppedFrames: 105})
	performance := first.captured("performance_summary")
	if len(performance) != 1 {
		t.Fatalf("interval summaries = %d", len(performance))
	}
	assertAnalyticsProperties(t, performance[0], posthog.Properties{
		"file_name": "Synthetic.dat", "duration_seconds": float64(60), "measured_speed": 1.5,
		"achieved_percent": float64(75), "frame_work_count": uint64(5), "mean_frame_work_ms": 4.2,
		"max_frame_work_ms": float64(8), "audio_underruns": uint64(4), "audio_dropped_frames": uint64(5), "summary_reason": "interval",
	})
	a.recordFrame(frameRunResult{workFrames: 1, workTotal: 2 * time.Millisecond, workMax: 2 * time.Millisecond, guestAdvanced: 10 * time.Second}, now.Add(65*time.Second))
	a.setHostActive(false, now.Add(65*time.Second), AudioQueueTelemetry{Underruns: 15, DroppedFrames: 105})
	a.setHostActive(false, now.Add(66*time.Second), AudioQueueTelemetry{Underruns: 15, DroppedFrames: 105})
	performance = first.captured("performance_summary")
	if len(performance) != 2 {
		t.Fatalf("final tail missing or duplicated: %d", len(performance))
	}
	assertAnalyticsProperties(t, performance[1], posthog.Properties{
		"duration_seconds": float64(5), "frame_work_count": uint64(1), "max_frame_work_ms": float64(2),
		"audio_underruns": uint64(1), "achieved_percent": float64(100), "summary_reason": "background",
	})
	ended := first.captured("session_ended")
	if len(ended) != 1 {
		t.Fatalf("duplicate session end: %d", len(ended))
	}
	assertAnalyticsProperties(t, ended[0], posthog.Properties{"duration_seconds": 65, "end_reason": "background", "file_name": "Synthetic.dat"})
	firstID := ended[0].Properties["$session_id"].(string)
	if id, err := uuid.Parse(firstID); err != nil || id.Version() != 7 {
		t.Fatalf("invalid PostHog session ID %s", firstID)
	}
	if performance[0].Properties["$session_id"] != firstID {
		t.Fatal("performance and session were not linked")
	}
	a.setHostActive(true, now.Add(100*time.Second), AudioQueueTelemetry{Underruns: 99})
	a.setRunning(true, settings, now.Add(100*time.Second), AudioQueueTelemetry{Underruns: 99})
	a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: 8 * time.Second}, now.Add(104*time.Second))
	a.shutdown(now.Add(104*time.Second), AudioQueueTelemetry{Underruns: 100})
	a.shutdown(now.Add(105*time.Second), AudioQueueTelemetry{Underruns: 100})
	second := (*clients)[1]
	secondEnd := second.captured("session_ended")
	if len(secondEnd) != 1 {
		t.Fatalf("second session ends = %d", len(secondEnd))
	}
	assertAnalyticsProperties(t, secondEnd[0], posthog.Properties{"duration_seconds": 4, "end_reason": "exit"})
	if secondEnd[0].Properties["$session_id"] == firstID {
		t.Fatal("resume reused the ended session")
	}
	if secondEnd[0].Properties["title_run_id"] != ended[0].Properties["title_run_id"] {
		t.Fatal("resume lost the continuing title run")
	}
	if len(second.captured("app_started")) != 0 || len(second.captured("session_started")) != 1 {
		t.Fatal("resume was counted as a new process launch")
	}
	assertAnalyticsProperties(t, second.captured("performance_summary")[0], posthog.Properties{"audio_underruns": uint64(1), "duration_seconds": float64(4)})
	select {
	case <-first.closeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("background did not flush the client")
	}
}

func TestAnalyticsModalPauseAndSettingsExcludeIdleTime(t *testing.T) {
	now := time.Unix(1000, 0)
	a, clients := newTestAnalytics(true, now)
	a.beginTitle(OpenRequest{DisplayName: "Synthetic.dat"}, now, AudioQueueTelemetry{})
	a.milestone(FrontendRunning, InputInfo{}, now, AudioQueueTelemetry{})
	settings := analyticsPerformanceSettings{CPUChoice: "fastest", RequestedSpeed: 1, EffectiveSpeed: 1}
	a.setRunning(true, settings, now, AudioQueueTelemetry{})
	a.recordFrame(frameRunResult{workFrames: 2, guestAdvanced: 2 * time.Second, workMax: 10 * time.Millisecond}, now.Add(2*time.Second))
	// A modal panel gates execution without changing the backend's running state.
	a.setRunning(false, settings, now.Add(2*time.Second), AudioQueueTelemetry{Underruns: 1})
	a.sampleAudio(now.Add(100*time.Second), AudioQueueTelemetry{Underruns: 90})
	a.setRunning(true, settings, now.Add(100*time.Second), AudioQueueTelemetry{Underruns: 90})
	a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: 2 * time.Second, workMax: time.Millisecond}, now.Add(102*time.Second))
	settings.RequestedSpeed, settings.EffectiveSpeed = 2, 2
	a.setRunning(true, settings, now.Add(102*time.Second), AudioQueueTelemetry{Underruns: 91})
	a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: 4 * time.Second, workMax: 2 * time.Millisecond}, now.Add(104*time.Second))
	a.beginTitle(OpenRequest{DisplayName: "Second.dat"}, now.Add(104*time.Second), AudioQueueTelemetry{Underruns: 92})
	a.shutdown(now.Add(110*time.Second), AudioQueueTelemetry{Underruns: 92})
	events := (*clients)[0].captured("performance_summary")
	if len(events) != 3 {
		t.Fatalf("summaries = %d", len(events))
	}
	for i, event := range events {
		assertAnalyticsProperties(t, event, posthog.Properties{"file_name": "Synthetic.dat", "duration_seconds": float64(2), "achieved_percent": float64(100), "audio_underruns": uint64(1)})
		if i == 2 {
			assertAnalyticsProperties(t, event, posthog.Properties{"effective_speed": float64(2), "summary_reason": "title_changed", "max_frame_work_ms": float64(2)})
		}
	}
}

func TestAnalyticsNativePauseDoesNotNeedUpdateOrWaitForNetwork(t *testing.T) {
	a, clients := newTestAnalytics(true, time.Now().Add(-time.Second))
	client := (*clients)[0]
	release := make(chan struct{})
	client.closeRelease = release
	t.Cleanup(func() { close(release) })
	shell := &Shell{analytics: a}
	shell.hostActiveRequest.Store(true)
	returned := make(chan struct{})
	go func() { shell.SetHostActive(false); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("native pause waited for network")
	}
	select {
	case <-client.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("native pause did not start flushing")
	}
	if len(client.captured("session_ended")) != 1 {
		t.Fatal("native pause needed the suspended game loop to end its session")
	}
	shell.SetHostActive(false)
	if len(client.captured("session_ended")) != 1 {
		t.Fatal("duplicate pause ended the session twice")
	}
	a.shutdown(time.Now(), AudioQueueTelemetry{})
}

func TestAnalyticsRefreshRateDriftKeepsMinuteSummary(t *testing.T) {
	now := time.Unix(1000, 0)
	a, clients := newTestAnalytics(true, now)
	a.beginTitle(OpenRequest{DisplayName: "Synthetic.dat"}, now, AudioQueueTelemetry{})
	a.milestone(FrontendRunning, InputInfo{}, now, AudioQueueTelemetry{})
	settings := analyticsPerformanceSettings{RequestedSpeed: 1, EffectiveSpeed: 0.96, DisplaySync: true, DisplaySyncActive: true}
	for second := range 60 {
		settings.EffectiveSpeed = 0.96 + float64(second)*0.00001
		a.setRunning(true, settings, now.Add(time.Duration(second)*time.Second), AudioQueueTelemetry{})
		a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: time.Second}, now.Add(time.Duration(second+1)*time.Second))
		a.sampleAudio(now.Add(time.Duration(second+1)*time.Second), AudioQueueTelemetry{})
	}
	a.shutdown(now.Add(time.Minute), AudioQueueTelemetry{})
	events := (*clients)[0].captured("performance_summary")
	if len(events) != 1 {
		t.Fatalf("refresh-rate drift flooded events: %d", len(events))
	}
	assertAnalyticsProperties(t, events[0], posthog.Properties{"summary_reason": "interval", "frame_work_count": uint64(60), "duration_seconds": float64(60)})
	if speed := events[0].Properties["effective_speed"].(float64); speed < 0.96 || speed > 0.961 {
		t.Fatalf("invalid averaged display-sync target: %f", speed)
	}
}

func TestAnalyticsOptOutDiscardsDisabledPerformance(t *testing.T) {
	now := time.Unix(1000, 0)
	a, clients := newTestAnalytics(false, now)
	a.beginTitle(OpenRequest{DisplayName: "Synthetic.dat"}, now, AudioQueueTelemetry{})
	a.milestone(FrontendRunning, InputInfo{}, now, AudioQueueTelemetry{})
	settings := analyticsPerformanceSettings{RequestedSpeed: 1, EffectiveSpeed: 1}
	a.setRunning(true, settings, now, AudioQueueTelemetry{})
	a.recordFrame(frameRunResult{workFrames: 100}, now.Add(time.Second))
	if len(*clients) != 0 {
		t.Fatal("disabled analytics constructed a client")
	}
	a.setEnabled(true, now.Add(2*time.Second), AudioQueueTelemetry{Underruns: 100})
	a.setRunning(true, settings, now.Add(2*time.Second), AudioQueueTelemetry{Underruns: 100})
	a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: time.Second}, now.Add(3*time.Second))
	a.setEnabled(false, now.Add(3*time.Second), AudioQueueTelemetry{Underruns: 101})
	a.recordFrame(frameRunResult{workFrames: 100}, now.Add(4*time.Second))
	a.setEnabled(true, now.Add(5*time.Second), AudioQueueTelemetry{Underruns: 200})
	a.setRunning(true, settings, now.Add(5*time.Second), AudioQueueTelemetry{Underruns: 200})
	a.recordFrame(frameRunResult{workFrames: 2, guestAdvanced: time.Second}, now.Add(6*time.Second))
	a.shutdown(now.Add(6*time.Second), AudioQueueTelemetry{Underruns: 201})
	if len((*clients)[0].captured("performance_summary")) != 0 {
		t.Fatal("opt-out sent a pending summary")
	}
	assertAnalyticsProperties(t, (*clients)[1].captured("performance_summary")[0], posthog.Properties{"frame_work_count": uint64(2), "audio_underruns": uint64(1), "duration_seconds": float64(1)})
}

func TestAnalyticsLongForegroundSessionRotates(t *testing.T) {
	now := time.Unix(1000, 0)
	a, clients := newTestAnalytics(true, now)
	later := now.Add(analyticsMaxSessionDuration)
	if !a.needsAudioSample(later) {
		t.Fatal("idle foreground session could not rotate")
	}
	a.sampleAudio(later, AudioQueueTelemetry{})
	if len(*clients) != 2 {
		t.Fatal("long foreground session did not rotate")
	}
	first := (*clients)[0].captured("session_ended")
	second := (*clients)[1].captured("session_started")
	assertAnalyticsProperties(t, first[0], posthog.Properties{"end_reason": "rotation"})
	assertAnalyticsProperties(t, second[0], posthog.Properties{"start_reason": "rotation"})
	if first[0].Properties["$session_id"] == second[0].Properties["$session_id"] {
		t.Fatal("rotation reused a session ID")
	}
	a.shutdown(later, AudioQueueTelemetry{})
}

func TestAnalyticsConcurrentFrameAndLifecycleCallbacks(t *testing.T) {
	a, _ := newTestAnalytics(true, time.Now())
	a.beginTitle(OpenRequest{DisplayName: "Synthetic.dat"}, time.Now(), AudioQueueTelemetry{})
	a.milestone(FrontendRunning, InputInfo{}, time.Now(), AudioQueueTelemetry{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 100 {
			a.setRunning(true, analyticsPerformanceSettings{EffectiveSpeed: 1}, time.Now(), AudioQueueTelemetry{})
			a.recordFrame(frameRunResult{workFrames: 1, guestAdvanced: time.Millisecond}, time.Now())
			a.sampleAudio(time.Now(), AudioQueueTelemetry{})
		}
	}()
	go func() {
		defer workers.Done()
		for range 20 {
			a.setHostActive(false, time.Now(), AudioQueueTelemetry{})
			a.setHostActive(true, time.Now(), AudioQueueTelemetry{})
		}
	}()
	workers.Wait()
	a.shutdown(time.Now(), AudioQueueTelemetry{})
}
