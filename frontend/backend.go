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
	// Data carries the input bytes in-band for a host that has no readable
	// filesystem path for the selection - the web/wasm build, whose picker
	// reads a browser File into memory rather than a disk path. When Data is
	// non-empty an adapter loads from it and ignores Path; DisplayName still
	// names the input for the UI. Desktop and mobile leave Data nil and pass a
	// Path (or a native-host cache handle).
	Data []byte
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
	Image      image.Image
	Sequence   uint64
	GuestNS    int64
	Generation uint64
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
	// MixMode selects the enhanced audio policy where sound effects mix over a
	// looping track instead of the title being able to silence it. False keeps
	// the faithful device behaviour. It is baked into the next machine created.
	MixMode bool
	// Soften applies a gentle output low-pass that tames the harsh top end of
	// the guest's FM (Yamaha MA-3) synthesis. It is a pure playback filter, not
	// a change to the emulated audio, so it takes effect immediately.
	Soften bool
}

type AudioDevice struct {
	ID   string
	Name string
}

// AudioChunk is signed PCM16 produced by a backend. Samples are interleaved
// when Channels is greater than one.
type AudioChunk struct {
	SampleRate   int
	Channels     int
	PCM16        []int16
	StartGuestNS int64
	StartSample  uint64
	Generation   uint64
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

// FirmwareBackend is implemented by a backend that can open a whole-phone
// firmware directory. A backend that only runs application titles leaves it
// unimplemented, and the frontend disables its firmware command rather than
// offering a menu entry whose only outcome is an error.
type FirmwareBackend interface {
	SupportsFirmware() bool
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

// CPUSettings selects which CPU backend (core) executes the guest. Name is a
// stable identifier such as "precise"; an empty or unknown name keeps the
// backend's default core.
type CPUSettings struct {
	Name string
}

// CPUBackendSelector is implemented by backends that can switch the CPU core.
// AvailableCPUBackends lists the selectable core names for the settings UI, and
// ConfigureCPU records the choice, which takes effect the next time a title is
// opened.
type CPUBackendSelector interface {
	ConfigureCPU(CPUSettings) error
	AvailableCPUBackends() []string
}

// DisplaySettings overrides the guest framebuffer geometry the backend hands to
// a title. A zero Width and Height keep the device-native size. This is an
// experimental control: only a title that lays its scene out from the
// runtime-reported screen size (camera-scrolled titles) fills the extra area;
// others letterbox or leave margins.
type DisplaySettings struct {
	Width  int
	Height int
}

// DisplayConfigurator is implemented by backends that can override the guest
// framebuffer geometry. The size takes effect the next time a title is opened.
type DisplayConfigurator interface {
	ConfigureDisplay(DisplaySettings) error
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

// HapticsState is the guest's current vibration request. Level is 0-100 motor
// strength; Duration is the time remaining before it stops. A zero Level means
// no vibration is active.
type HapticsState struct {
	Level    uint8
	Duration time.Duration
}

// HapticsBackend publishes the guest's vibration request so the frontend can
// actuate a real gamepad rumble motor or phone vibrator. It is optional; a
// backend that cannot report vibration simply does not implement it, and the
// frontend then drives no haptics.
type HapticsBackend interface {
	Haptics() HapticsState
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

// SaveTransferBackend exports and imports the loaded title's writable storage
// (its flash save) as a portable, self-describing backup blob. It lets a save
// survive loss of the local state directory: the user writes the blob somewhere
// safe and restores it later, even onto another install. ExportSaveData returns
// the backup bytes for the loaded title; ImportSaveData validates a backup,
// refuses one that belongs to a different title, and applies it. A backend that
// does not implement it simply offers no save backup, and the frontend reports
// that rather than silently doing nothing.
type SaveTransferBackend interface {
	ExportSaveData() ([]byte, error)
	ImportSaveData([]byte) error
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
