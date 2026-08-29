package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func (s *Shell) virtualKeypadVisible() bool {
	return s.settings.ShowVirtualKeypad && !platformUsesTouchLayout()
}

// touchKeypadVisible is the same setting on a touch layout, where the keypad
// joins the on-screen control deck instead of taking a rail beside the guest
// display. Without it a handset has no way to reach the numeric keys at all.
func (s *Shell) touchKeypadVisible() bool {
	return s.settings.ShowVirtualKeypad && platformUsesTouchLayout()
}

func virtualKeypadWidthFor(width int) int {
	return min(232, max(188, width/4))
}

func virtualKeypadPanelBoundsFor(width, height int) image.Rectangle {
	panelWidth := virtualKeypadWidthFor(width)
	return image.Rect(
		width-12-panelWidth,
		menuHeight+applicationToolbarHeight+12,
		width-12,
		height-statusHeight-12,
	)
}

func virtualKeypadReservedWidthFor(width int) int {
	return virtualKeypadWidthFor(width) + 12
}

func virtualKeypadButtonsFor(width, height int) []touchButton {
	panel := virtualKeypadPanelBoundsFor(width, height)
	const (
		padding     = 10
		headerSpace = 34
		gap         = 6
		rows        = 9
		columns     = 3
	)
	availableHeight := max(1, panel.Dy()-padding*2-headerSpace)
	buttonHeight := min(52, max(28, (availableHeight-gap*(rows-1))/rows))
	buttonWidth := max(32, (panel.Dx()-padding*2-gap*(columns-1))/columns)
	gridHeight := buttonHeight*rows + gap*(rows-1)
	x := panel.Min.X + padding
	y := panel.Min.Y + headerSpace + max(0, (availableHeight-gridHeight)/2)

	layout := [rows][columns]touchButton{
		{
			{Control: "volume-down", Label: "VOL-"},
			{Control: "menu", Label: "MENU"},
			{Control: "volume-up", Label: "VOL+"},
		},
		{
			{Control: "soft-left", Label: "L"},
			{Control: "up", Label: "UP"},
			{Control: "soft-right", Label: "CANCEL"},
		},
		{
			{Control: "left", Label: "LEFT"},
			{Control: "ok", Label: "OK"},
			{Control: "right", Label: "RIGHT"},
		},
		{
			{Control: "send", Label: "CALL"},
			{Control: "down", Label: "DOWN"},
			{Control: "end", Label: "END"},
		},
		{
			{},
			{Control: "back", Label: "C"},
			{},
		},
		{
			{Control: "num1", Label: "1"},
			{Control: "num2", Label: "2"},
			{Control: "num3", Label: "3"},
		},
		{
			{Control: "num4", Label: "4"},
			{Control: "num5", Label: "5"},
			{Control: "num6", Label: "6"},
		},
		{
			{Control: "num7", Label: "7"},
			{Control: "num8", Label: "8"},
			{Control: "num9", Label: "9"},
		},
		{
			{Control: "star", Label: "*"},
			{Control: "num0", Label: "0"},
			{Control: "hash", Label: "#"},
		},
	}

	buttons := make([]touchButton, 0, rows*columns)
	for row := range layout {
		for column := range layout[row] {
			button := layout[row][column]
			if button.Control == "" {
				continue
			}
			button.Bounds = rectAt(
				x+column*(buttonWidth+gap),
				y+row*(buttonHeight+gap),
				buttonWidth,
				buttonHeight,
			)
			buttons = append(buttons, button)
		}
	}
	return buttons
}

func virtualKeypadControlAtSize(x, y, width, height int) (string, bool) {
	for _, button := range virtualKeypadButtonsFor(width, height) {
		if pointInRect(x, y, button.Bounds) {
			return button.Control, true
		}
	}
	return "", false
}

func (s *Shell) drawVirtualKeypad(screen *ebiten.Image) {
	if !s.virtualKeypadVisible() {
		return
	}
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	panel := virtualKeypadPanelBoundsFor(width, height)
	ebitenutil.DrawRect(
		screen,
		float64(panel.Min.X),
		float64(panel.Min.Y),
		float64(panel.Dx()),
		float64(panel.Dy()),
		palette.Border,
	)
	ebitenutil.DrawRect(
		screen,
		float64(panel.Min.X+1),
		float64(panel.Min.Y+1),
		float64(panel.Dx()-2),
		float64(panel.Dy()-2),
		palette.Surface,
	)
	titleBounds := image.Rect(panel.Min.X, panel.Min.Y+10, panel.Max.X, panel.Min.Y+30)
	if s.design != nil {
		drawCenteredText(
			screen,
			s.tr("VIRTUAL KEYS"),
			s.design.Type.Caption,
			palette.TextMuted,
			titleBounds,
			titleBounds.Min.Y,
		)
	} else {
		ebitenutil.DebugPrintAt(
			screen,
			s.tr("VIRTUAL KEYS"),
			panel.Min.X+12,
			panel.Min.Y+12,
		)
	}

	active := make(map[string]bool, len(s.controlState))
	for control, pressed := range s.controlState {
		active[control] = pressed
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if control, ok := virtualKeypadControlAtSize(x, y, width, height); ok {
			active[control] = true
		}
	}
	for _, button := range virtualKeypadButtonsFor(width, height) {
		s.drawTouchButton(screen, button, active[button.Control])
	}
}
