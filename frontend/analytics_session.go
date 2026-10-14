package frontend

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Native Pause may arrive after Ebitengine has stopped updating. Keep everything
// needed to end a session here, under a mutex, rather than reading UI-owned
// Shell fields from Android's Activity thread. Network flushing runs separately.
type analyticsReporter struct {
	mu           sync.Mutex
	newService   func() analyticsService
	service      analyticsService
	enabled      bool
	active       bool
	closed       bool
	sessionID    string
	startedAt    time.Time
	title        analyticsTitle
	state        FrontendState
	performance  analyticsPerformanceSummary
	runningSince time.Time
	lastAudio    AudioQueueTelemetry
	lastAudioAt  time.Time
}

func newAnalyticsReporter(enabled bool, distinctID string) *analyticsReporter {
	return newAnalyticsReporterWithFactory(enabled && AnalyticsAPIKey != "" && distinctID != "",
		func() analyticsService { return newAnalyticsService(true, distinctID) }, time.Now())
}

func newAnalyticsReporterWithFactory(enabled bool, factory func() analyticsService, now time.Time) *analyticsReporter {
	a := &analyticsReporter{newService: factory, enabled: enabled, active: true}
	a.startSessionLocked(now, "launch")
	return a
}

func (a *analyticsReporter) startSessionLocked(now time.Time, reason string) {
	if !a.enabled || !a.active || a.closed || a.service != nil {
		return
	}
	a.service = a.newService()
	a.sessionID = uuid.Must(uuid.NewV7()).String()
	a.startedAt = now
	a.resetPerformanceLocked()
	if reason == "launch" {
		a.service.CaptureAppStarted(a.sessionID)
	}
	a.service.CaptureSessionStarted(a.sessionID, reason)
	if a.title.RunID != "" {
		a.service.CaptureTitleMilestone(a.sessionID, a.title, a.state)
	}
}

func (a *analyticsReporter) setEnabled(enabled bool, now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.enabled == enabled || a.closed {
		a.mu.Unlock()
		return
	}
	a.enabled = enabled
	old := a.service
	a.service = nil
	a.runningSince = time.Time{}
	a.lastAudio, a.lastAudioAt = audio, now
	a.resetPerformanceLocked()
	a.startSessionLocked(now, "analytics_enabled")
	a.mu.Unlock()
	if old != nil {
		go old.Shutdown()
	}
}

func (a *analyticsReporter) setHostActive(active bool, now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.active == active || a.closed {
		a.mu.Unlock()
		return
	}
	var old analyticsService
	if !active {
		old = a.endSessionLocked(now, audio, "background")
	}
	a.active = active
	// A cumulative audio counter may span foreground sessions. Start at the
	// current count so a new session never inherits the previous one's totals.
	a.lastAudio, a.lastAudioAt = audio, now
	if active {
		a.startSessionLocked(now, "resume")
	}
	a.mu.Unlock()
	if old != nil {
		go old.Shutdown()
	}
}

func (a *analyticsReporter) shutdown(now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	old := a.endSessionLocked(now, audio, "exit")
	a.closed = true
	a.mu.Unlock()
	if old != nil {
		old.Shutdown()
	}
}

func (a *analyticsReporter) endSessionLocked(now time.Time, audio AudioQueueTelemetry, reason string) analyticsService {
	if a.service == nil {
		return nil
	}
	a.addAudioLocked(audio)
	a.flushPerformanceLocked(now, reason)
	hadError := a.state == FrontendGuestFaulted || a.state == FrontendMalformedInput ||
		a.state == FrontendUnsupportedProfile || a.state == FrontendBackendUnavailable
	a.service.CaptureSessionEnded(analyticsSessionSummary{
		SessionID: a.sessionID, Title: a.title,
		Duration: max(time.Duration(0), now.Sub(a.startedAt)),
		State:    a.state, HadError: hadError, Reason: reason,
	})
	old := a.service
	a.service = nil
	a.runningSince = time.Time{}
	return old
}

func (a *analyticsReporter) beginTitle(request OpenRequest, now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addAudioLocked(audio)
	a.flushPerformanceLocked(now, "title_changed")
	name := request.DisplayName
	if name == "" {
		name = request.Path
	}
	mode := "application"
	if request.Firmware {
		mode = "system"
	}
	a.title = analyticsTitle{RunID: uuid.NewString(), FileName: analyticsFileName(name), Mode: mode}
	a.state = FrontendInspecting
	a.runningSince = time.Time{}
	a.lastAudio, a.lastAudioAt = audio, now
	if a.service != nil {
		a.service.CaptureTitleMilestone(a.sessionID, a.title, a.state)
	}
}

func (a *analyticsReporter) clearTitle(now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addAudioLocked(audio)
	a.flushPerformanceLocked(now, "title_closed")
	a.title, a.state = analyticsTitle{}, ""
	a.runningSince = time.Time{}
}

func (a *analyticsReporter) needsMilestone(state FrontendState, info InputInfo) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state != state || a.title.SHA256 != analyticsSHA256(info.SHA256) || a.title.ProfileID != info.ProfileID
}

