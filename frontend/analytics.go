package frontend

import (
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
)

const (
	analyticsHost = "https://eu.i.posthog.com"
	// analyticsShutdownTimeout bounds how long app exit waits for queued
	// events to flush, so an offline or slow connection never delays closing
	// the app.
	analyticsShutdownTimeout = 1500 * time.Millisecond
	analyticsSchemaVersion   = 1
)

// analyticsService is the app's usage-analytics seam, deliberately narrow: a
// handful of lifecycle events, not a generic property bag. A disabled or
// unconfigured client is a no-op so callers never branch on whether
// analytics is on.
type analyticsService interface {
	CaptureAppStarted()
	CaptureTitleMilestone(sha256, profileID, milestone string)
	CaptureSessionEnded(duration time.Duration, reachedMilestone string, crashed bool)
	Shutdown()
}

type noopAnalyticsService struct{}

func (noopAnalyticsService) CaptureAppStarted()                                        {}
func (noopAnalyticsService) CaptureTitleMilestone(sha256, profileID, milestone string) {}
func (noopAnalyticsService) CaptureSessionEnded(time.Duration, string, bool)           {}
func (noopAnalyticsService) Shutdown()                                                 {}

type postHogAnalyticsService struct {
	client     posthog.Client
	distinctID string
}

// newAnalyticsService returns a no-op service when analytics is disabled in
// settings or the API key hasn't been configured yet, so a disabled user's
// process never opens a PostHog connection at all.
func newAnalyticsService(enabled bool, distinctID string) analyticsService {
	if !enabled || AnalyticsAPIKey == "" || distinctID == "" {
		return noopAnalyticsService{}
	}
	client, err := posthog.NewWithConfig(AnalyticsAPIKey, posthog.Config{
		Endpoint:        analyticsHost,
		ShutdownTimeout: analyticsShutdownTimeout,
		// This is a client app, not a server - omit $is_server so events
		// attribute to the running OS/arch normally.
		IsServer: posthog.Ptr(false),
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

func (a *postHogAnalyticsService) CaptureAppStarted() {
	a.capture("app_started", nil)
}

func (a *postHogAnalyticsService) CaptureTitleMilestone(sha256, profileID, milestone string) {
	a.capture("title_milestone", posthog.NewProperties().
		Set("sha256", sha256).
		Set("profile_id", profileID).
		Set("milestone", milestone))
}

func (a *postHogAnalyticsService) CaptureSessionEnded(duration time.Duration, reachedMilestone string, crashed bool) {
	a.capture("session_ended", posthog.NewProperties().
		Set("duration_seconds", int(duration.Seconds())).
		Set("reached_milestone", reachedMilestone).
		Set("crashed", crashed))
}

// Shutdown flushes and closes the client, bounded by analyticsShutdownTimeout
// (Config.ShutdownTimeout above) so a dead connection never delays app exit.
func (a *postHogAnalyticsService) Shutdown() {
	_ = a.client.Close()
}

// newAnalyticsDistinctID mints a random anonymous identifier, persisted in
// Settings so it stays stable across runs. It is never derived from any
// hardware or account identifier.
func newAnalyticsDistinctID() string {
	return uuid.NewString()
}

// toggleAnalyticsEnabled flips the persisted analytics preference and
// (re)initializes the client - a disabled user never has one constructed.
func (s *Shell) toggleAnalyticsEnabled() {
	s.settings.AnalyticsEnabled = !s.settings.AnalyticsEnabled
	_ = s.settings.save()
	s.analytics.Shutdown()
	s.analytics = newAnalyticsService(s.settings.AnalyticsEnabled, s.settings.AnalyticsID)
	s.setStatus(s.trf("Usage analytics: %s", s.tr(onOff(s.settings.AnalyticsEnabled))))
}

// reportAnalyticsMilestone captures a title-load milestone, deduped so a
// state that hasn't changed (syncBackendState runs every frame) doesn't
// resend the same event. sha256/profileID come from the caller rather than
// s.input directly, because on a failed open s.input may still hold the
// previous title (Shell only updates it once a DisplayName comes back).
func (s *Shell) reportAnalyticsMilestone(state FrontendState, sha256, profileID string) {
	if state == s.analyticsLastMilestone {
		return
	}
	s.analyticsLastMilestone = state
	if !validIssueSHA256(sha256) {
		return
	}
	s.analytics.CaptureTitleMilestone(strings.ToLower(sha256), profileID, string(state))
}

// reportAnalyticsSessionEnded fires once as the process shuts down. Desktop
// and web reach it through Run/RunWithOptions; there is no equivalent
// teardown hook on Android/iOS today (the mobile host owns the process
// lifecycle through Pause/Resume, not a Run-loop return), so mobile sessions
// don't yet emit this event.
func (s *Shell) reportAnalyticsSessionEnded() {
	reached := "none"
	if s.analyticsLastMilestone != "" {
		reached = string(s.analyticsLastMilestone)
	}
	s.analytics.CaptureSessionEnded(
		time.Since(s.analyticsSessionStartedAt),
		reached,
		s.problem != nil,
	)
	s.analytics.Shutdown()
}
