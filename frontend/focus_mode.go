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
	return focusDeckHeightAtScale(width, height, 1)
}

func focusDeckHeightAtScale(width, height int, renderScale float64) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	return min(scaledPixels(300, renderScale), max(scaledPixels(174, renderScale), height*32/100))
}

func focusViewportFor(width, height int) image.Rectangle {
	return focusViewportForScale(width, height, 1)
}

func focusViewportForScale(width, height int, scale float64) image.Rectangle {
	deckTop := height - focusDeckHeightAtScale(width, height, scale)
	return image.Rect(scaledPixels(8, scale), scaledPixels(8, scale), width-scaledPixels(8, scale), deckTop-scaledPixels(4, scale))
}

func focusExitBoundsFor(width, _ int) image.Rectangle {
	return focusExitBoundsForScale(width, 0, 1)
}

func focusExitBoundsForScale(width, _ int, scale float64) image.Rectangle {
	return rectAt(width-scaledPixels(12+76, scale), scaledPixels(12, scale), scaledPixels(76, scale), scaledPixels(36, scale))
}

func focusControlButtonsFor(width, height int) []touchButton {
	return focusControlButtonsScaled(width, height, 1)
}

func focusControlButtonsScaled(width, height int, scale float64) []touchButton {
	return focusControlButtonsAtScale(width, height, scale, 1)
}

func focusControlButtonsAtScale(width, height int, scale, renderScale float64) []touchButton {
	if scale <= 0 {
		scale = 1
	}
	px := func(value int) int { return scaledPixels(value, renderScale) }
	deckHeight := focusDeckHeightAtScale(width, height, renderScale)
	deckTop := height - deckHeight
	margin := max(px(12), min(px(28), width/32))
	gap := max(px(6), min(px(12), width/96))

	// Scale grows a control toward its geometric space limit; the limit
	// itself keeps the D-pad and the number grid from colliding.
	padLimit := min((deckHeight-margin*2-gap*2)/3, (width/2-margin*2-gap*2)/3)
	pad := clampInt(
		int(float64(max(px(32), min(px(72), padLimit)))*scale+0.5),
		px(32),
		max(px(32), padLimit),
	)
	dpadLeft := margin
	dpadTop := deckTop + max(margin, (deckHeight-pad*3-gap*2)/2)
	buttons := []touchButton{
		{Control: "up", Label: "UP", Bounds: rectAt(dpadLeft+pad+gap, dpadTop, pad, pad)},
		{Control: "left", Label: "LEFT", Bounds: rectAt(dpadLeft, dpadTop+pad+gap, pad, pad)},
		{Control: "ok", Label: "OK", Bounds: rectAt(dpadLeft+pad+gap, dpadTop+pad+gap, pad, pad)},
		{Control: "right", Label: "RIGHT", Bounds: rectAt(dpadLeft+pad*2+gap*2, dpadTop+pad+gap, pad, pad)},
		{Control: "down", Label: "DOWN", Bounds: rectAt(dpadLeft+pad+gap, dpadTop+pad*2+gap*2, pad, pad)},
	}

	keyWidthLimit := (min(width/2-margin-gap, px(320)) - gap*2) / 3
	keyWidth := clampInt(
		int(float64(max(px(40), min(px(96), keyWidthLimit)))*scale+0.5),
		px(40),
		max(px(40), keyWidthLimit),
	)
	keyHeightLimit := (deckHeight - margin*2 - gap*3) / 4
	keyHeight := clampInt(
		int(float64(max(px(28), min(px(64), keyHeightLimit)))*scale+0.5),
		px(28),
		max(px(28), keyHeightLimit),
	)
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
	return focusControlAtScaled(x, y, width, height, 1)
}

func focusControlAtScaled(x, y, width, height int, scale float64) (string, bool) {
	return focusControlAtRenderScale(x, y, width, height, scale, 1)
}

func focusControlAtRenderScale(x, y, width, height int, scale, renderScale float64) (string, bool) {
	for _, button := range focusControlButtonsAtScale(width, height, scale, renderScale) {
		if pointInRect(x, y, button.Bounds) {
			return button.Control, true
		}
	}
	return "", false
}

func (s *Shell) drawFocusMode(screen *ebiten.Image) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	viewport := focusViewportForScale(width, height, s.renderScale)
	if viewport.Dx() >= s.px(32) && viewport.Dy() >= s.px(32) {
		s.drawFilledGuestViewport(screen, viewport)
	}
	active := make(map[string]bool)
	for _, control := range s.touchControls {
		active[control] = true
	}
	for _, button := range focusControlButtonsAtScale(width, height, s.touchScale(), s.renderScale) {
		s.drawTouchButton(screen, button, active[button.Control])
	}
	s.drawTouchButton(screen, touchButton{
		Label:  "EXIT",
		Bounds: focusExitBoundsForScale(width, height, s.renderScale),
	}, false)
}
