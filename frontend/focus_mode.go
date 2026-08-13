package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Focus mode strips the shell chrome on touch layouts so the guest screen and
// the D-pad/number controls are the only surfaces. The floating EXIT button is
// the single way back to the full interface, so it must never be covered.

func (s *Shell) focusModeActive() bool {
	return s.focusMode && platformUsesTouchLayout()
}

func (s *Shell) toggleFocusMode() {
	s.focusMode = !s.focusMode
	s.activeMenu = -1
}

func focusDeckHeight(width, height int) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	return min(300, max(174, height*32/100))
}

func focusViewportFor(width, height int) image.Rectangle {
	deckTop := height - focusDeckHeight(width, height)
	return image.Rect(8, 8, width-8, deckTop-4)
}

func focusExitBoundsFor(width, _ int) image.Rectangle {
	return rectAt(width-12-76, 12, 76, 36)
}

func focusControlButtonsFor(width, height int) []touchButton {
	deckHeight := focusDeckHeight(width, height)
	deckTop := height - deckHeight
	margin := max(12, min(28, width/32))
	gap := max(6, min(12, width/96))

	pad := min((deckHeight-margin*2-gap*2)/3, (width/2-margin*2-gap*2)/3)
	pad = max(32, min(72, pad))
	dpadLeft := margin
	dpadTop := deckTop + max(margin, (deckHeight-pad*3-gap*2)/2)
	buttons := []touchButton{
		{Control: "up", Label: "UP", Bounds: rectAt(dpadLeft+pad+gap, dpadTop, pad, pad)},
		{Control: "left", Label: "LEFT", Bounds: rectAt(dpadLeft, dpadTop+pad+gap, pad, pad)},
		{Control: "ok", Label: "OK", Bounds: rectAt(dpadLeft+pad+gap, dpadTop+pad+gap, pad, pad)},
		{Control: "right", Label: "RIGHT", Bounds: rectAt(dpadLeft+pad*2+gap*2, dpadTop+pad+gap, pad, pad)},
		{Control: "down", Label: "DOWN", Bounds: rectAt(dpadLeft+pad+gap, dpadTop+pad*2+gap*2, pad, pad)},
	}

	keyWidth := (min(width/2-margin-gap, 320) - gap*2) / 3
	keyWidth = max(40, min(96, keyWidth))
	keyHeight := (deckHeight - margin*2 - gap*3) / 4
	keyHeight = max(28, min(64, keyHeight))
	gridLeft := width - margin - keyWidth*3 - gap*2
	gridTop := deckTop + max(margin, (deckHeight-keyHeight*4-gap*3)/2)
	layout := [4][3]touchButton{
		{{Control: "num1", Label: "1"}, {Control: "num2", Label: "2"}, {Control: "num3", Label: "3"}},
		{{Control: "num4", Label: "4"}, {Control: "num5", Label: "5"}, {Control: "num6", Label: "6"}},
		{{Control: "num7", Label: "7"}, {Control: "num8", Label: "8"}, {Control: "num9", Label: "9"}},
		{{Control: "star", Label: "*"}, {Control: "num0", Label: "0"}, {Control: "hash", Label: "#"}},
	}
	for row := range layout {
		for column := range layout[row] {
			button := layout[row][column]
			button.Bounds = rectAt(
				gridLeft+column*(keyWidth+gap),
				gridTop+row*(keyHeight+gap),
				keyWidth,
				keyHeight,
			)
			buttons = append(buttons, button)
		}
	}
	return buttons
}

func focusControlAtSize(x, y, width, height int) (string, bool) {
	for _, button := range focusControlButtonsFor(width, height) {
		if pointInRect(x, y, button.Bounds) {
			return button.Control, true
		}
	}
	return "", false
}

func (s *Shell) drawFocusMode(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	viewport := focusViewportFor(width, height)
	if viewport.Dx() >= 32 && viewport.Dy() >= 32 {
		s.drawGuestViewport(screen, viewport)
	}
	active := make(map[string]bool)
	for _, control := range s.touchControls {
		active[control] = true
	}
	for _, button := range focusControlButtonsFor(width, height) {
		s.drawTouchButton(screen, button, active[button.Control])
	}
	s.drawTouchButton(screen, touchButton{
		Label:  "EXIT",
		Bounds: focusExitBoundsFor(width, height),
	}, false)
}
