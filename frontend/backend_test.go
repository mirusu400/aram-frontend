package frontend

import (
	"context"
	"errors"
	"image"
	"strings"
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

func TestNullBackendExplainsHowToRunIntegratedProduct(t *testing.T) {
	_, err := (NullBackend{}).Open(context.Background(), OpenRequest{
		Path: `games/example.dat`,
	})
	if err == nil ||
		!strings.Contains(err.Error(), `aram-emu`) ||
		!strings.Contains(err.Error(), `go run ./cmd/aram`) {
		t.Fatalf("NullBackend Open error = %v", err)
	}
	capability := (NullBackend{}).Capability(CommandStart)
	if capability.Supported ||
		!strings.Contains(capability.Reason, `go run ./cmd/aram`) {
		t.Fatalf("NullBackend start capability = %+v", capability)
	}
}

func TestStandaloneShellStatusExplainsHowToRunIntegratedProduct(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	shell := NewShell(NullBackend{}, nil, "")
	want := shell.tr("Frontend preview only - run `go run ./cmd/aram` from aram-emu")
	if shell.status != want {
		t.Fatalf("standalone shell status = %q", shell.status)
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

type autoStartBackend struct {
	mu       sync.Mutex
	state    BackendState
	commands chan BackendCommand
}

func (backend *autoStartBackend) Open(
	context.Context,
	OpenRequest,
) (InputInfo, error) {
	return InputInfo{DisplayName: "synthetic.dat"}, nil
}

func (backend *autoStartBackend) State() BackendState {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.state
}

func (*autoStartBackend) Supports(command BackendCommand) bool {
	return command == CommandStart
}

func (backend *autoStartBackend) Execute(
	_ context.Context,
	command BackendCommand,
) error {
	backend.mu.Lock()
	if command == CommandStart {
		backend.state = StateRunning
	}
	backend.mu.Unlock()
	backend.commands <- command
	return nil
}

func (*autoStartBackend) Close() error { return nil }

func TestLoadedReadyOrPausedInputAutomaticallyStarts(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)

	for _, initialState := range []BackendState{StateReady, StatePaused} {
		t.Run(string(initialState), func(t *testing.T) {
			backend := &autoStartBackend{
				state:    initialState,
				commands: make(chan BackendCommand, 1),
			}
			shell := NewShell(backend, nil, "")

			shell.consumeBackendResult(backendResult{
				request: OpenRequest{Path: "synthetic.dat"},
				info: InputInfo{
					DisplayName: "synthetic.dat",
					Format:      "eads",
				},
			})

			select {
			case command := <-backend.commands:
				if command != CommandStart {
					t.Fatalf("automatic command = %q, want %q", command, CommandStart)
				}
			case <-time.After(time.Second):
				t.Fatal("loaded input did not automatically start")
			}

			deadline := time.Now().Add(time.Second)
			for shell.busyCommands[CommandStart] && time.Now().Before(deadline) {
				shell.consumeResults()
				time.Sleep(time.Millisecond)
			}
			if shell.busyCommands[CommandStart] {
				t.Fatal("automatic start did not complete")
			}
			if state := backend.State(); state != StateRunning {
				t.Fatalf("backend state = %q, want %q", state, StateRunning)
			}
			if shell.state != FrontendRunning {
				t.Fatalf("frontend state = %q, want %q", shell.state, FrontendRunning)
			}
		})
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

type configurableAudioBackend struct {
	NullBackend
	settings AudioSettings
}

func (backend *configurableAudioBackend) ConfigureAudio(settings AudioSettings) error {
	backend.settings = settings
	return nil
}

func (*configurableAudioBackend) AudioDevices() []AudioDevice {
	return []AudioDevice{
		{ID: "speakers", Name: "Desk speakers"},
		{ID: "headset", Name: "USB headset"},
		{ID: "speakers", Name: "Duplicate"},
	}
}

func TestAudioDeviceSelectionReachesBackend(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	backend := &configurableAudioBackend{}
	shell := NewShell(backend, nil, "")
	shell.settings.AudioDeviceID = ""

	shell.cycleAudioDevice()

	if shell.settings.AudioDeviceID != "speakers" {
		t.Fatalf("selected audio device = %q", shell.settings.AudioDeviceID)
	}
	if backend.settings.DeviceID != "speakers" {
		t.Fatalf("backend audio settings = %#v", backend.settings)
	}
	if got := len(shell.audioDevices()); got != 3 {
		t.Fatalf("deduplicated audio devices = %d", got)
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

type schedulingBackend struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	state   BackendState
}

func (*schedulingBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	return InputInfo{DisplayName: "synthetic.dat"}, nil
}

func (backend *schedulingBackend) State() BackendState {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.state
}

func (*schedulingBackend) Supports(BackendCommand) bool {
	return true
}

func (*schedulingBackend) Execute(context.Context, BackendCommand) error {
	return nil
}

func (*schedulingBackend) Close() error {
	return nil
}

func (backend *schedulingBackend) RunFrame(context.Context) error {
	backend.mu.Lock()
	backend.calls++
	backend.mu.Unlock()
	backend.started <- struct{}{}
	<-backend.release
	return nil
}

func TestShellContinuouslySchedulesOneRunningFrameAtATime(t *testing.T) {
	backend := &schedulingBackend{
		started: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
		state:   StateRunning,
	}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	// Pacing is driven from the clock, so the test advances it by a whole
	// quantum whenever it expects another frame to be issued.
	clock := time.Now()
	shell.nowFunc = func() time.Time { return clock }
	advance := func() { clock = clock.Add(shell.frameQuantum()) }

	shell.scheduleRunningFrame()
	waitSignal(t, backend.started, "first frame")
	shell.scheduleRunningFrame()
	backend.mu.Lock()
	calls := backend.calls
	backend.mu.Unlock()
	if calls != 1 {
		t.Fatalf("concurrent frame calls = %d, want 1", calls)
	}

	backend.release <- struct{}{}
	waitFrameCompletion(t, shell)
	advance()
	shell.scheduleRunningFrame()
	waitSignal(t, backend.started, "second frame")
	backend.mu.Lock()
	calls = backend.calls
	backend.mu.Unlock()
	if calls != 2 {
		t.Fatalf("sequential frame calls = %d, want 2", calls)
	}
	backend.release <- struct{}{}
	waitFrameCompletion(t, shell)

	backend.mu.Lock()
	backend.state = StatePaused
	backend.mu.Unlock()
	advance()
	shell.scheduleRunningFrame()
	backend.mu.Lock()
	calls = backend.calls
	backend.mu.Unlock()
	if calls != 2 {
		t.Fatalf("paused backend was advanced; calls = %d", calls)
	}
}

func waitSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func waitFrameCompletion(t *testing.T, shell *Shell) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for shell.frameRunPending && time.Now().Before(deadline) {
		shell.consumeResults()
		time.Sleep(time.Millisecond)
	}
	if shell.frameRunPending {
		t.Fatal("timed out waiting for scheduled frame completion")
	}
}