func (a *analyticsReporter) milestone(state FrontendState, info InputInfo, now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addAudioLocked(audio)
	if a.state == FrontendRunning && state != FrontendRunning {
		a.flushPerformanceLocked(now, string(state))
		a.runningSince = time.Time{}
	}
	a.title.SHA256, a.title.ImageSHA256 = analyticsSHA256(info.SHA256), analyticsSHA256(info.ImageSHA256)
	a.title.ProfileID, a.title.Format = info.ProfileID, info.Format
	if a.title.FileName == "" {
		a.title.FileName = analyticsFileName(info.DisplayName)
	}
	if a.title.RunID == "" {
		a.title.RunID = uuid.NewString()
	}
	a.state = state
	if a.service != nil {
		a.service.CaptureTitleMilestone(a.sessionID, a.title, state)
	}
}

func (a *analyticsReporter) recordFrame(result frameRunResult, now time.Time) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.service == nil || !a.active || a.title.RunID == "" || a.runningSince.IsZero() {
		return
	}
	a.accountRunningTimeLocked(now)
	a.performance.FrameCount += uint64(max(0, result.workFrames))
	a.performance.FrameWork += result.workTotal
	a.performance.MaxFrameWork = max(a.performance.MaxFrameWork, result.workMax)
	a.performance.GuestTime += result.guestAdvanced
}

func (a *analyticsReporter) needsAudioSample(now time.Time) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.service != nil && a.active &&
		now.Sub(a.lastAudioAt) >= analyticsAudioSampleInterval
}

func (a *analyticsReporter) needsRunningChange(running bool, settings analyticsPerformanceSettings) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.service != nil && a.active && a.title.RunID != "" &&
		(running != !a.runningSince.IsZero() || (running && a.performance.Settings != settings))
}

// The scheduler owns this gate: a modal panel can suspend guest execution
// while the backend still reports StateRunning. Exclude that time as well.
func (a *analyticsReporter) setRunning(running bool, settings analyticsPerformanceSettings, now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addAudioLocked(audio)
	if a.service == nil || !a.active || a.title.RunID == "" {
		return
	}
	if running {
		if a.performance.Settings != settings {
			if !a.performance.Settings.sameConfiguration(settings) {
				a.flushPerformanceLocked(now, "settings_changed")
			} else {
				a.accountRunningTimeLocked(now)
			}
			a.performance.Settings = settings
		}
		if a.runningSince.IsZero() {
			a.runningSince = now
		}
	} else if !a.runningSince.IsZero() {
		a.flushPerformanceLocked(now, "paused")
		a.runningSince = time.Time{}
	}
}

func (a *analyticsReporter) sampleAudio(now time.Time, audio AudioQueueTelemetry) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.addAudioLocked(audio)
	a.lastAudioAt = now
	a.accountRunningTimeLocked(now)
	var old analyticsService
	if a.service != nil && now.Sub(a.startedAt) >= analyticsMaxSessionDuration {
		running := !a.runningSince.IsZero()
		old = a.endSessionLocked(now, audio, "rotation")
		a.startSessionLocked(now, "rotation")
		if running {
			a.runningSince = now
		}
	} else if a.performance.Duration >= analyticsPerformanceInterval {
		a.flushPerformanceLocked(now, "interval")
	}
	a.mu.Unlock()
	if old != nil {
		go old.Shutdown()
	}
}

func (a *analyticsReporter) addAudioLocked(audio AudioQueueTelemetry) {
	if a.service != nil && a.active && !a.runningSince.IsZero() {
		a.performance.Underruns += analyticsCounterDelta(audio.Underruns, a.lastAudio.Underruns)
		a.performance.Overruns += analyticsCounterDelta(audio.Overruns, a.lastAudio.Overruns)
		a.performance.Dropped += analyticsCounterDelta(audio.DroppedFrames, a.lastAudio.DroppedFrames)
	}
	a.lastAudio = audio
}

func analyticsCounterDelta(current, previous uint64) uint64 {
	if current < previous {
		return current
	} // A recreated output starts at zero.
	return current - previous
}

func (a *analyticsReporter) accountRunningTimeLocked(now time.Time) {
	if a.runningSince.IsZero() || a.service == nil {
		return
	}
	elapsed := max(time.Duration(0), now.Sub(a.runningSince))
	a.performance.Duration += elapsed
	speed := a.performance.Settings.EffectiveSpeed
	if speed <= 0 {
		speed = 1
	}
	a.performance.TargetTime += time.Duration(float64(elapsed) * speed)
	a.runningSince = now
}

func (a *analyticsReporter) resetPerformanceLocked() {
	a.performance = analyticsPerformanceSummary{Settings: a.performance.Settings}
}

func (a *analyticsReporter) flushPerformanceLocked(now time.Time, reason string) {
	a.accountRunningTimeLocked(now)
	// Idle sessions and repeated lifecycle callbacks must not create empty rows.
	if a.service != nil && a.performance.FrameCount > 0 {
		a.performance.SessionID, a.performance.Title = a.sessionID, a.title
		a.performance.Reason = reason
		a.service.CapturePerformanceSummary(a.performance)
	}
	a.resetPerformanceLocked()
}
