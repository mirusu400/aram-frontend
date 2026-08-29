package frontend

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"sync/atomic"
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
	generation      uint64
	completedQuanta int
	guestAdvanced   time.Duration
	startedAt       time.Time
	completedAt     time.Time
	err             error
}

// frameRunRequest hands a batch of guest quanta to the persistent low-priority
// frame worker. Running the guest on its own de-prioritised OS thread keeps a
// heavy title from starving the interface of CPU.
type frameRunRequest struct {
	backend    FrameBackend
	owed       int
	quantum    time.Duration
	generation uint64
	startedAt  time.Time
	uiPriority bool
}

type Shell struct {
	backend              Backend
	picker               Picker
	design               *ARAMDesignSystem
	interfaceUI          *shellUI
	menus                []Menu
	settings             Settings
	customFontData       []byte
	state                FrontendState
	lastRunState         FrontendState
	problem              *FrontendProblem
	activeMenu           int
	focusMode            bool
	touchChromeHidden    bool
	touchChromeSyncState FrontendState
	fillGuestViewport    bool
	uiPointerSuppressed  bool
	status               string
	input                *InputInfo
	selectedPath         string
	temporaryPath        string
	dialogOpen           bool
	loading              bool
	quitting             bool
	hostActive           bool
	hostPaused           bool
	// hostActiveRequest is the newest activity the native host reported.
	// Lifecycle is state, not a stream of events: ebitenmobile suspends the
	// game loop while the app is in the background, so nothing drains a queue
	// there. A queue also filled up - a lifecycle pause and its audio focus
	// loss are two events - and silently dropped the resume that followed,
	// which left the shell convinced the app was still backgrounded. Every
	// title then paused the moment it started, and the document picker put
	// the app in the background before every single open.
	hostActiveRequest         atomic.Bool
	preDialogState            FrontendState
	panel                     *Panel
	settingsSection           string
	layoutWidth               int
	layoutHeight              int
	logs                      []string
	frame                     VideoFrame
	frameImage                *ebiten.Image
	frameScratch              *image.RGBA
	latestVideoGuestNS        atomic.Int64
	latestVideoGeneration     atomic.Uint64
	audioOutput               *audioOutput
	audioMu                   sync.Mutex
	audioPumpStarted          bool
	audioSuspended            bool
	cpuProfile                cpuProfileState
	controlState              map[string]bool
	directionPressOrder       []string
	battery                   batteryReading
	batteryPolledAt           time.Time
	bindingCapture            *bindingCapture
	gamepadMappingsLoaded     bool
	hapticActive              bool
	touchControls             map[ebiten.TouchID]string
	touchLayoutEditing        bool
	touchLayoutDraft          map[string]TouchPlacement
	touchHiddenDraft          map[string]bool
	touchDeckRatioDraft       int
	touchScaleDraft           int
	touchGridStepDraft        int
	touchLayoutDrag           map[ebiten.TouchID]string
	touchLayoutDragOffset     map[ebiten.TouchID]image.Point
	touchLayoutDragPoint      map[ebiten.TouchID]image.Point
	busyCommands              map[BackendCommand]bool
	frameRunPending           bool
	frameGeneration           uint64
	frameWorkerOnce           sync.Once
	frameAccumulator          time.Duration
	lastFramePacingAt         time.Time
	pacingGuestAdvanced       time.Duration
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
	frameRunRequests          chan frameRunRequest
	openStageResults          chan OpenStage
	externalOpen              chan OpenRequest
	externalCommands          chan string
	externalSelectionCanceled chan struct{}
	dropResults               chan dropResult
	artifactResults           chan artifactResult
	saveRestoreResults        chan saveRestoreResult
	issueReportResults        chan issueReportResult
	issueCommentResults       chan issueCommentResult
	toolResults               chan toolResult
	updateResults             chan updateResult
	updateCheckResults        chan updateCheckResult
	updateNoticeReady         bool
	updateNoticeVersion       string
	updateNoticeChannel       updateChannel
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
		frameRunRequests:          make(chan frameRunRequest, 1),
		openStageResults:          make(chan OpenStage, 4),
		externalOpen:              make(chan OpenRequest, 2),
		externalCommands:          make(chan string, 4),
		externalSelectionCanceled: make(chan struct{}, 1),
		dropResults:               make(chan dropResult, 2),
		artifactResults:           make(chan artifactResult, 4),
		saveRestoreResults:        make(chan saveRestoreResult, 2),
		issueReportResults:        make(chan issueReportResult, 2),
		issueCommentResults:       make(chan issueCommentResult, 2),
		toolResults:               make(chan toolResult, 2),
		updateResults:             make(chan updateResult, 4),
		updateCheckResults:        make(chan updateCheckResult, 1),
	}
	shell.hostActiveRequest.Store(true)
	if shell.settings.CPUProfile {
		shell.settings.CPUProfile = shell.setCPUProfiling(true)
	}
	setPlatformWindowTitle(shell.tr("ARAM - Archived Runtime for ARM Mobiles"))
	if shell.shouldOpenWelcome() {
		shell.openWelcome()
	}
	shell.menus = defaultMenus()
	shell.design = newARAMDesignSystem(shell.settings.ThemeMode, shell.settings.ThemeFamily)
	shell.interfaceUI = newShellUI(shell, shell.design)
	if audio, ok := shell.backend.(AudioBackend); ok {
		if err := audio.ConfigureAudio(shell.currentAudioSettings()); err != nil {
			shell.appendLog(shell.tr("Audio settings: ") + err.Error())
		}
	}
	shell.loadCustomFontAtStartup()
	if font, ok := shell.backend.(FontBackend); ok {
		if err := font.ConfigureFont(shell.currentFontSettings()); err != nil {
			shell.appendLog(shell.tr("Handset font: ") + err.Error())
		}
	}
	if selector, ok := shell.backend.(CPUBackendSelector); ok {
		if err := selector.ConfigureCPU(shell.currentCPUSettings()); err != nil {
			shell.appendLog(shell.tr("CPU core: ") + err.Error())
		}
	}
	if applied, err := loadCustomGamepadMappings(); err != nil {
		shell.appendLog(shell.tr("Controller database: ") + err.Error())
	} else if applied {
		shell.gamepadMappingsLoaded = true
		shell.appendLog(shell.tr("Custom controller database loaded"))
	}
	shell.appendLog(shell.status)
	shell.startUpdateCheck()
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

