package frontend

import "errors"

type FrontendState string

const (
	FrontendEmpty              FrontendState = "empty"
	FrontendSelecting          FrontendState = "selecting"
	FrontendInspecting         FrontendState = "inspecting"
	FrontendLoading            FrontendState = "loading"
	FrontendReady              FrontendState = "ready"
	FrontendRunning            FrontendState = "running"
	FrontendPaused             FrontendState = "paused"
	FrontendStopped            FrontendState = "stopped"
	FrontendBackendUnavailable FrontendState = "backend-unavailable"
	FrontendGuestFaulted       FrontendState = "guest-faulted"
	FrontendMalformedInput     FrontendState = "malformed-input"
	FrontendUnsupportedProfile FrontendState = "unsupported-profile"
)

type FrontendProblem struct {
	State       FrontendState
	Input       string
	Format      string
	Profile     string
	Backend     string
	Reason      string
	Recoverable bool
}

func frontendStateForBackend(state BackendState) FrontendState {
	switch state {
	case StateReady:
		return FrontendReady
	case StateRunning:
		return FrontendRunning
	case StatePaused:
		return FrontendPaused
	case StateStopped:
		return FrontendStopped
	case StateFaulted:
		return FrontendGuestFaulted
	default:
		return FrontendEmpty
	}
}

func frontendStateForError(err error) FrontendState {
	var backendError *BackendError
	if errors.As(err, &backendError) {
		switch backendError.Kind {
		case FailureBackendUnavailable:
			return FrontendBackendUnavailable
		case FailureGuestFaulted:
			return FrontendGuestFaulted
		case FailureMalformedInput:
			return FrontendMalformedInput
		case FailureUnsupportedProfile:
			return FrontendUnsupportedProfile
		}
	}
	if errors.Is(err, ErrBackendUnavailable) {
		return FrontendBackendUnavailable
	}
	return FrontendGuestFaulted
}
