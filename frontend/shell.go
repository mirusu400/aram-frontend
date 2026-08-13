package frontend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	logicalWidth   = 960
	logicalHeight  = 720
	menuHeight     = menuBarHeight
	statusHeight   = statusBarHeight
	menuItemHeight = 40
	dropdownWidth  = 330
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

type backendResult struct {
	request OpenRequest
	info    InputInfo
	err     error
}

type commandResult struct {
	command BackendCommand
	err     error
}

type frameRunResult struct {
	generation uint64
	err        error
}

type Shell struct {
	backend                   Backend
	picker                    Picker
	design                    *ARAMDesignSystem
	interfaceUI               *shellUI
	menus                     []Menu
	settings                  Settings
	state                     FrontendState
	problem                   *FrontendProblem
	activeMenu                int
	focusMode                 bool
	status                    string
	input                     *InputInfo
	selectedPath              string
	temporaryPath             string
	dialogOpen                bool
	loading                   bool
	quitting                  bool
	hostActive                bool
	hostPaused                bool
	preDialogState            FrontendState
	panel                     *Panel
	settingsSection           string
	layoutWidth               int
	layoutHeight              int
	logs                      []string
	frame                     VideoFrame
	frameImage                *ebiten.Image
	audioOutput               *audioOutput
	controlState              map[string]bool
	directionPressOrder       []string
	bindingCapture            *bindingCapture
	gamepadMappingsLoaded     bool
	touchControls             map[ebiten.TouchID]string
	busyCommands              map[BackendCommand]bool
	frameRunPending           bool
	frameGeneration           uint64
	frameAccumulator          time.Duration
	lastFramePacingAt         time.Time
	pacingQuantaIssued        int
	pacingSampleStartedAt     time.Time
	measuredSpeed             float64
	nowFunc                   func() time.Time
	welcomeInstalling         bool
	updater                   updateDownloader
	issueRelay                issueRelayService
	updateProgress            map[updateComponent]updateProgress
	pickerResults             chan pickerResult
	backendResults            chan backendResult
	commandResults            chan commandResult
	frameRunResults           chan frameRunResult
	openStageResults          chan OpenStage
	externalOpen              chan OpenRequest
	externalCommands          chan string
	externalSelectionCanceled chan struct{}
	hostLifecycle             chan bool
	dropResults               chan dropResult
	artifactResults           chan artifactResult
	issueReportResults        chan issueReportResult
	issueCommentResults       chan issueCommentResult
	toolResults               chan toolResult
	updateResults             chan updateResult
}

func NewShell(backend Backend, picker Picker, initialPath string) *Shell {
	if backend == nil {
		backend = NullBackend{}
	}
	if picker == nil {
		picker = NewPlatformPicker()
	}
	settings := loadSettings()
	language := normalizeLanguage(settings.Language)
	if localized, ok := picker.(languageAwarePicker); ok {
		localized.SetLanguage(language)
	}
	initialStatus := translate(language, "Ready - use File > Open File...")
	if isPreviewBackend(backend) {
		initialStatus = translate(
			language,
			"Frontend preview only - run `go run ./cmd/aram` from aram-emu",
		)
	}
	shell := &Shell{
		backend:                   backend,
		picker:                    picker,
		settings:                  settings,
		state:                     FrontendEmpty,
		activeMenu:                -1,
		status:                    initialStatus,
		settingsSection:           "General",
		layoutWidth:               logicalWidth,
		layoutHeight:              logicalHeight,
		hostActive:                true,
		controlState:              make(map[string]bool),
		touchControls:             make(map[ebiten.TouchID]string),
		busyCommands:              make(map[BackendCommand]bool),
		updater:                   newGitHubUpdater(),
		issueRelay:                newIssueRelayClient(),
		updateProgress:            make(map[updateComponent]updateProgress),
		pickerResults:             make(chan pickerResult, 2),
		backendResults:            make(chan backendResult, 2),
		commandResults:            make(chan commandResult, 8),
		frameRunResults:           make(chan frameRunResult, 2),
		openStageResults:          make(chan OpenStage, 4),
		externalOpen:              make(chan OpenRequest, 2),
		externalCommands:          make(chan string, 4),
		externalSelectionCanceled: make(chan struct{}, 1),
		hostLifecycle:             make(chan bool, 2),
		dropResults:               make(chan dropResult, 2),
		artifactResults:           make(chan artifactResult, 4),
		issueReportResults:        make(chan issueReportResult, 2),
		issueCommentResults:       make(chan issueCommentResult, 2),
		toolResults:               make(chan toolResult, 2),
		updateResults:             make(chan updateResult, 4),
	}
	setPlatformWindowTitle(shell.tr("ARAM - Archived Runtime for ARM Mobiles"))
	if shell.shouldOpenWelcome() {
		shell.openWelcome()
	}
	shell.menus = defaultMenus()
	shell.design = newARAMDesignSystem(shell.settings.ThemeMode)
	shell.interfaceUI = newShellUI(shell, shell.design)
	if audio, ok := shell.backend.(AudioBackend); ok {
		if err := audio.ConfigureAudio(shell.currentAudioSettings()); err != nil {
			shell.appendLog(shell.tr("Audio settings: ") + err.Error())
		}
	}
	if applied, err := loadCustomGamepadMappings(); err != nil {
		shell.appendLog(shell.tr("Controller database: ") + err.Error())
	} else if applied {
		shell.gamepadMappingsLoaded = true
		shell.appendLog(shell.tr("Custom controller database loaded"))
	}
	shell.appendLog(shell.status)
	if initialPath != "" {
		shell.openRequest(OpenRequest{Path: initialPath})
	}
	return shell
}

