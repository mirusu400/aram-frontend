package frontend

import (
	"context"
	"errors"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type operation uint8

const (
	operationOpen operation = iota
	operationFirmware
	operationRecent
)

type pickerResult struct {
	operation operation
	path      string
	err       error
}

func (s *Shell) consumePickerResult(result pickerResult) {
	s.dialogOpen = false
	if result.err != nil {
		s.state = s.preDialogState
		switch {
		case errors.Is(result.err, ErrPickerCanceled):
			s.setStatus(s.tr("Selection canceled"))
		case errors.Is(result.err, ErrPickerDeferred):
			s.setStatus(s.tr("Waiting for the native document picker..."))
		case errors.Is(result.err, ErrPickerUnavailable):
			s.setStatus(s.tr("Use the native mobile document picker"))
		default:
			s.setStatus(s.tr("File picker: ") + result.err.Error())
		}
		return
	}
	s.openRequest(OpenRequest{
		Path:     result.path,
		Firmware: result.operation == operationFirmware,
	})
}

func (s *Shell) chooseFile() {
	if s.dialogOpen || s.loading {
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	s.setStatus(s.tr("Waiting for file selection..."))
	go func() {
		path, err := s.picker.OpenFile()
		s.pickerResults <- pickerResult{operation: operationOpen, path: path, err: err}
	}()
}

func (s *Shell) chooseFirmwareDirectory() {
	if s.dialogOpen || s.loading {
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	s.setStatus(s.tr("Waiting for firmware directory selection..."))
	go func() {
		path, err := s.picker.OpenFirmwareDirectory(s.settings.LastFirmwarePath)
		s.pickerResults <- pickerResult{operation: operationFirmware, path: path, err: err}
	}()
}

func (s *Shell) chooseRecent() {
	if s.dialogOpen || len(s.settings.RecentFiles) == 0 {
		return
	}
	if s.interfaceUI != nil {
		s.panel = &Panel{
			Kind:  "recent",
			Title: "Open Recent",
		}
		s.setStatus(s.tr("Select a recent input"))
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	recent := append([]string(nil), s.settings.RecentFiles...)
	s.setStatus(s.tr("Choose a recent input..."))
	go func() {
		path, err := s.picker.ChooseRecent(recent)
		s.pickerResults <- pickerResult{operation: operationRecent, path: path, err: err}
	}()
}

func (s *Shell) openRecentPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		s.setStatus(s.tr("Open recent: no input selected"))
		return
	}
	s.panel = nil
	s.openRequest(OpenRequest{Path: path})
}

func (s *Shell) openRequest(request OpenRequest) {
	if s.loading {
		s.setStatus(s.tr("An input is already loading"))
		return
	}
	if s.input != nil {
		if err := s.releaseCurrentInput(); err != nil {
			s.setStatus(s.tr("Close current title: ") + err.Error())
			return
		}
	}
	s.loading = true
	s.problem = nil
	s.state = FrontendInspecting
	s.setStatus(s.trf("Inspecting %s...", displayName(request)))
	if request.Firmware && request.Path != "" {
		s.settings.LastFirmwarePath = request.Path
		_ = s.settings.save()
	}
	go func() {
		progress := func(stage OpenStage) {
			select {
			case s.openStageResults <- stage:
			default:
			}
		}
		var (
			info InputInfo
			err  error
		)
		if backend, ok := s.backend.(OpenProgressBackend); ok {
			info, err = backend.OpenWithProgress(context.Background(), request, progress)
		} else {
			progress(OpenStageLoading)
			info, err = s.backend.Open(context.Background(), request)
		}
		s.backendResults <- backendResult{request: request, info: info, err: err}
	}()
}

func (s *Shell) executeBackend(command BackendCommand) {
	if s.busyCommands[command] {
		s.setStatus(s.trf(
			"%s: already in progress",
			s.backendCommandLabel(command),
		))
		return
	}
	s.busyCommands[command] = true
	s.setStatus(s.trf("%s...", s.backendCommandLabel(command)))
	request := CommandRequest{
		Command: command,
		Slot:    s.settings.StateSlot,
		Speed:   s.settings.Speed,
	}
	go func() {
		var err error
		if backend, ok := s.backend.(CommandBackend); ok {
			err = backend.ExecuteCommand(context.Background(), request)
		} else {
			err = s.backend.Execute(context.Background(), command)
		}
		s.commandResults <- commandResult{command: command, err: err}
	}()
}

func (s *Shell) handleDroppedFiles() {
	files := ebiten.DroppedFiles()
	if files == nil || s.loading {
		return
	}
	s.state = FrontendInspecting
	s.setStatus(s.tr("Copying dropped input into the ARAM cache..."))
	go copyFirstDroppedFile(files, s.dropResults)
}
