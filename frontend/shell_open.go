package frontend

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type operation uint8

const (
	operationOpen operation = iota
	operationFirmware
	operationRecent
	operationImportSave
	operationLibraryFolder
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
	if result.operation == operationImportSave {
		s.state = s.preDialogState
		s.importSaveDataFromPath(result.path)
		return
	}
	if result.operation == operationLibraryFolder {
		s.state = s.preDialogState
		s.addLibraryFolderPath(result.path)
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
	recent := recentEntryPaths(s.settings.RecentFiles)
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
	s.openRequest(OpenRequest{Path: path, DisplayName: s.rememberedDisplayName(path)})
}

// rememberedDisplayName recovers the name the recent list already knows for
// path. A mobile host imports an input into private storage under a generated
// file name, so the path's own base name is a UUID the user never saw; the
// name the host reported at import time is kept in the recent entry. Reopening
// without it would both mislabel the session and, through addRecent, overwrite
// the good name with the generated one.
func (s *Shell) rememberedDisplayName(path string) string {
	// addRecent stores an absolute, cleaned path, so match the way it wrote
	// the entry rather than the string a caller happened to hold.
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	for _, entry := range s.settings.RecentFiles {
		if strings.EqualFold(filepath.Clean(entry.Path), path) {
			return entry.Name
		}
	}
	return ""
}

func (s *Shell) openRequest(request OpenRequest) {
	if s.loading {
		s.setStatus(s.tr("An input is already loading"))
		return
	}
	if s.input != nil {
		if err := s.releaseCurrentInput(true); err != nil {
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
	if isAudioDiscontinuityCommand(command) {
		s.beginAudioDiscontinuity()
	}
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

// startCurrentTitle begins or resumes the loaded input. A machine sitting in
// StateStopped - the user's own Stop, or the guest exiting on its own -
// otherwise resumes through the backend's own soft reset, which reuses the
// already constructed machine and its original factory settings exactly like
// the backend's Reset command did. Routing that case through the same full
// close/reopen as Reset means a plain Stop-then-Start also picks up the
// current widescreen/font/CPU/audio choices instead of stale ones.
func (s *Shell) startCurrentTitle() {
	if s.input != nil && s.backend.State() == StateStopped {
		s.restartCurrentTitle()
		return
	}
	s.executeBackend(CommandStart)
}

// restartCurrentTitle fully closes and reopens the input that is currently
// loaded. A geometry-only change such as the widescreen override is read by
// the backend's machine factory, so it only takes effect on a fresh Open; the
// backend's own reset (and a plain Stop/Start pair) reuse the already
// constructed machine and its original geometry. Close() performs a full,
// synchronous teardown, so no artificial settle delay is needed before the
// reopen.
func (s *Shell) restartCurrentTitle() {
	if s.input == nil || s.loading {
		return
	}
	request := s.lastOpenRequest
	if err := s.releaseCurrentInput(false); err != nil {
		s.setStatus(s.tr("Restart: ") + err.Error())
		return
	}
	s.openRequest(request)
}

func (s *Shell) handleDroppedFiles() {
	files := ebiten.DroppedFiles()
	if files == nil || s.loading {
		return
	}
	s.state = FrontendInspecting
	s.setStatus(s.tr("Reading dropped input..."))
	go readFirstDroppedFile(files, s.dropResults)
}