// OpenExternalDocument is the mobile/native-host entry point after a platform
// document picker grants access. The integration layer may use Path as a cache
// file or another handle understood by its Backend implementation.
func (s *Shell) OpenExternalDocument(path, displayName string, firmware bool) {
	request := OpenRequest{Path: path, DisplayName: displayName, Firmware: firmware}
	select {
	case s.externalOpen <- request:
	default:
	}
}

// DispatchExternalCommand lets a native host invoke the same stable command
// IDs used by desktop menus without mutating Shell state from another thread.
func (s *Shell) DispatchExternalCommand(commandID string) {
	select {
	case s.externalCommands <- commandID:
	default:
	}
}

// SetHostActive is the Android/iOS lifecycle bridge. It only resumes a
// machine that was automatically paused by a prior inactive transition.
func (s *Shell) SetHostActive(active bool) {
	select {
	case s.hostLifecycle <- active:
	default:
	}
}

// CancelExternalDocumentSelection lets a native picker restore the shell
// after the user dismisses its platform-owned document UI.
func (s *Shell) CancelExternalDocumentSelection() {
	select {
	case s.externalSelectionCanceled <- struct{}{}:
	default:
	}
}

func (s *Shell) Update() error {
	if s.quitting {
		return ebiten.Termination
	}
	s.consumeResults()
	s.syncBackendState()
	s.syncHostLifecycle()
	s.updateVideo()
	s.updateAudio()
	s.handleDroppedFiles()
	if !s.handleBindingCapture() {
		s.handleShortcuts()
	}
	s.handleTouch()
	s.syncDesignSystem()
	if s.focusModeActive() {
		// The chrome is hidden, so the interface UI must not consume input.
	} else if s.interfaceUI != nil {
		s.interfaceUI.sync(s)
		s.interfaceUI.ui.Update()
	} else {
		s.handleMouse()
	}
	s.handleMappedInput()
	s.scheduleRunningFrame()
	return nil
}

func (s *Shell) syncDesignSystem() {
	if s.design != nil && s.design.Mode == s.settings.ThemeMode {
		return
	}
	s.design = newARAMDesignSystem(s.settings.ThemeMode)
	s.interfaceUI = newShellUI(s, s.design)
}

func (s *Shell) Draw(screen *ebiten.Image) {
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	screen.Fill(palette.Canvas)
	if s.focusModeActive() {
		s.drawFocusMode(screen)
		return
	}
	s.drawWorkspace(screen)
	s.drawTouchControls(screen)
	s.drawVirtualKeypad(screen)
	if s.interfaceUI != nil {
		s.interfaceUI.ui.Draw(screen)
	}
}

func (s *Shell) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		outsideWidth, outsideHeight = logicalWidth, logicalHeight
	}
	s.layoutWidth = outsideWidth
	s.layoutHeight = outsideHeight
	return outsideWidth, outsideHeight
}

func (s *Shell) viewportSize() (int, int) {
	width, height := s.layoutWidth, s.layoutHeight
	if width <= 0 || height <= 0 {
		return logicalWidth, logicalHeight
	}
	return width, height
}

