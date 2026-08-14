package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type touchButton struct {
	Control string
	Label   string
	Bounds  image.Rectangle
}

func touchControlButtons() []touchButton {
	return touchControlButtonsFor(logicalWidth, logicalHeight)
}

func touchControlButtonsFor(width, height int) []touchButton {
	deckHeight := touchDeckHeight(width, height)
	deckTop := height - statusBarHeight - deckHeight
	margin := max(12, min(28, width/32))
	gap := max(6, min(12, width/96))
	buttonSize := max(40, min(62, min(width/12, (deckHeight-64-gap*2)/3)))
	dpadX := margin + buttonSize
	dpadY := deckTop + 40 + buttonSize + gap
	actionX := width - margin - buttonSize*3
	actionY := dpadY

	return []touchButton{
		{Control: "up", Label: "UP", Bounds: rectAt(dpadX, dpadY-buttonSize-gap, buttonSize, buttonSize)},
		{Control: "left", Label: "LEFT", Bounds: rectAt(dpadX-buttonSize-gap, dpadY, buttonSize, buttonSize)},
		{Control: "ok", Label: "OK", Bounds: rectAt(dpadX, dpadY, buttonSize, buttonSize)},
		{Control: "right", Label: "RIGHT", Bounds: rectAt(dpadX+buttonSize+gap, dpadY, buttonSize, buttonSize)},
		{Control: "down", Label: "DOWN", Bounds: rectAt(dpadX, dpadY+buttonSize+gap, buttonSize, buttonSize)},
		{Control: "soft-left", Label: "L", Bounds: rectAt(actionX, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{Control: "soft-right", Label: "R", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{Control: "back", Label: "BACK", Bounds: rectAt(actionX, actionY, buttonSize, buttonSize)},
		{Control: "menu", Label: "MENU", Bounds: rectAt(actionX+buttonSize+gap, actionY, buttonSize, buttonSize)},
		{Control: "ok", Label: "OK", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY, buttonSize, buttonSize)},
	}
}

func touchNavigationButtons() []touchButton {
	return touchNavigationButtonsFor(logicalWidth, logicalHeight)
}

func touchNavigationButtonsFor(width, height int) []touchButton {
	labels := []string{"File", "Emulation", "View", "Tools", "Help"}
	margin := max(12, min(28, width/32))
	gap := max(4, min(10, width/120))
	available := width - margin*2 - gap*(len(labels)-1)
	buttonWidth := max(72, available/len(labels))
	y := height - statusBarHeight - touchDeckHeight(width, height) + 8
	buttons := make([]touchButton, 0, len(labels))
	for index, label := range labels {
		x := margin + index*(buttonWidth+gap)
		buttons = append(buttons, touchButton{
			Label:  label,
			Bounds: rectAt(x, y, buttonWidth, 32),
		})
	}
	return buttons
}

func touchDeckHeight(width, height int) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	return min(230, max(174, height*3/10))
}

func rectAt(x, y, width, height int) image.Rectangle {
	return image.Rect(x, y, x+width, y+height)
}

func touchNavigationAt(x, y int) (int, bool) {
	return touchNavigationAtSize(x, y, logicalWidth, logicalHeight)
}

func touchNavigationAtSize(x, y, width, height int) (int, bool) {
	for index, button := range touchNavigationButtonsFor(width, height) {
		if pointInRect(x, y, button.Bounds) {
			return index, true
		}
	}
	return 0, false
}

func touchControlAt(x, y int) (string, bool) {
	return touchControlAtSize(x, y, logicalWidth, logicalHeight)
}

func touchControlAtSize(x, y, width, height int) (string, bool) {
	for _, button := range touchControlButtonsFor(width, height) {
		if pointInRect(x, y, button.Bounds) {
			return button.Control, true
		}
	}
	return "", false
}

func pointInRect(x, y int, bounds image.Rectangle) bool {
	return image.Pt(x, y).In(bounds)
}

func (s *Shell) drawTouchControls(screen *ebiten.Image) {
	if !platformUsesTouchLayout() {
		return
	}
	active := make(map[string]bool)
	for _, control := range s.touchControls {
		active[control] = true
	}
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	for _, button := range touchControlButtonsFor(width, height) {
		s.drawTouchButton(screen, button, active[button.Control])
	}
	for index, button := range touchNavigationButtonsFor(width, height) {
		s.drawTouchButton(screen, button, s.activeMenu == index)
	}
}

func (s *Shell) drawTouchButton(screen *ebiten.Image, button touchButton, active bool) {
	bounds := button.Bounds
	label := s.tr(button.Label)
	if s.design != nil {
		surface := s.design.Components.TouchButton.Image.Idle
		textColor := s.design.Palette.TextMuted
		if active {
			surface = s.design.Components.TouchButton.Image.Pressed
			textColor = s.design.Palette.Text
		}
		surface.Draw(screen, bounds.Dx(), bounds.Dy(), func(options *ebiten.DrawImageOptions) {
			options.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
		})
		drawCenteredText(
			screen,
			label,
			s.design.Type.Strong,
			textColor,
			bounds,
			bounds.Min.Y+(bounds.Dy()-13)/2,
		)
		return
	}
	palette := defaultARAMPalette()
	ebitenutil.DrawRect(
		screen,
		float64(bounds.Min.X),
		float64(bounds.Min.Y),
		float64(bounds.Dx()),
		float64(bounds.Dy()),
		palette.SurfaceRaised,
	)
	textX := bounds.Min.X + (bounds.Dx()-len([]rune(label))*6)/2
	textY := bounds.Min.Y + (bounds.Dy()-8)/2
	ebitenutil.DebugPrintAt(screen, label, textX, textY)
}
