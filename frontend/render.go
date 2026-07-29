package frontend

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

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
	stateText := strings.ToUpper(string(s.state))
	ebitenutil.DebugPrintAt(screen, stateText, logicalWidth-len(stateText)*8-14, 8)
}

func (s *Shell) drawDropdown(screen *ebiten.Image, menuIndex int) {
	commands := s.menus[menuIndex].Commands
	x := menuStartX(s.menus, menuIndex)
	itemHeight := effectiveMenuItemHeight()
	height := len(commands) * itemHeight
	ebitenutil.DrawRect(screen, float64(x), menuHeight, dropdownWidth, float64(height), menuColor)
	for index, command := range commands {
		y := menuHeight + index*itemHeight
		if !command.IsEnabled(s) {
			ebitenutil.DrawRect(screen, float64(x), float64(y), dropdownWidth, float64(itemHeight), disabledColor)
		}
		textY := y + (itemHeight-10)/2
		ebitenutil.DebugPrintAt(screen, shorten(command.DisplayLabel(s), 28), x+12, textY)
		if command.Shortcut != "" {
			ebitenutil.DebugPrintAt(screen, command.Shortcut, x+222, textY)
		}
	}
	ebitenutil.DrawRect(screen, float64(x), float64(menuHeight+height-1), dropdownWidth, 1, borderColor)
}

func (s *Shell) drawWorkspace(screen *ebiten.Image) {
	contentTop := menuHeight + 20
	contentBottom := logicalHeight - statusHeight - 20
	viewportPanel := image.Rect(20, contentTop, 670, contentBottom)
	viewport := image.Rect(40, contentTop+20, 650, contentBottom-20)
	ebitenutil.DrawRect(
		screen,
		float64(viewportPanel.Min.X),
		float64(viewportPanel.Min.Y),
		float64(viewportPanel.Dx()),
		float64(viewportPanel.Dy()),
		panelColor,
	)
	s.drawGuestViewport(screen, viewport)

	panelX := 690
	ebitenutil.DrawRect(screen, float64(panelX), float64(contentTop), 250, float64(contentBottom-contentTop), panelColor)
	ebitenutil.DebugPrintAt(screen, "ARAM", panelX+16, contentTop+18)
	lines := []string{
		"Archived Runtime for ARM Mobiles",
		"",
		"Frontend: " + string(s.state),
		"Backend: " + shorten(s.backendName(), 21),
		"Core state: " + string(s.backend.State()),
		fmt.Sprintf("Scale: integer=%t", s.settings.IntegerScaling),
		fmt.Sprintf("Aspect: locked=%t", s.settings.PreserveAspect),
		fmt.Sprintf("Rotation: %d degrees", s.settings.Rotation),
		"Layout/filter: " + s.settings.ScreenLayout + "/" + s.settings.Filter,
		fmt.Sprintf("State slot/speed: %d / %gx", s.settings.StateSlot, s.settings.Speed),
	}
	if s.input != nil {
		lines = append(lines,
			"",
			"Selected input:",
			shorten(s.input.DisplayName, 27),
			"Format: "+emptyFallback(s.input.Format, "unknown"),
			"Profile: "+emptyFallback(s.input.ProfileID, "unselected"),
			fmt.Sprintf("Size: %d bytes", s.input.Size),
			"SHA-256:",
			shorten(s.input.SHA256, 27),
			"Path:",
			shorten(s.selectedPath, 27),
		)
	}
	if s.problem != nil {
		lines = append(lines,
			"",
			"Actionable error:",
			"State: "+string(s.problem.State),
			"Reason:",
			shorten(s.problem.Reason, 27),
		)
	}
	ebitenutil.DebugPrintAt(screen, strings.Join(lines, "\n"), panelX+16, contentTop+42)
}