func (s *Shell) handleShortcuts() {
	control := ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift) ||
		ebiten.IsKeyPressed(ebiten.KeyShiftLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	if s.panel != nil {
		s.handlePanelShortcuts(control)
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.activeMenu = -1
		return
	}

	switch {
	case control && shift && inpututil.IsKeyJustPressed(ebiten.KeyD):
		s.dispatchCommand("tools.export_debug")
	case control && shift && inpututil.IsKeyJustPressed(ebiten.KeyS):
		s.dispatchCommand("view.screenshot")
	case control && inpututil.IsKeyJustPressed(ebiten.KeyO):
		s.dispatchCommand("file.open")
	case control && inpututil.IsKeyJustPressed(ebiten.KeyR):
		s.dispatchCommand("emu.reset")
	case control && inpututil.IsKeyJustPressed(ebiten.KeyDigit0):
		s.dispatchCommand("view.fit")
	case inpututil.IsKeyJustPressed(ebiten.KeyF5):
		s.dispatchCommand("emu.start")
	case inpututil.IsKeyJustPressed(ebiten.KeyF6):
		s.dispatchCommand("emu.pause")
	case inpututil.IsKeyJustPressed(ebiten.KeyF7):
		s.dispatchCommand("emu.frame")
	case inpututil.IsKeyJustPressed(ebiten.KeyF8):
		s.dispatchCommand("emu.stop")
	case inpututil.IsKeyJustPressed(ebiten.KeyF9):
		s.dispatchCommand("emu.load_state")
	case inpututil.IsKeyJustPressed(ebiten.KeyF10):
		s.dispatchCommand("emu.save_state")
	case inpututil.IsKeyJustPressed(ebiten.KeyF11):
		s.dispatchCommand("view.fullscreen")
	}
}

func (s *Shell) handleMouse() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	x, y := ebiten.CursorPosition()
	s.handlePointerPress(x, y)
}

func (s *Shell) handlePointerPress(x, y int) {
	if s.panel != nil {
		if x >= 760 && x <= 850 && y >= 612 && y <= 648 {
			s.panel = nil
		}
		return
	}
	if platformUsesTouchLayout() {
		for index, button := range touchNavigationButtons() {
			if pointInRect(x, y, button.Bounds) {
				if s.activeMenu == index {
					s.activeMenu = -1
				} else {
					s.activeMenu = index
				}
				return
			}
		}
	}
	if y < menuHeight {
		offset := 0
		for index, width := range menuWidths(s.menus) {
			if x >= offset && x < offset+width {
				if s.activeMenu == index {
					s.activeMenu = -1
				} else {
					s.activeMenu = index
				}
				return
			}
			offset += width
		}
		s.activeMenu = -1
		return
	}
	if s.activeMenu < 0 {
		return
	}
	startX := menuStartX(s.menus, s.activeMenu)
	if x < startX || x >= startX+dropdownWidth || y < menuHeight {
		s.activeMenu = -1
		return
	}
	index := (y - menuHeight) / effectiveMenuItemHeight()
	commands := s.menus[s.activeMenu].Commands
	if index < 0 || index >= len(commands) {
		s.activeMenu = -1
		return
	}
	commandID := commands[index].ID
	s.activeMenu = -1
	s.dispatchCommand(commandID)
}

func effectiveMenuItemHeight() int {
	if platformUsesTouchLayout() {
		return 44
	}
	return menuItemHeight
}

func (s *Shell) dispatchCommand(id string) {
	command, found := s.findCommand(id)
	if !found {
		s.setStatus(s.trf("Unknown command: %s", id))
		return
	}
	availability := command.Availability(s)
	if !availability.Supported {
		s.setStatus(command.DisplayLabel(s) + ": " + availability.Reason)
		return
	}
	if command.Action != nil {
		command.Action(s)
		return
	}
	if command.Backend != "" {
		s.executeBackend(command.Backend)
	}
}

func (s *Shell) findCommand(id string) (Command, bool) {
	for _, menu := range s.menus {
		for _, command := range menu.Commands {
			if command.ID == id {
				return command, true
			}
		}
	}
	return Command{}, false
}

func (s *Shell) backendCommandLabel(command BackendCommand) string {
	for _, menu := range s.menus {
		for _, item := range menu.Commands {
			if item.Backend == command {
				return item.DisplayLabel(s)
			}
		}
	}
	return s.tr(settingValueLabel(string(command)))
}