// OpenExternalBytes is the entry point for a host that hands the shell input
// bytes instead of a filesystem path - the web/wasm picker, which reads a
// browser File into memory. It mirrors OpenExternalDocument but carries the
// data in-band (OpenRequest.Data) so a backend can load without a filesystem.
func (s *Shell) OpenExternalBytes(displayName string, data []byte, firmware bool) {
	request := OpenRequest{DisplayName: displayName, Data: data, Firmware: firmware}
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
	s.hostActiveRequest.Store(active)
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
	s.syncTouchChrome()
	s.syncHostLifecycle()
	s.updateVideo()
	s.updateAudio()
	s.updateHaptics()
	s.handleDroppedFiles()
	if !s.handleBindingCapture() {
		s.handleShortcuts()
	}
	s.handleTouch()
	s.syncDesignSystem()
	s.syncUIPointerSuppression()
	if s.focusModeActive() || s.touchLayoutEditing ||
		s.touchChromeHiddenActive() || s.uiPointerSuppressed {
		// The chrome is hidden or a chrome toggle owns the current touch,
		// so the interface UI must not consume input.
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
	if s.design != nil && s.design.Mode == s.settings.ThemeMode &&
		s.design.Family == s.settings.ThemeFamily {
		return
	}
	s.design = newARAMDesignSystem(s.settings.ThemeMode, s.settings.ThemeFamily)
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
	if s.touchLayoutEditing {
		s.drawTouchLayoutEditor(screen)
		return
	}
	if s.touchChromeHiddenActive() {
		s.drawImmersiveWorkspace(screen)
		s.drawTouchControls(screen)
		s.drawTouchChromeToggle(screen)
		return
	}
	s.drawWorkspace(screen)
	s.drawTouchControls(screen)
	s.drawVirtualKeypad(screen)
	if s.interfaceUI != nil {
		s.interfaceUI.ui.Draw(screen)
	}
	s.drawTouchChromeToggle(screen)
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

	if s.touchLayoutEditing {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			s.cancelTouchLayoutEdit()
		}
		return
	}
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
