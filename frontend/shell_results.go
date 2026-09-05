package frontend

import (
	"errors"
	"fmt"
)

func (s *Shell) consumeResults() {
	for {
		select {
		case request := <-s.externalOpen:
			s.openRequest(request)
		case commandID := <-s.externalCommands:
			s.dispatchCommand(commandID)
		case <-s.externalSelectionCanceled:
			s.dialogOpen = false
			s.state = s.preDialogState
			s.setStatus(s.tr("Selection canceled"))
		case stage := <-s.openStageResults:
			switch stage {
			case OpenStageInspecting:
				s.state = FrontendInspecting
				s.setStatus(s.tr("Inspecting selected input..."))
			case OpenStageLoading:
				s.state = FrontendLoading
				s.setStatus(s.tr("Loading selected input..."))
			}
		case entries := <-s.libraryResults:
			s.libraryScanning = false
			s.libraryEntries = entries
		case result := <-s.iconResults:
			s.consumeIconResult(result)
		case result := <-s.pickerResults:
			s.consumePickerResult(result)
		case result := <-s.backendResults:
			s.consumeBackendResult(result)
		case result := <-s.commandResults:
			delete(s.busyCommands, result.command)
			if isAudioDiscontinuityCommand(result.command) {
				s.finishAudioDiscontinuity(s.backend.State())
			}
			if result.err != nil {
				s.state = frontendStateForError(result.err)
				s.setStatus(s.trf(
					"%s: %s",
					s.backendCommandLabel(result.command),
					result.err.Error(),
				))
				continue
			}
			s.state = s.stableState()
			s.setStatus(s.trf(
				"%s: complete",
				s.backendCommandLabel(result.command),
			))
		case result := <-s.frameRunResults:
			if result.generation != s.frameGeneration {
				continue
			}
			s.frameRunPending = false
			s.recordPacingSample(
				result.startedAt,
				result.completedAt,
				result.guestAdvanced,
			)
			if result.err != nil {
				s.state = frontendStateForError(result.err)
				s.problem = &FrontendProblem{
					State:       s.state,
					Input:       displayNameForInfo(s.input),
					Backend:     s.backendName(),
					Reason:      result.err.Error(),
					Recoverable: true,
				}
				s.setStatus(s.tr("Run frame: ") + result.err.Error())
				s.promptFaultReport(s.problem)
			}
		case reason := <-s.faultReportRequests:
			s.consumeFaultReportRequest(reason)
		case result := <-s.dropResults:
			if result.err != nil {
				s.setStatus(s.tr("Drop: ") + result.err.Error())
				continue
			}
			if result.data != nil {
				s.openRequest(OpenRequest{
					Data:        result.data,
					DisplayName: result.displayName,
				})
				continue
			}
			s.openRequest(OpenRequest{
				Path:        result.path,
				DisplayName: result.displayName,
				Temporary:   true,
			})
		case result := <-s.artifactResults:
			if result.err != nil {
				s.setStatus(s.trf(
					"%s: %s",
					s.tr(settingValueLabel(result.kind)),
					result.err.Error(),
				))
				continue
			}
			if result.warning != "" {
				s.setStatus(s.trf(
					"%s saved with warning: %s (%s)",
					s.tr(settingValueLabel(result.kind)),
					result.path,
					result.warning,
				))
				continue
			}
			s.setStatus(s.trf(
				"%s saved: %s",
				s.tr(settingValueLabel(result.kind)),
				result.path,
			))
		case result := <-s.saveRestoreResults:
			if result.err != nil {
				s.setStatus(s.tr("Restore save: ") + result.err.Error())
				continue
			}
			s.setStatus(s.trf("Save restored from %s", result.name))
		case result := <-s.issueReportResults:
			s.consumeIssueReportResult(result)
		case result := <-s.issueCommentResults:
			s.consumeIssueCommentResult(result)
		case result := <-s.toolResults:
			s.consumeToolResult(result)
		case result := <-s.updateResults:
			s.consumeUpdateResult(result)
		case result := <-s.updateCheckResults:
			s.consumeUpdateCheckResult(result)
		default:
			return
		}
	}
}

func (s *Shell) consumeBackendResult(result backendResult) {
	s.loading = false
	if result.request.Temporary {
		s.temporaryPath = result.request.Path
	}
	if result.info.DisplayName != "" {
		s.input = &result.info
		s.firmwareSession = result.request.Firmware
		s.selectedPath = result.request.Path
		if result.request.Path != "" && !result.request.Temporary {
			s.settings.addRecent(result.request.Path, result.info.DisplayName)
			_ = s.settings.save()
		}
	}
	if result.err != nil {
		s.state = frontendStateForError(result.err)
		s.problem = &FrontendProblem{
			State:       s.state,
			Input:       displayName(result.request),
			Format:      result.info.Format,
			Profile:     result.info.ProfileID,
			Backend:     s.backendName(),
			Reason:      result.err.Error(),
			Recoverable: true,
		}
		var backendError *BackendError
		if errors.As(result.err, &backendError) && backendError.Backend != "" {
			s.problem.Backend = backendError.Backend
		}
		s.setStatus(fmt.Sprintf("%s: %v", displayName(result.request), result.err))
		return
	}

	s.problem = nil
	s.frameGeneration++
	s.frameRunPending = false
	s.state = s.stableState()
	if s.state == FrontendEmpty {
		s.state = FrontendReady
	}
	s.setStatus(s.trf(
		"Loaded %s | %s | profile %s",
		result.info.DisplayName,
		emptyFallback(result.info.Format, s.tr("unknown")),
		emptyFallback(result.info.ProfileID, s.tr("auto")),
	))
	setPlatformWindowTitle("ARAM - " + result.info.DisplayName)

	backendState := s.backend.State()
	s.finishAudioDiscontinuity(backendState)
	if (backendState == StateReady || backendState == StatePaused) &&
		s.backend.Supports(CommandStart) {
		s.executeBackend(CommandStart)
	}
}