func (s *Shell) consumeResults() {
	for {
		select {
		case request := <-s.externalOpen:
			s.openRequest(request)
		case commandID := <-s.externalCommands:
			s.dispatchCommand(commandID)
		case <-s.externalSelectionCanceled:
			s.dialogOpen = false
			s.state = s.preDialogState
			s.setStatus(s.tr("Selection canceled"))
		case active := <-s.hostLifecycle:
			s.hostActive = active
		case stage := <-s.openStageResults:
			switch stage {
			case OpenStageInspecting:
				s.state = FrontendInspecting
				s.setStatus(s.tr("Inspecting selected input..."))
			case OpenStageLoading:
				s.state = FrontendLoading
				s.setStatus(s.tr("Loading selected input..."))
			}
		case result := <-s.pickerResults:
			s.consumePickerResult(result)
		case result := <-s.backendResults:
			s.consumeBackendResult(result)
		case result := <-s.commandResults:
			delete(s.busyCommands, result.command)
			if result.err != nil {
				s.state = frontendStateForError(result.err)
				s.setStatus(s.trf(
					"%s: %s",
					s.backendCommandLabel(result.command),
					result.err.Error(),
				))
				continue
			}
			s.state = s.stableState()
			s.setStatus(s.trf(
				"%s: complete",
				s.backendCommandLabel(result.command),
			))
		case result := <-s.frameRunResults:
			if result.generation != s.frameGeneration {
				continue
			}
			s.frameRunPending = false
			if result.err != nil {
				s.state = frontendStateForError(result.err)
				s.problem = &FrontendProblem{
					State:       s.state,
					Input:       displayNameForInfo(s.input),
					Backend:     s.backendName(),
					Reason:      result.err.Error(),
					Recoverable: true,
				}
				s.setStatus(s.tr("Run frame: ") + result.err.Error())
			}
		case result := <-s.dropResults:
			if result.err != nil {
				s.setStatus(s.tr("Drop: ") + result.err.Error())
				continue
			}
			s.openRequest(OpenRequest{
				Path:        result.path,
				DisplayName: result.displayName,
				Temporary:   true,
			})
		case result := <-s.artifactResults:
			if result.err != nil {
				s.setStatus(s.trf(
					"%s: %s",
					s.tr(settingValueLabel(result.kind)),
					result.err.Error(),
				))
				continue
			}
			if result.warning != "" {
				s.setStatus(s.trf(
					"%s saved with warning: %s (%s)",
					s.tr(settingValueLabel(result.kind)),
					result.path,
					result.warning,
				))
				continue
			}
			s.setStatus(s.trf(
				"%s saved: %s",
				s.tr(settingValueLabel(result.kind)),
				result.path,
			))
		case result := <-s.issueReportResults:
			s.consumeIssueReportResult(result)
		case result := <-s.issueCommentResults:
			s.consumeIssueCommentResult(result)
		case result := <-s.toolResults:
			s.consumeToolResult(result)
		case result := <-s.updateResults:
			s.consumeUpdateResult(result)
		default:
			return
		}
	}
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

func (s *Shell) consumeBackendResult(result backendResult) {
	s.loading = false
	if result.request.Temporary {
		s.temporaryPath = result.request.Path
	}
	if result.info.DisplayName != "" {
		s.input = &result.info
		s.selectedPath = result.request.Path
		if result.request.Path != "" && !result.request.Temporary {
			s.settings.addRecent(result.request.Path)
			_ = s.settings.save()
		}
	}
	if result.err != nil {
		s.state = frontendStateForError(result.err)
		s.problem = &FrontendProblem{
			State:       s.state,
			Input:       displayName(result.request),
			Format:      result.info.Format,
			Profile:     result.info.ProfileID,
			Backend:     s.backendName(),
			Reason:      result.err.Error(),
			Recoverable: true,
		}
		var backendError *BackendError
		if errors.As(result.err, &backendError) && backendError.Backend != "" {
			s.problem.Backend = backendError.Backend
		}
		s.setStatus(fmt.Sprintf("%s: %v", displayName(result.request), result.err))
		return
	}

	s.problem = nil
	s.frameGeneration++
	s.frameRunPending = false
	s.state = s.stableState()
	if s.state == FrontendEmpty {
		s.state = FrontendReady
	}
	s.setStatus(s.trf(
		"Loaded %s | %s | profile %s",
		result.info.DisplayName,
		emptyFallback(result.info.Format, s.tr("unknown")),
		emptyFallback(result.info.ProfileID, s.tr("auto")),
	))
	setPlatformWindowTitle("ARAM - " + result.info.DisplayName)

	backendState := s.backend.State()
	if (backendState == StateReady || backendState == StatePaused) &&
		s.backend.Supports(CommandStart) {
		s.executeBackend(CommandStart)
	}
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

func (s *Shell) closeInput() {
	if err := s.releaseCurrentInput(); err != nil {
		s.setStatus(s.tr("Close: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Title closed"))
}

func (s *Shell) releaseCurrentInput() error {
	if err := s.backend.Close(); err != nil {
		return err
	}
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
	if s.temporaryPath != "" {
		removeTemporaryDrop(s.temporaryPath)
		s.temporaryPath = ""
	}
	s.input = nil
	s.selectedPath = ""
	s.problem = nil
	s.hostPaused = false
	s.frameGeneration++
	s.frameRunPending = false
	s.clearMeasuredSpeed()
	s.frame = VideoFrame{}
	s.frameImage = nil
	s.state = FrontendEmpty
	setPlatformWindowTitle(s.tr("ARAM - Archived Runtime for ARM Mobiles"))
	return nil
}

func (s *Shell) updateAudio() {
	if s.loading || s.input == nil || s.frameRunPending {
		return
	}
	backend, ok := s.backend.(AudioStreamBackend)
	if !ok {
		return
	}
	chunk := backend.DrainAudio()
	if len(chunk.PCM16) == 0 {
		if s.audioOutput != nil {
			s.audioOutput.startIfReady(time.Now())
		}
		return
	}
	if s.audioOutput == nil {
		output, err := newAudioOutput(s.currentAudioSettings())
		if err != nil {
			s.appendLog(s.tr("Audio output: ") + err.Error())
			return
		}
		s.audioOutput = output
	}
	if err := s.audioOutput.enqueue(chunk); err != nil {
		s.appendLog(s.tr("Audio stream: ") + err.Error())
	}
	s.audioOutput.startIfReady(time.Now())
}

func (s *Shell) closeAudio() error {
	if s.audioOutput == nil {
		return nil
	}
	err := s.audioOutput.close()
	s.audioOutput = nil
	return err
}

func (s *Shell) syncBackendState() {
	if s.loading || s.dialogOpen || s.problem != nil || s.input == nil {
		return
	}
	state := frontendStateForBackend(s.backend.State())
	if state == FrontendEmpty {
		state = FrontendReady
	}
	s.state = state
}

func (s *Shell) syncHostLifecycle() {
	state := s.backend.State()
	if !s.hostActive &&
		!s.hostPaused &&
		state == StateRunning &&
		!s.busyCommands[CommandPauseResume] {
		s.hostPaused = true
		s.executeBackend(CommandPauseResume)
		return
	}
	if s.hostActive &&
		s.hostPaused &&
		state == StatePaused &&
		!s.busyCommands[CommandPauseResume] {
		s.hostPaused = false
		s.executeBackend(CommandPauseResume)
	}
}

func (s *Shell) stableState() FrontendState {
	if s.input == nil {
		return FrontendEmpty
	}
	state := frontendStateForBackend(s.backend.State())
	if state == FrontendEmpty {
		return FrontendReady
	}
	return state
}

func (s *Shell) currentFrame() VideoFrame {
	return s.frame
}

func (s *Shell) updateVideo() {
	backend, ok := s.backend.(VideoBackend)
	if !ok {
		return
	}
	frame := backend.VideoFrame()
	if frame.Image == nil {
		return
	}
	bounds := frame.Image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	if s.frameImage != nil && frame.Sequence == s.frame.Sequence {
		return
	}
	s.frame = frame
	s.frameImage = ebiten.NewImageFromImage(frame.Image)
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

func (s *Shell) toggleFullscreen() {
	s.setStatus(s.tr(togglePlatformFullscreen()))
}

func (s *Shell) toggleIntegerScaling() {
	s.settings.IntegerScaling = !s.settings.IntegerScaling
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Integer scaling: %s",
		s.tr(onOff(s.settings.IntegerScaling)),
	))
}

func (s *Shell) toggleAspectRatio() {
	s.settings.PreserveAspect = !s.settings.PreserveAspect
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Preserve aspect ratio: %s",
		s.tr(onOff(s.settings.PreserveAspect)),
	))
}

