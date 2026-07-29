package frontend

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"
)

func TestNullBackendPreservesSelectedName(t *testing.T) {
	info, err := (NullBackend{}).Open(context.Background(), OpenRequest{
		Path: `games/example.dat`,
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("Open error = %v", err)
	}
	if info.DisplayName != "example.dat" {
		t.Fatalf("DisplayName = %q", info.DisplayName)
	}
}

func TestBackendErrorPreservesCause(t *testing.T) {
	cause := errors.New("synthetic failure")
	err := &BackendError{
		Kind:   FailureMalformedInput,
		Reason: "input is malformed",
		Err:    cause,
	}
	if err.Error() != "input is malformed" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("BackendError does not unwrap its cause")
	}
	if got := frontendStateForError(err); got != FrontendMalformedInput {
		t.Fatalf("frontendStateForError = %q", got)
	}
}

type recordingBackend struct {
	requests chan CommandRequest
}

func (backend *recordingBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	return InputInfo{DisplayName: "synthetic.dat"}, nil
}

func (backend *recordingBackend) State() BackendState {
	return StateReady
}

func (backend *recordingBackend) Supports(BackendCommand) bool {
	return true
}

func (backend *recordingBackend) Execute(context.Context, BackendCommand) error {
	return errors.New("legacy Execute called")
}

func (backend *recordingBackend) ExecuteCommand(_ context.Context, request CommandRequest) error {
	backend.requests <- request
	return nil
}

func (backend *recordingBackend) Close() error {
	return nil
}

func TestParameterizedCommandCarriesSlotAndSpeed(t *testing.T) {
	backend := &recordingBackend{requests: make(chan CommandRequest, 1)}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	shell.settings.StateSlot = 7
	shell.settings.Speed = 4

	shell.executeBackend(CommandSaveState)
	request := <-backend.requests
	if request.Command != CommandSaveState || request.Slot != 7 || request.Speed != 4 {
		t.Fatalf("command request = %#v", request)
	}
}

func TestFrameDestinationPreservesAspectAndIntegerScale(t *testing.T) {
	shell := &Shell{settings: defaultSettings()}
	destination := shell.frameDestination(image.Rect(0, 0, 650, 650), 240, 320)
	if destination.Dx() != 480 || destination.Dy() != 640 {
		t.Fatalf("destination = %v, want 480x640", destination)
	}
	if destination.Min.X != 85 || destination.Min.Y != 5 {
		t.Fatalf("destination origin = %v", destination.Min)
	}
}

type lifecycleBackend struct {
	mu       sync.Mutex
	state    BackendState
	requests chan CommandRequest
}

func (backend *lifecycleBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	return InputInfo{DisplayName: "synthetic.dat"}, nil
}

func (backend *lifecycleBackend) State() BackendState {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.state
}

func (backend *lifecycleBackend) Supports(BackendCommand) bool {
	return true
}

func (backend *lifecycleBackend) Execute(context.Context, BackendCommand) error {
	return errors.New("legacy Execute called")
}

func (backend *lifecycleBackend) ExecuteCommand(_ context.Context, request CommandRequest) error {
	backend.mu.Lock()
	if request.Command == CommandPauseResume {
		if backend.state == StateRunning {
			backend.state = StatePaused
		} else if backend.state == StatePaused {
			backend.state = StateRunning
		}
	}
	backend.mu.Unlock()
	backend.requests <- request
	return nil
}

func (backend *lifecycleBackend) Close() error {
	return nil
}

func TestHostLifecycleOnlyResumesAutomaticPause(t *testing.T) {
	backend := &lifecycleBackend{
		state:    StateRunning,
		requests: make(chan CommandRequest, 2),
	}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	shell.hostActive = false
	shell.syncHostLifecycle()
	if request := waitCommandRequest(t, backend.requests); request.Command != CommandPauseResume {
		t.Fatalf("pause request = %#v", request)
	}
	<-shell.commandResults
	delete(shell.busyCommands, CommandPauseResume)
	if !shell.hostPaused || backend.State() != StatePaused {
		t.Fatalf("automatic pause: hostPaused=%t state=%s", shell.hostPaused, backend.State())
	}

	shell.hostActive = true
	shell.syncHostLifecycle()
	if request := waitCommandRequest(t, backend.requests); request.Command != CommandPauseResume {
		t.Fatalf("resume request = %#v", request)
	}
	if shell.hostPaused || backend.State() != StateRunning {
		t.Fatalf("automatic resume: hostPaused=%t state=%s", shell.hostPaused, backend.State())
	}

	backend.mu.Lock()
	backend.state = StatePaused
	backend.mu.Unlock()
	shell.hostPaused = false
	shell.hostActive = false
	delete(shell.busyCommands, CommandPauseResume)
	shell.syncHostLifecycle()
	select {
	case request := <-backend.requests:
		t.Fatalf("manual pause was changed by lifecycle: %#v", request)
	default:
	}
}

func waitCommandRequest(t *testing.T, requests <-chan CommandRequest) CommandRequest {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for backend command")
		return CommandRequest{}
	}
}
