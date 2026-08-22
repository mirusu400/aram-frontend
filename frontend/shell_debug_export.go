package frontend

import (
	"image"
	"time"
)

func (s *Shell) saveDebugBundle() {
	s.setStatus(s.tr("Collecting debug bundle..."))
	snapshot := s.captureDebugBundleSnapshot(time.Now().UTC())
	backend := s.backend
	go func() {
		path, warning, err := collectDebugBundle(snapshot, backend)
		s.artifactResults <- artifactResult{
			kind:    "Debug bundle",
			path:    path,
			warning: warning,
			err:     err,
		}
	}()
}

func (s *Shell) captureDebugBundleSnapshot(createdAt time.Time) debugBundleSnapshot {
	var input *InputInfo
	if s.input != nil {
		value := *s.input
		input = &value
	}
	var problem *FrontendProblem
	if s.problem != nil {
		value := *s.problem
		problem = &value
	}

	redactions := debugRedactionRoots(s.selectedPath)
	logs := append([]string(nil), s.logs...)
	for index := range logs {
		logs[index] = redactDebugText(logs[index], redactions)
	}
	if problem != nil {
		problem.Reason = redactDebugText(problem.Reason, redactions)
	}
	build := currentDebugBuildReport()
	redactDebugBuildReport(&build, redactions)
	var screenshot *image.RGBA
	if frame := s.currentFrame(); frame.Image != nil {
		screenshot = cloneImage(frame.Image)
	}

	return debugBundleSnapshot{
		CreatedAt:     createdAt,
		Input:         input,
		Backend:       s.backendName(),
		BackendState:  s.backend.State(),
		FrontendState: s.state,
		Problem:       problem,
		Settings: debugSettingsReport{
			Language:       s.settings.Language,
			CPU:            s.settings.CPUChoice,
			Speed:          s.settings.Speed,
			StateSlot:      s.settings.StateSlot,
			Theme:          s.settings.ThemeMode,
			IntegerScaling: s.settings.IntegerScaling,
			PreserveAspect: s.settings.PreserveAspect,
			Rotation:       s.settings.Rotation,
			ScreenLayout:   s.settings.ScreenLayout,
			Filter:         s.settings.Filter,
			Muted:          s.settings.Muted,
			Volume:         s.settings.Volume,
			AudioLatencyMS: s.settings.AudioLatencyMS,
			AudioMixMode:   s.settings.AudioMixMode,
		},
		Build:        build,
		FrontendLogs: logs,
		Redactions:   redactions,
		Screenshot:   screenshot,
	}
}
