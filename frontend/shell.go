package frontend

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	logicalWidth   = 960
	logicalHeight  = 720
	menuHeight     = 28
	statusHeight   = 24
	menuItemHeight = 26
	dropdownWidth  = 290
)

var (
	backgroundColor = color.RGBA{R: 0x18, G: 0x1b, B: 0x20, A: 0xff}
	menuColor       = color.RGBA{R: 0x27, G: 0x2b, B: 0x33, A: 0xff}
	menuActiveColor = color.RGBA{R: 0x3a, G: 0x61, B: 0x8f, A: 0xff}
	panelColor      = color.RGBA{R: 0x20, G: 0x24, B: 0x2b, A: 0xff}
	borderColor     = color.RGBA{R: 0x51, G: 0x58, B: 0x65, A: 0xff}
	disabledColor   = color.RGBA{R: 0x52, G: 0x56, B: 0x5f, A: 0xff}
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
	backend        Backend
	picker         Picker
	menus          []Menu
	settings       Settings
	activeMenu     int
	status         string
	input          *InputInfo
	selectedPath   string
	dialogOpen     bool
	loading        bool
	quitting       bool
	pickerResults  chan pickerResult
	backendResults chan backendResult
	commandResults chan commandResult
	externalOpen   chan OpenRequest
}

func NewShell(backend Backend, picker Picker, initialPath string) *Shell {
	if backend == nil {
		backend = NullBackend{}
	}
	if picker == nil {
		picker = NewPlatformPicker()
	}
	shell := &Shell{
		backend:        backend,
		picker:         picker,
		settings:       loadSettings(),
		activeMenu:     -1,
		status:         "Ready — use File > Open File...",
		pickerResults:  make(chan pickerResult, 2),
		backendResults: make(chan backendResult, 2),
		commandResults: make(chan commandResult, 4),
		externalOpen:   make(chan OpenRequest, 2),
	}
	shell.menus = defaultMenus()
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

func (s *Shell) Update() error {
	if s.quitting {
		return ebiten.Termination
	}
	s.consumeResults()
	s.handleShortcuts()
	s.handleMouse()
	return nil
}

func (s *Shell) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)
	s.drawMenuBar(screen)
	s.drawWorkspace(screen)
	s.drawStatusBar(screen)
	if s.activeMenu >= 0 {
		s.drawDropdown(screen, s.activeMenu)
	}
}

func (s *Shell) Layout(int, int) (int, int) {
	return logicalWidth, logicalHeight
}

func (s *Shell) handleShortcuts() {
	control := ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight)
	if control && inpututil.IsKeyJustPressed(ebiten.KeyO) {
		s.chooseFile()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		s.toggleFullscreen()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.activeMenu = -1
	}
}