func (s *Shell) fitWindow() {
	s.setStatus(s.tr(fitPlatformWindow()))
}

func (s *Shell) cycleRotation() {
	s.settings.Rotation = (s.settings.Rotation + 90) % 360
	_ = s.settings.save()
	s.setStatus(s.trf("Rotation: %d°", s.settings.Rotation))
}

func (s *Shell) cycleScreenLayout() {
	if s.settings.ScreenLayout == "center" {
		s.settings.ScreenLayout = "stretch"
	} else {
		s.settings.ScreenLayout = "center"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Screen layout: %s",
		s.tr(settingValueLabel(s.settings.ScreenLayout)),
	))
}

func (s *Shell) cycleFilter() {
	if s.settings.Filter == "nearest" {
		s.settings.Filter = "linear"
	} else {
		s.settings.Filter = "nearest"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Filter: %s",
		s.tr(settingValueLabel(s.settings.Filter)),
	))
}

func (s *Shell) cycleStateSlot() {
	s.setStateSlot((s.settings.StateSlot + 1) % 10)
}

func (s *Shell) setStateSlot(slot int) {
	if slot < 0 {
		slot = 0
	} else if slot > 9 {
		slot = 9
	}
	s.settings.StateSlot = slot
	_ = s.settings.save()
	s.setStatus(s.trf("State slot: %d", s.settings.StateSlot))
}

