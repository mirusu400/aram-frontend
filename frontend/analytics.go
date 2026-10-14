package frontend

import (
	"path"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
)

const (
	analyticsHost = "https://eu.i.posthog.com"
	// Close flushes queued events, but must never hold up app exit indefinitely.
	analyticsShutdownTimeout     = 1500 * time.Millisecond
	analyticsSchemaVersion       = 2
	analyticsPerformanceInterval = 60 * time.Second
	analyticsAudioSampleInterval = time.Second
	// PostHog accepts custom UUIDv7 sessions lasting at most 24 hours.
	analyticsMaxSessionDuration = 23 * time.Hour
)

// analyticsService keeps the transport separate from lifecycle and aggregation.
// Only the fields in these typed events may leave the app.
type analyticsService interface {
	CaptureAppStarted(sessionID string)
	CaptureSessionStarted(sessionID, reason string)
	CaptureTitleMilestone(sessionID string, title analyticsTitle, state FrontendState)
	CaptureSessionEnded(summary analyticsSessionSummary)
	CapturePerformanceSummary(summary analyticsPerformanceSummary)
	Shutdown()
}

type noopAnalyticsService struct{}

func (noopAnalyticsService) CaptureAppStarted(string)                                    {}
func (noopAnalyticsService) CaptureSessionStarted(string, string)                        {}
func (noopAnalyticsService) CaptureTitleMilestone(string, analyticsTitle, FrontendState) {}
func (noopAnalyticsService) CaptureSessionEnded(analyticsSessionSummary)                 {}
func (noopAnalyticsService) CapturePerformanceSummary(analyticsPerformanceSummary)       {}
func (noopAnalyticsService) Shutdown()                                                   {}

type analyticsTitle struct {
	RunID       string
	FileName    string
	SHA256      string
	ImageSHA256 string
	ProfileID   string
	Format      string
	Mode        string
}

func (title analyticsTitle) properties(sessionID string) posthog.Properties {
	properties := posthog.NewProperties().Set("$session_id", sessionID)
	if title.RunID != "" {
		properties.Set("title_run_id", title.RunID).
			Set("file_name", title.FileName).
			Set("sha256", title.SHA256).
			Set("image_sha256", title.ImageSHA256).
			Set("profile_id", title.ProfileID).
			Set("input_format", title.Format).
			Set("mode", title.Mode)
	}
	return properties
}

type analyticsSessionSummary struct {
	SessionID string
	Title     analyticsTitle
	Duration  time.Duration
	State     FrontendState
	HadError  bool
	Reason    string
}

// This is the selected CPU choice, not a claim about the resolved CPU backend.
type analyticsPerformanceSettings struct {
	CPUChoice         string
	RequestedSpeed    float64
	EffectiveSpeed    float64
	DisplaySync       bool
	DisplaySyncActive bool
	VsyncDisabled     bool
	UIPriority        bool
}

func (settings analyticsPerformanceSettings) sameConfiguration(other analyticsPerformanceSettings) bool {
	// A measured refresh rate drifts each second. Update the time-weighted
	// target without making that drift create a new event window.
	settings.EffectiveSpeed = other.EffectiveSpeed
	return settings == other
}

type analyticsPerformanceSummary struct {
	SessionID    string
	Title        analyticsTitle
	Settings     analyticsPerformanceSettings
	Duration     time.Duration
	GuestTime    time.Duration
	TargetTime   time.Duration
	FrameCount   uint64
	FrameWork    time.Duration
	MaxFrameWork time.Duration
	Underruns    uint64
	Overruns     uint64
	Dropped      uint64
	Reason       string
}

func (summary analyticsPerformanceSummary) properties() posthog.Properties {
	properties := summary.Title.properties(summary.SessionID).
		Set("duration_seconds", summary.Duration.Seconds()).
		Set("guest_seconds", summary.GuestTime.Seconds()).
		Set("frame_work_count", summary.FrameCount).
		Set("mean_frame_work_ms", meanFrameWorkMS(summary.FrameWork, summary.FrameCount)).
		Set("max_frame_work_ms", float64(summary.MaxFrameWork)/float64(time.Millisecond)).
		Set("audio_underruns", summary.Underruns).
		Set("audio_overruns", summary.Overruns).
		Set("audio_dropped_frames", summary.Dropped).
		Set("cpu_choice", summary.Settings.CPUChoice).
		Set("requested_speed", summary.Settings.RequestedSpeed).
		Set("effective_speed", summary.Settings.EffectiveSpeed).
		Set("display_sync", summary.Settings.DisplaySync).
		Set("display_sync_active", summary.Settings.DisplaySyncActive).
		Set("vsync_disabled", summary.Settings.VsyncDisabled).
		Set("ui_priority", summary.Settings.UIPriority).
		Set("summary_reason", summary.Reason)
	if summary.Duration > 0 {
		properties.Set("measured_speed", float64(summary.GuestTime)/float64(summary.Duration))
		properties.Set("effective_speed", float64(summary.TargetTime)/float64(summary.Duration))
	}
	if summary.TargetTime > 0 {
		properties.Set("achieved_percent", float64(summary.GuestTime)/float64(summary.TargetTime)*100)
	}
	return properties
}

