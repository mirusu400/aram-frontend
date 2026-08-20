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
	// ImageSHA256 identifies the loaded executable image rather than the file
	// that delivered it, so it survives re-archiving. Backends that cannot
	// describe an image leave it empty.
	ImageSHA256 string
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
	// AllowGuestInput keeps host input reaching the guest while this panel is
	// open. A panel meant to be used mid-play, such as cheats, sets it; one
	// with text entry must not, or typing would drive the game as well.
	AllowGuestInput bool
}

type ToolField struct {
	ID          string
	Label       string
	Value       string
	Placeholder string
	Detail      string
	Options     []ToolFieldOption
	Checkbox    bool
	// Action makes a control self-applying: changing it runs that tool action
	// with the panel's current field values instead of waiting for a button.
	// A list of toggles needs it, since a per-row button would be noise.
	Action string
}

type ToolFieldOption struct {
	Value string
	Label string
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

// DebugArtifact is a backend-owned diagnostic file for a user-exported debug
// bundle. Data must contain logs or metadata only, never source images,
// framebuffer pixels, guest memory dumps, persistence, or proprietary media.
type DebugArtifact struct {
	Name      string
	MediaType string
	Data      []byte
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

// FontSettings selects the handset bitmap font the backend uses to render
// in-game text that has no glyphs of its own. Name is a stable identifier
// ("galmuri9", "neodgm", or "custom"); an empty or unknown name lets the backend
// keep its default. When Data is non-empty it holds a user-supplied BDF or
// TrueType/OpenType font file, which the backend builds and uses in place of a
// named built-in.
type FontSettings struct {
	Name string
	Data []byte
}

// FontBackend is implemented by backends that can switch the handset fallback
// font. The selection takes effect the next time a title is opened.
type FontBackend interface {
	ConfigureFont(FontSettings) error
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

// DebugExportBackend contributes bounded backend diagnostics to the
// frontend-owned ZIP bundle. Collection errors are recorded as warnings so
// frontend logs can still be exported after a backend fault.
type DebugExportBackend interface {
	DebugArtifacts(context.Context) ([]DebugArtifact, error)
}

type BackendNamer interface {
	BackendName() string
}

// ProductUpdate is a verified integrated ARAM archive selected by the shared
// frontend. ProductUpdateInstaller is implemented by the integration hosts
// because only a host knows how to install its own package: the desktop host
// extracts the archive and relaunches its executable, while the Android host
// hands the APK to the system package installer.
type ProductUpdate struct {
	Channel      string
	Version      string
	ArchivePath  string
	RelaunchPath string
}

type ProductUpdateInstaller interface {
	InstallProductUpdate(ProductUpdate) error
}

// ErrProductInstallDeferred is returned by a ProductUpdateInstaller whose
// platform finishes the installation outside the running process, such as the
// Android package installer. The frontend keeps the archive for that installer,
// stays running, and reports the hand-off instead of restarting.
var ErrProductInstallDeferred = errors.New(
	"product installation delegated to the platform installer",
)

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