var speedPresets = []float64{0.5, 1, 2, 4}

// speedPresetIndex returns the preset closest to speed, so values saved by
// older builds still land on a valid slider position.
func speedPresetIndex(speed float64) int {
	best := 0
	for index, preset := range speedPresets {
		if math.Abs(preset-speed) < math.Abs(speedPresets[best]-speed) {
			best = index
		}
	}
	return best
}

func (s *Shell) cycleSpeed() {
	s.setSpeed(speedPresets[(speedPresetIndex(s.settings.Speed)+1)%len(speedPresets)])
}

func (s *Shell) setSpeed(speed float64) {
	s.settings.Speed = speed
	_ = s.settings.save()
	s.setStatus(s.trf("Emulation speed: %gx", s.settings.Speed))
}

func (s *Shell) showAbout() {
	s.panel = &Panel{
		Kind:  "about",
		Title: "About ARAM",
		Lines: []string{
			"ARAM - Archived Runtime for ARM Mobiles",
			"",
			"Cross-platform frontend for Korean feature-phone emulation.",
			s.trf("Version: %s", currentApplicationVersion()),
			s.trf(
				"Frontend state: %s",
				s.tr(stateValueLabel(string(s.state))),
			),
			s.trf("Backend: %s", s.backendName()),
		},
	}
}

func (s *Shell) openDocumentation() {
	if err := openPlatformURL("https://github.com/mirusu400/aram-emu/tree/main/docs"); err != nil {
		s.setStatus(s.tr("Documentation: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Opened ARAM documentation"))
}

func (s *Shell) backendName() string {
	if backend, ok := s.backend.(BackendNamer); ok {
		return backend.BackendName()
	}
	return fmt.Sprintf("%T", s.backend)
}

func (s *Shell) setStatus(message string) {
	s.status = message
	s.appendLog(message)
}

func (s *Shell) appendLog(message string) {
	entry := time.Now().Format("15:04:05") + "  " + strings.TrimSpace(message)
	s.logs = append(s.logs, entry)
	if len(s.logs) > 250 {
		s.logs = append([]string(nil), s.logs[len(s.logs)-250:]...)
	}
}

func menuWidths(menus []Menu) []int {
	widths := make([]int, len(menus))
	for index, menu := range menus {
		width := len(menu.Label)*7 + 22
		if width < 58 {
			width = 58
		}
		widths[index] = width
	}
	return widths
}

func menuStartX(menus []Menu, index int) int {
	offset := 0
	widths := menuWidths(menus)
	for current := 0; current < index; current++ {
		offset += widths[current]
	}
	return offset
}

func displayName(request OpenRequest) string {
	if request.DisplayName != "" {
		return request.DisplayName
	}
	if request.Path != "" {
		return filepath.Base(request.Path)
	}
	return "document"
}

func displayNameForInfo(info *InputInfo) string {
	if info == nil {
		return ""
	}
	return info.DisplayName
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func shorten(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