type postHogAnalyticsService struct {
	client     posthog.Client
	distinctID string
}

func newAnalyticsService(enabled bool, distinctID string) analyticsService {
	if !enabled || AnalyticsAPIKey == "" || distinctID == "" {
		return noopAnalyticsService{}
	}
	client, err := posthog.NewWithConfig(AnalyticsAPIKey, posthog.Config{
		Endpoint:        analyticsHost,
		ShutdownTimeout: analyticsShutdownTimeout,
		IsServer:        posthog.Ptr(false),
		DefaultEventProperties: posthog.NewProperties().
			Set("platform", runtime.GOOS+"/"+runtime.GOARCH).
			Set("app_version", currentApplicationVersion()).
			Set("schema_version", analyticsSchemaVersion),
	})
	if err != nil || client == nil {
		return noopAnalyticsService{}
	}
	return &postHogAnalyticsService{client: client, distinctID: distinctID}
}

func (a *postHogAnalyticsService) capture(event string, properties posthog.Properties) {
	_ = a.client.Enqueue(posthog.Capture{
		DistinctId: a.distinctID,
		Event:      event,
		Properties: properties,
	})
}

func (a *postHogAnalyticsService) CaptureAppStarted(sessionID string) {
	a.capture("app_started", posthog.NewProperties().Set("$session_id", sessionID))
}

func (a *postHogAnalyticsService) CaptureSessionStarted(sessionID, reason string) {
	a.capture("session_started", posthog.NewProperties().
		Set("$session_id", sessionID).Set("start_reason", reason))
}

func (a *postHogAnalyticsService) CaptureTitleMilestone(sessionID string, title analyticsTitle, state FrontendState) {
	a.capture("title_milestone", title.properties(sessionID).Set("milestone", string(state)))
}

func (a *postHogAnalyticsService) CaptureSessionEnded(summary analyticsSessionSummary) {
	reached := string(summary.State)
	if reached == "" {
		reached = "none"
	}
	a.capture("session_ended", summary.Title.properties(summary.SessionID).
		Set("duration_seconds", int(summary.Duration.Seconds())).
		Set("reached_milestone", reached).
		// Keep the old property for existing insights; it denotes guest/load errors.
		Set("crashed", summary.HadError).
		Set("had_error", summary.HadError).
		Set("end_reason", summary.Reason))
}

func (a *postHogAnalyticsService) CapturePerformanceSummary(summary analyticsPerformanceSummary) {
	a.capture("performance_summary", summary.properties())
}

func (a *postHogAnalyticsService) Shutdown() { _ = a.client.Close() }

// Installation identity is random and stable across launches, with no hardware
// or account identifier. PostHog session aggregation separately requires UUIDv7.
func newAnalyticsDistinctID() string { return uuid.NewString() }

func analyticsFileName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	const maxNameRunes = 255
	runes := []rune(name)
	if len(runes) > maxNameRunes {
		return string(runes[:maxNameRunes])
	}
	return name
}

func analyticsSHA256(value string) string {
	if validIssueSHA256(value) {
		return strings.ToLower(value)
	}
	return ""
}

func (s *Shell) toggleAnalyticsEnabled() {
	s.settings.AnalyticsEnabled = !s.settings.AnalyticsEnabled
	_ = s.settings.save()
	s.analytics.setEnabled(s.settings.AnalyticsEnabled && AnalyticsAPIKey != "", s.now(), s.audioQueueTelemetry())
	s.setStatus(s.trf("Usage analytics: %s", s.tr(onOff(s.settings.AnalyticsEnabled))))
}

func (s *Shell) analyticsPerformanceSettings() analyticsPerformanceSettings {
	return analyticsPerformanceSettings{
		CPUChoice:      s.currentCPUSettings().Name,
		RequestedSpeed: s.framePacingSpeed(), EffectiveSpeed: s.effectivePacingSpeed(),
		DisplaySync: s.settings.DisplaySync, DisplaySyncActive: s.displaySync.active,
		VsyncDisabled: s.settings.VsyncDisabled, UIPriority: s.settings.UIPriority,
	}
}

func (s *Shell) reportAnalyticsMilestone(state FrontendState, info InputInfo) {
	if s.analytics.needsMilestone(state, info) {
		s.analytics.milestone(state, info, s.now(), s.audioQueueTelemetry())
	}
}

func (s *Shell) reportAnalyticsPerformance() {
	if s.analytics.needsAudioSample(s.now()) {
		s.analytics.sampleAudio(s.now(), s.audioQueueTelemetry())
	}
}

func (s *Shell) setAnalyticsRunning(running bool) {
	settings := s.analyticsPerformanceSettings()
	if s.analytics.needsRunningChange(running, settings) {
		s.analytics.setRunning(running, settings, s.now(), s.audioQueueTelemetry())
	}
}

func (s *Shell) reportAnalyticsSessionEnded() {
	s.analytics.shutdown(s.now(), s.audioQueueTelemetry())
}