func (s *Shell) drawGuestViewport(screen *ebiten.Image, viewport image.Rectangle) {
	ebitenutil.DrawRect(
		screen,
		float64(viewport.Min.X-5),
		float64(viewport.Min.Y-5),
		float64(viewport.Dx()+10),
		float64(viewport.Dy()+10),
		borderColor,
	)
	ebitenutil.DrawRect(
		screen,
		float64(viewport.Min.X),
		float64(viewport.Min.Y),
		float64(viewport.Dx()),
		float64(viewport.Dy()),
		backgroundColor,
	)

	if s.frameImage == nil || s.frame.Image == nil {
		s.drawEmptyViewport(screen, viewport)
		return
	}

	sourceBounds := s.frame.Image.Bounds()
	sourceWidth, sourceHeight := sourceBounds.Dx(), sourceBounds.Dy()
	rotatedWidth, rotatedHeight := sourceWidth, sourceHeight
	if s.settings.Rotation == 90 || s.settings.Rotation == 270 {
		rotatedWidth, rotatedHeight = sourceHeight, sourceWidth
	}
	destination := s.frameDestination(viewport, rotatedWidth, rotatedHeight)
	scaleX := float64(destination.Dx()) / float64(rotatedWidth)
	scaleY := float64(destination.Dy()) / float64(rotatedHeight)

	options := &ebiten.DrawImageOptions{}
	options.GeoM.Translate(float64(-sourceBounds.Min.X), float64(-sourceBounds.Min.Y))
	switch s.settings.Rotation {
	case 90:
		options.GeoM.Rotate(math.Pi / 2)
		options.GeoM.Translate(float64(sourceHeight), 0)
	case 180:
		options.GeoM.Rotate(math.Pi)
		options.GeoM.Translate(float64(sourceWidth), float64(sourceHeight))
	case 270:
		options.GeoM.Rotate(3 * math.Pi / 2)
		options.GeoM.Translate(0, float64(sourceWidth))
	}
	options.GeoM.Scale(scaleX, scaleY)
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	if s.settings.Filter == "linear" {
		options.Filter = ebiten.FilterLinear
	} else {
		options.Filter = ebiten.FilterNearest
	}
	screen.DrawImage(s.frameImage, options)
}

func (s *Shell) frameDestination(viewport image.Rectangle, width, height int) image.Rectangle {
	if width <= 0 || height <= 0 {
		return viewport
	}
	scaleX := float64(viewport.Dx()) / float64(width)
	scaleY := float64(viewport.Dy()) / float64(height)

	if s.settings.ScreenLayout == "stretch" && !s.settings.PreserveAspect {
		return viewport
	}

	scale := math.Min(scaleX, scaleY)
	if s.settings.IntegerScaling && scale >= 1 {
		scale = math.Max(1, math.Floor(scale))
	}
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))
	if !s.settings.PreserveAspect && s.settings.ScreenLayout == "stretch" {
		targetWidth = viewport.Dx()
		targetHeight = viewport.Dy()
	}
	x := viewport.Min.X + (viewport.Dx()-targetWidth)/2
	y := viewport.Min.Y + (viewport.Dy()-targetHeight)/2
	return image.Rect(x, y, x+targetWidth, y+targetHeight)
}

func (s *Shell) drawEmptyViewport(screen *ebiten.Image, viewport image.Rectangle) {
	lines := []string{
		"No guest frame",
		"",
		"File > Open File...",
		"or drag a file here",
	}
	if s.loading {
		lines = []string{"Preparing input...", "", strings.ToUpper(string(s.state))}
	} else if s.problem != nil {
		lines = []string{
			"Unable to start this input",
			"",
			"State: " + string(s.problem.State),
			"Input: " + shorten(s.problem.Input, 40),
			"Format: " + emptyFallback(s.problem.Format, "unknown"),
			"Profile: " + emptyFallback(s.problem.Profile, "unselected"),
			"Backend: " + emptyFallback(s.problem.Backend, "unknown"),
			"",
			shorten(s.problem.Reason, 64),
		}
		ebitenutil.DrawRect(
			screen,
			float64(viewport.Min.X),
			float64(viewport.Min.Y),
			float64(viewport.Dx()),
			4,
			faultColor,
		)
	}
	x := viewport.Min.X + 28
	y := viewport.Min.Y + viewport.Dy()/2 - len(lines)*8
	ebitenutil.DebugPrintAt(screen, strings.Join(lines, "\n"), x, y)
}

func (s *Shell) drawStatusBar(screen *ebiten.Image) {
	y := logicalHeight - statusHeight
	ebitenutil.DrawRect(screen, 0, float64(y), logicalWidth, statusHeight, menuColor)
	ebitenutil.DebugPrintAt(screen, shorten(s.status, 116), 10, y+7)
}
