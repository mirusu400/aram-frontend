package frontend

import (
	"context"
	"errors"
	"path/filepath"
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

type Backend interface {
	Open(context.Context, OpenRequest) (InputInfo, error)
	State() BackendState
	Supports(BackendCommand) bool
	Execute(context.Context, BackendCommand) error
	Close() error
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
func (NullBackend) Execute(context.Context, BackendCommand) error {
	return ErrBackendUnavailable
}
func (NullBackend) Close() error { return nil }
