package frontend

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	logicalWidth   = 960
	logicalHeight  = 720
	menuHeight     = 28
	statusHeight   = 24
	menuItemHeight = 26
	dropdownWidth  = 310
)

var (
	backgroundColor = color.RGBA{R: 0x18, G: 0x1b, B: 0x20, A: 0xff}
	menuColor       = color.RGBA{R: 0x27, G: 0x2b, B: 0x33, A: 0xff}
	menuActiveColor = color.RGBA{R: 0x3a, G: 0x61, B: 0x8f, A: 0xff}
	panelColor      = color.RGBA{R: 0x20, G: 0x24, B: 0x2b, A: 0xff}
	borderColor     = color.RGBA{R: 0x51, G: 0x58, B: 0x65, A: 0xff}
	disabledColor   = color.RGBA{R: 0x52, G: 0x56, B: 0x5f, A: 0xff}
	accentColor     = color.RGBA{R: 0x52, G: 0x86, B: 0xbd, A: 0xff}
	faultColor      = color.RGBA{R: 0x8f, G: 0x3f, B: 0x46, A: 0xff}
	colorOverlay    = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
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

type Shell struct {
	backend          Backend
	picker           Picker
	menus            []Menu
	settings         Settings
	state            FrontendState
	problem          *FrontendProblem
	activeMenu       int
	status           string
	input            *InputInfo
	selectedPath     string
	temporaryPath    string
	dialogOpen       bool
	loading          bool
	quitting         bool
	hostActive       bool
	hostPaused       bool
	preDialogState   FrontendState
	panel            *Panel
	logs             []string
	startedAt        time.Time
	frame            VideoFrame
	frameImage       *ebiten.Image
	controlState     map[string]bool
	touchControls    map[ebiten.TouchID]string
	busyCommands     map[BackendCommand]bool
	pickerResults    chan pickerResult
	backendResults   chan backendResult
	commandResults   chan commandResult
	openStageResults chan OpenStage
	externalOpen     chan OpenRequest
	externalCommands chan string
	hostLifecycle    chan bool
	dropResults      chan dropResult
	artifactResults  chan artifactResult
	toolResults      chan toolResult
}

func NewShell(backend Backend, picker Picker, initialPath string) *Shell {
	if backend == nil {
		backend = NullBackend{}
	}
	if picker == nil {
		picker = NewPlatformPicker()
	}
	shell := &Shell{
		backend:          backend,
		picker:           picker,
		settings:         loadSettings(),
		state:            FrontendEmpty,
		activeMenu:       -1,
		status:           "Ready - use File > Open File...",
		startedAt:        time.Now(),
		hostActive:       true,
		controlState:     make(map[string]bool),
		touchControls:    make(map[ebiten.TouchID]string),
		busyCommands:     make(map[BackendCommand]bool),
		pickerResults:    make(chan pickerResult, 2),
		backendResults:   make(chan backendResult, 2),
		commandResults:   make(chan commandResult, 8),
		openStageResults: make(chan OpenStage, 4),
		externalOpen:     make(chan OpenRequest, 2),
		externalCommands: make(chan string, 4),
		hostLifecycle:    make(chan bool, 2),
		dropResults:      make(chan dropResult, 2),
		artifactResults:  make(chan artifactResult, 4),
		toolResults:      make(chan toolResult, 2),
	}
	shell.menus = defaultMenus()
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

func (s *Shell) Update() error {
	if s.quitting {
		return ebiten.Termination
	}
	s.consumeResults()
	s.syncBackendState()
	s.syncHostLifecycle()
	s.updateVideo()
	s.handleDroppedFiles()
	s.handleShortcuts()
	s.handleTouch()
	s.handleMappedInput()
	s.handleMouse()
	return nil
}

func (s *Shell) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)
	s.drawMenuBar(screen)
	s.drawWorkspace(screen)
	s.drawTouchControls(screen)
	s.drawStatusBar(screen)
	if s.activeMenu >= 0 {
		s.drawDropdown(screen, s.activeMenu)
	}
	if s.panel != nil {
		s.drawPanel(screen)
	}
}

