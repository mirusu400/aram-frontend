package frontend

import (
	"context"
	"errors"
	"image"
	"path/filepath"
	"time"
)

var ErrBackendUnavailable = errors.New("emulation backend is not attached")

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
	At      time.Duration
}

type AudioSettings struct {
	Muted    bool
	Volume   int
	Latency  time.Duration
	DeviceID string
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
	Title string
	Lines []string
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

// InputBackend receives normalized controls rather than host-specific keys.
type InputBackend interface {
	QueueInput(InputEvent) error
}

type AudioBackend interface {
	ConfigureAudio(AudioSettings) error
}

// ToolBackend provides read-only tool data. Memory mutation and debugger
// control remain backend operations and never leak guest memory into the UI.
type ToolBackend interface {
	ToolSnapshot(context.Context, ToolKind) (ToolSnapshot, error)
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
		Reason:    "Connect an aram-core integration backend to use this command",
	}
}
func (NullBackend) Execute(context.Context, BackendCommand) error {
	return ErrBackendUnavailable
}
func (NullBackend) BackendName() string { return "not attached" }
func (NullBackend) Close() error        { return nil }