func (s *Shell) handleMouse() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	x, y := ebiten.CursorPosition()
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
	index := (y - menuHeight) / menuItemHeight
	commands := s.menus[s.activeMenu].Commands
	if index < 0 || index >= len(commands) {
		s.activeMenu = -1
		return
	}
	command := commands[index]
	s.activeMenu = -1
	if !command.IsEnabled(s) {
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

func (s *Shell) consumeResults() {
	for {
		select {
		case request := <-s.externalOpen:
			s.openRequest(request)
		case result := <-s.pickerResults:
			s.dialogOpen = false
			if result.err != nil {
				switch {
				case errors.Is(result.err, ErrPickerCanceled):
					s.status = "Selection canceled"
				case errors.Is(result.err, ErrPickerUnavailable):
					s.status = "Use the native mobile document picker"
				default:
					s.status = "File picker: " + result.err.Error()
				}
				continue
			}
			s.openRequest(OpenRequest{
				Path:     result.path,
				Firmware: result.operation == operationFirmware,
			})
		case result := <-s.backendResults:
			s.loading = false
			if result.info.DisplayName != "" {
				s.input = &result.info
				s.selectedPath = result.request.Path
				if result.request.Path != "" {
					s.settings.addRecent(result.request.Path)
					_ = s.settings.save()
				}
			}
			if result.err != nil {
				s.status = fmt.Sprintf("%s: %v", displayName(result.request), result.err)
				continue
			}
			s.status = fmt.Sprintf(
				"Loaded %s · %s · profile %s",
				result.info.DisplayName,
				result.info.Format,
				emptyFallback(result.info.ProfileID, "auto"),
			)
		case result := <-s.commandResults:
			if result.err != nil {
				s.status = string(result.command) + ": " + result.err.Error()
				continue
			}
			s.status = string(result.command) + ": complete"
		default:
			return
		}
	}
}

func (s *Shell) chooseFile() {
	if s.dialogOpen || s.loading {
		return
	}
	s.dialogOpen = true
	s.status = "Waiting for file selection..."
	go func() {
		path, err := s.picker.OpenFile()
		s.pickerResults <- pickerResult{operation: operationOpen, path: path, err: err}
	}()
}

func (s *Shell) chooseFirmwareDirectory() {
	if s.dialogOpen || s.loading {
		return
	}
	s.dialogOpen = true
	s.status = "Waiting for firmware directory selection..."
	go func() {
		path, err := s.picker.OpenFirmwareDirectory(s.settings.LastFirmwarePath)
		s.pickerResults <- pickerResult{operation: operationFirmware, path: path, err: err}
	}()
}

func (s *Shell) chooseRecent() {
	if s.dialogOpen || len(s.settings.RecentFiles) == 0 {
		return
	}
	s.dialogOpen = true
	recent := append([]string(nil), s.settings.RecentFiles...)
	go func() {
		path, err := s.picker.ChooseRecent(recent)
		s.pickerResults <- pickerResult{operation: operationRecent, path: path, err: err}
	}()
}

func (s *Shell) openRequest(request OpenRequest) {
	if s.loading {
		return
	}
	s.loading = true
	s.status = "Opening " + displayName(request) + "..."
	if request.Firmware && request.Path != "" {
		s.settings.LastFirmwarePath = request.Path
		_ = s.settings.save()
	}
	go func() {
		info, err := s.backend.Open(context.Background(), request)
		s.backendResults <- backendResult{request: request, info: info, err: err}
	}()
}

func (s *Shell) executeBackend(command BackendCommand) {
	go func() {
		err := s.backend.Execute(context.Background(), command)
		s.commandResults <- commandResult{command: command, err: err}
	}()
}

func (s *Shell) closeInput() {
	if err := s.backend.Close(); err != nil {
		s.status = "Close: " + err.Error()
		return
	}
	s.input = nil
	s.selectedPath = ""
	s.status = "Title closed"
	setPlatformWindowTitle("ARAM — Archived Runtime for ARM Mobiles")
}

func (s *Shell) toggleFullscreen() {
	s.status = togglePlatformFullscreen()
}

func (s *Shell) toggleIntegerScaling() {
	s.settings.IntegerScaling = !s.settings.IntegerScaling
	_ = s.settings.save()
	s.status = fmt.Sprintf("Integer scaling: %t", s.settings.IntegerScaling)
}

func (s *Shell) toggleAspectRatio() {
	s.settings.PreserveAspect = !s.settings.PreserveAspect
	_ = s.settings.save()
	s.status = fmt.Sprintf("Preserve aspect ratio: %t", s.settings.PreserveAspect)
}

func (s *Shell) showAbout() {
	s.status = "ARAM — Archived Runtime for ARM Mobiles"
}

func (s *Shell) drawMenuBar(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, logicalWidth, menuHeight, menuColor)
	offset := 0
	widths := menuWidths(s.menus)
	for index, menu := range s.menus {
		width := widths[index]
		if s.activeMenu == index {
			ebitenutil.DrawRect(screen, float64(offset), 0, float64(width), menuHeight, menuActiveColor)
		}
		ebitenutil.DebugPrintAt(screen, menu.Label, offset+12, 8)
		offset += width
	}
}