func (s *Shell) Layout(int, int) (int, int) {
	return logicalWidth, logicalHeight
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
		s.setStatus("Unknown command: " + id)
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

func (s *Shell) consumeResults() {
	for {
		select {
		case request := <-s.externalOpen:
			s.openRequest(request)
		case commandID := <-s.externalCommands:
			s.dispatchCommand(commandID)
		case active := <-s.hostLifecycle:
			s.hostActive = active
		case stage := <-s.openStageResults:
			switch stage {
			case OpenStageInspecting:
				s.state = FrontendInspecting
				s.setStatus("Inspecting selected input...")
			case OpenStageLoading:
				s.state = FrontendLoading
				s.setStatus("Loading selected input...")
			}
		case result := <-s.pickerResults:
			s.consumePickerResult(result)
		case result := <-s.backendResults:
			s.consumeBackendResult(result)
		case result := <-s.commandResults:
			delete(s.busyCommands, result.command)
			if result.err != nil {
				s.state = frontendStateForError(result.err)
				s.setStatus(string(result.command) + ": " + result.err.Error())
				continue
			}
			s.state = s.stableState()
			s.setStatus(string(result.command) + ": complete")
		case result := <-s.dropResults:
			if result.err != nil {
				s.setStatus("Drop: " + result.err.Error())
				continue
			}
			s.openRequest(OpenRequest{
				Path:        result.path,
				DisplayName: result.displayName,
				Temporary:   true,
			})
		case result := <-s.artifactResults:
			if result.err != nil {
				s.setStatus(result.kind + ": " + result.err.Error())
				continue
			}
			s.setStatus(result.kind + " saved: " + result.path)
		case result := <-s.toolResults:
			s.consumeToolResult(result)
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
			s.setStatus("Selection canceled")
		case errors.Is(result.err, ErrPickerUnavailable):
			s.setStatus("Use the native mobile document picker")
		default:
			s.setStatus("File picker: " + result.err.Error())
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
	s.state = s.stableState()
	if s.state == FrontendEmpty {
		s.state = FrontendReady
	}
	s.setStatus(fmt.Sprintf(
		"Loaded %s | %s | profile %s",
		result.info.DisplayName,
		emptyFallback(result.info.Format, "unknown"),
		emptyFallback(result.info.ProfileID, "auto"),
	))
	setPlatformWindowTitle("ARAM - " + result.info.DisplayName)
}

func (s *Shell) chooseFile() {
	if s.dialogOpen || s.loading {
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	s.setStatus("Waiting for file selection...")
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
	s.setStatus("Waiting for firmware directory selection...")
	go func() {
		path, err := s.picker.OpenFirmwareDirectory(s.settings.LastFirmwarePath)
		s.pickerResults <- pickerResult{operation: operationFirmware, path: path, err: err}
	}()
}

func (s *Shell) chooseRecent() {
	if s.dialogOpen || len(s.settings.RecentFiles) == 0 {
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	recent := append([]string(nil), s.settings.RecentFiles...)
	s.setStatus("Choose a recent input...")
	go func() {
		path, err := s.picker.ChooseRecent(recent)
		s.pickerResults <- pickerResult{operation: operationRecent, path: path, err: err}
	}()
}

func (s *Shell) openRequest(request OpenRequest) {
	if s.loading {
		s.setStatus("An input is already loading")
		return
	}
	if s.input != nil {
		if err := s.releaseCurrentInput(); err != nil {
			s.setStatus("Close current title: " + err.Error())
			return
		}
	}
	s.loading = true
	s.problem = nil
	s.state = FrontendInspecting
	s.setStatus("Inspecting " + displayName(request) + "...")
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
		s.setStatus(string(command) + ": already in progress")
		return
	}
	s.busyCommands[command] = true
	s.setStatus(string(command) + "...")
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
		s.setStatus("Close: " + err.Error())
		return
	}
	s.setStatus("Title closed")
}

func (s *Shell) releaseCurrentInput() error {
	if err := s.backend.Close(); err != nil {
		return err
	}
	if s.temporaryPath != "" {
		removeTemporaryDrop(s.temporaryPath)
		s.temporaryPath = ""
	}
	s.input = nil
	s.selectedPath = ""
	s.problem = nil
	s.hostPaused = false
	s.frame = VideoFrame{}
	s.frameImage = nil
	s.state = FrontendEmpty
	setPlatformWindowTitle("ARAM - Archived Runtime for ARM Mobiles")
	return nil
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
	s.setStatus("Copying dropped input into the ARAM cache...")
	go copyFirstDroppedFile(files, s.dropResults)
}

func (s *Shell) toggleFullscreen() {
	s.setStatus(togglePlatformFullscreen())
}

func (s *Shell) toggleIntegerScaling() {
	s.settings.IntegerScaling = !s.settings.IntegerScaling
	_ = s.settings.save()
	s.setStatus(fmt.Sprintf("Integer scaling: %t", s.settings.IntegerScaling))
}

func (s *Shell) toggleAspectRatio() {
	s.settings.PreserveAspect = !s.settings.PreserveAspect
	_ = s.settings.save()
	s.setStatus(fmt.Sprintf("Preserve aspect ratio: %t", s.settings.PreserveAspect))
}

func (s *Shell) fitWindow() {
	s.setStatus(fitPlatformWindow())
}

func (s *Shell) cycleRotation() {
	s.settings.Rotation = (s.settings.Rotation + 90) % 360
	_ = s.settings.save()
	s.setStatus(fmt.Sprintf("Rotation: %d degrees", s.settings.Rotation))
}

func (s *Shell) cycleScreenLayout() {
	if s.settings.ScreenLayout == "center" {
		s.settings.ScreenLayout = "stretch"
	} else {
		s.settings.ScreenLayout = "center"
	}
	_ = s.settings.save()
	s.setStatus("Screen layout: " + s.settings.ScreenLayout)
}

func (s *Shell) cycleFilter() {
	if s.settings.Filter == "nearest" {
		s.settings.Filter = "linear"
	} else {
		s.settings.Filter = "nearest"
	}
	_ = s.settings.save()
	s.setStatus("Filter: " + s.settings.Filter)
}

func (s *Shell) cycleStateSlot() {
	s.settings.StateSlot = (s.settings.StateSlot + 1) % 10
	_ = s.settings.save()
	s.setStatus(fmt.Sprintf("State slot: %d", s.settings.StateSlot))
}

func (s *Shell) cycleSpeed() {
	speeds := []float64{0.5, 1, 2, 4}
	for index, speed := range speeds {
		if s.settings.Speed == speed {
			s.settings.Speed = speeds[(index+1)%len(speeds)]
			_ = s.settings.save()
			s.setStatus(fmt.Sprintf("Emulation speed: %gx", s.settings.Speed))
			return
		}
	}
	s.settings.Speed = 1
	_ = s.settings.save()
}

func (s *Shell) showAbout() {
	s.panel = &Panel{
		Kind:  "about",
		Title: "About ARAM",
		Lines: []string{
			"ARAM - Archived Runtime for ARM Mobiles",
			"",
			"Cross-platform frontend for Korean feature-phone emulation.",
			"Frontend state: " + string(s.state),
			"Backend: " + s.backendName(),
		},
	}
}

func (s *Shell) openDocumentation() {
	if err := openPlatformURL("https://github.com/mirusu400/aram-emu/tree/main/docs"); err != nil {
		s.setStatus("Documentation: " + err.Error())
		return
	}
	s.setStatus("Opened ARAM documentation")
}

func (s *Shell) openIssueTracker() {
	if err := openPlatformURL("https://github.com/mirusu400/aram-emu/issues"); err != nil {
		s.setStatus("Issue tracker: " + err.Error())
		return
	}
	s.setStatus("Opened ARAM issue tracker")
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
		width := len(menu.Label)*8 + 28
		if width < 68 {
			width = 68
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
