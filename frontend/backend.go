package frontend

import (
	"context"
	"errors"
	"image"
	"path/filepath"
	"time"
)

var ErrBackendUnavailable = errors.New(
	"emulation backend is not attached; run the integrated product from " +
		"aram-emu with `go run ./cmd/aram`",
)

type BackendState string

const (
	StateEmpty   BackendState = "empty"
	StateReady   BackendState = "ready"
	StateRunning BackendState = "running"
	StatePaused  BackendState = "paused"
	StateStopped BackendState = "stopped"
	StateFaulted BackendState = "faulted"
)

type OpenRequest struct {
	Path        string
	DisplayName string
	Firmware    bool
	Temporary   bool
}

type InputInfo struct {
	DisplayName string
	Format      string
	Size        int64
	SHA256      string
	ProfileID   string
}

type BackendCommand string

const (
	CommandStart       BackendCommand = "start"
	CommandPauseResume BackendCommand = "pause-resume"
	CommandStop        BackendCommand = "stop"
	CommandReset       BackendCommand = "reset"
	CommandFrame       BackendCommand = "frame-advance"
	CommandFastForward BackendCommand = "fast-forward"
	CommandLoadState   BackendCommand = "load-state"
	CommandSaveState   BackendCommand = "save-state"
	CommandRewind      BackendCommand = "rewind"
)

type OpenStage string

const (
	OpenStageInspecting OpenStage = "inspecting"
	OpenStageLoading    OpenStage = "loading"
)

type FailureKind string

const (
	FailureBackendUnavailable FailureKind = "backend-unavailable"
	FailureGuestFaulted       FailureKind = "guest-faulted"
	FailureMalformedInput     FailureKind = "malformed-input"
	FailureUnsupportedProfile FailureKind = "unsupported-profile"
	FailureUnknown            FailureKind = "unknown"
)

type BackendError struct {
	Kind    FailureKind
	Backend string
	Reason  string
	Err     error
}

func (e *BackendError) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason != "" {
		return e.Reason
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *BackendError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Capability struct {
	Supported bool
	Reason    string
}

type CommandRequest struct {
	Command BackendCommand
	Slot    int
	Speed   float64
}

type VideoFrame struct {
	Image    image.Image
	Sequence uint64
}

type InputEvent struct {
	Control string
	Pressed bool
	// At is guest-relative. Zero applies live host input at the backend's
	// current guest time.
	At time.Duration
}

type AudioSettings struct {
	Muted    bool
	Volume   int
	Latency  time.Duration
	DeviceID string
}

type AudioDevice struct {
	ID   string
	Name string
}

// AudioChunk is signed PCM16 produced by a backend. Samples are interleaved
// when Channels is greater than one.
type AudioChunk struct {
	SampleRate int
	Channels   int
	PCM16      []int16
}

type ToolKind string

const (
	ToolCheats        ToolKind = "cheats"
	ToolMemory        ToolKind = "memory"
	ToolPatches       ToolKind = "patches"
	ToolDebugger      ToolKind = "debugger"
	ToolLogs          ToolKind = "logs"
	ToolCompatibility ToolKind = "compatibility"
)

type ToolSnapshot struct {
	Title   string
	Lines   []string
	Fields  []ToolField
	Actions []ToolAction
}

type ToolField struct {
	ID          string
	Label       string
	Value       string
	Placeholder string
}

type ToolAction struct {
	ID      string
	Label   string
	Enabled bool
}

type ToolRequest struct {
	Kind   ToolKind
	Action string
	Fields map[string]string
}

type Backend interface {
	Open(context.Context, OpenRequest) (InputInfo, error)
	State() BackendState
	Supports(BackendCommand) bool
	Execute(context.Context, BackendCommand) error
	Close() error
}

// OpenProgressBackend lets an integration adapter expose the inspect/load
// boundary without making the shared frontend understand core source handles.
type OpenProgressBackend interface {
	OpenWithProgress(context.Context, OpenRequest, func(OpenStage)) (InputInfo, error)
}

// CapabilityBackend supplies the reason shown when a persistent command is
// disabled. Backends that do not implement it fall back to Supports.
type CapabilityBackend interface {
	Capability(BackendCommand) Capability
}

// CommandBackend receives frontend-owned command parameters such as state
// slot and speed. Execute remains the compatibility path for small backends.
type CommandBackend interface {
	ExecuteCommand(context.Context, CommandRequest) error
}

// VideoBackend publishes immutable guest-native frames. Sequence changes when
// Image changes, allowing the frontend to avoid recreating a GPU image.
type VideoBackend interface {
	VideoFrame() VideoFrame
}

// FrameBackend advances one presentation quantum while the backend is in its
// logical running state. The frontend schedules at most one call at a time so
// implementations can keep guest execution off the UI thread.
type FrameBackend interface {
	RunFrame(context.Context) error
}

// InputBackend receives normalized controls rather than host-specific keys.
type InputBackend interface {
	QueueInput(InputEvent) error
}

type AudioBackend interface {
	ConfigureAudio(AudioSettings) error
}

// AudioStreamBackend transfers guest-generated PCM to the host output owned by
// the frontend. DrainAudio must return quickly and transfer ownership of PCM16
// to the caller.
type AudioStreamBackend interface {
	DrainAudio() AudioChunk
}

// AudioDeviceBackend publishes a fast, immutable snapshot of selectable
// output devices. An empty ID always represents the host system default.
type AudioDeviceBackend interface {
	AudioDevices() []AudioDevice
}

// ToolBackend provides read-only tool data. Memory mutation and debugger
// control remain backend operations and never leak guest memory into the UI.
type ToolBackend interface {
	ToolSnapshot(context.Context, ToolKind) (ToolSnapshot, error)
}

// ToolActionBackend executes checked tool operations. The frontend only owns
// form state and normalized action IDs; all guest inspection and mutation
// remains behind the backend boundary.
type ToolActionBackend interface {
	ExecuteToolAction(context.Context, ToolRequest) (ToolSnapshot, error)
}

type BackendNamer interface {
	BackendName() string
}

type NullBackend struct{}

func (NullBackend) Open(_ context.Context, request OpenRequest) (InputInfo, error) {
	name := request.DisplayName
	if name == "" {
		name = filepath.Base(request.Path)
	}
	return InputInfo{
		DisplayName: name,
		Format:      "uninspected",
	}, ErrBackendUnavailable
}

func (NullBackend) State() BackendState          { return StateEmpty }
func (NullBackend) Supports(BackendCommand) bool { return false }
func (NullBackend) Capability(BackendCommand) Capability {
	return Capability{
		Supported: false,
		Reason: "Frontend preview only; run `go run ./cmd/aram` from " +
			"aram-emu to use emulation commands",
	}
}
func (NullBackend) Execute(context.Context, BackendCommand) error {
	return ErrBackendUnavailable
}
func (NullBackend) BackendName() string { return "not attached" }
func (NullBackend) Close() error        { return nil }