func (s *Shell) drawDropdown(screen *ebiten.Image, menuIndex int) {
	commands := s.menus[menuIndex].Commands
	x := menuStartX(s.menus, menuIndex)
	height := len(commands) * menuItemHeight
	ebitenutil.DrawRect(screen, float64(x), menuHeight, dropdownWidth, float64(height), menuColor)
	for index, command := range commands {
		y := menuHeight + index*menuItemHeight
		if !command.IsEnabled(s) {
			ebitenutil.DrawRect(screen, float64(x), float64(y), dropdownWidth, menuItemHeight, disabledColor)
		}
		ebitenutil.DebugPrintAt(screen, command.Label, x+12, y+8)
		if command.Shortcut != "" {
			ebitenutil.DebugPrintAt(screen, command.Shortcut, x+210, y+8)
		}
	}
	ebitenutil.DrawRect(screen, float64(x), float64(menuHeight+height-1), dropdownWidth, 1, borderColor)
}

func (s *Shell) drawWorkspace(screen *ebiten.Image) {
	contentTop := menuHeight + 20
	contentBottom := logicalHeight - statusHeight - 20
	viewportAreaWidth := 650
	ebitenutil.DrawRect(screen, 20, float64(contentTop), float64(viewportAreaWidth), float64(contentBottom-contentTop), panelColor)

	scale := 1
	if s.settings.IntegerScaling {
		scale = 2
	}
	phoneWidth, phoneHeight := 240*scale, 320*scale
	if phoneHeight > contentBottom-contentTop-40 {
		phoneWidth, phoneHeight = 240, 320
	}
	phoneX := 20 + (viewportAreaWidth-phoneWidth)/2
	phoneY := contentTop + (contentBottom-contentTop-phoneHeight)/2
	ebitenutil.DrawRect(screen, float64(phoneX-5), float64(phoneY-5), float64(phoneWidth+10), float64(phoneHeight+10), borderColor)
	ebitenutil.DrawRect(screen, float64(phoneX), float64(phoneY), float64(phoneWidth), float64(phoneHeight), color.Black)

	if s.input == nil {
		ebitenutil.DebugPrintAt(screen, "No title loaded\n\nFile > Open File...\nCtrl+O", phoneX+28, phoneY+phoneHeight/2-24)
	} else {
		ebitenutil.DebugPrintAt(screen, "Input selected\nWaiting for emulator core", phoneX+18, phoneY+phoneHeight/2-12)
	}

	panelX := 690
	ebitenutil.DrawRect(screen, float64(panelX), float64(contentTop), 250, float64(contentBottom-contentTop), panelColor)
	ebitenutil.DebugPrintAt(screen, "ARAM", panelX+16, contentTop+18)
	lines := []string{
		"Archived Runtime for ARM Mobiles",
		"",
		"Backend: " + string(s.backend.State()),
		fmt.Sprintf("Integer scale: %t", s.settings.IntegerScaling),
		fmt.Sprintf("Aspect lock: %t", s.settings.PreserveAspect),
	}
	if s.input != nil {
		lines = append(lines,
			"",
			"Selected input:",
			shorten(s.input.DisplayName, 30),
			"Format: "+emptyFallback(s.input.Format, "unknown"),
			"Profile: "+emptyFallback(s.input.ProfileID, "unselected"),
			"Path:",
			shorten(s.selectedPath, 30),
		)
	}
	ebitenutil.DebugPrintAt(screen, strings.Join(lines, "\n"), panelX+16, contentTop+42)
}

func (s *Shell) drawStatusBar(screen *ebiten.Image) {
	y := logicalHeight - statusHeight
	ebitenutil.DrawRect(screen, 0, float64(y), logicalWidth, statusHeight, menuColor)
	ebitenutil.DebugPrintAt(screen, shorten(s.status, 142), 10, y+7)
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
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
