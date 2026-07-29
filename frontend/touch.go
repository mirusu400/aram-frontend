package frontend

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type touchButton struct {
	Control string
	Label   string
	Bounds  image.Rectangle
}

var (
	touchButtonColor = color.RGBA{R: 0x27, G: 0x2b, B: 0x33, A: 0xc8}
	touchActiveColor = color.RGBA{R: 0x52, G: 0x86, B: 0xbd, A: 0xe8}
)

func touchControlButtons() []touchButton {
	return []touchButton{
		{Control: "up", Label: "UP", Bounds: image.Rect(100, 514, 162, 558)},
		{Control: "left", Label: "LEFT", Bounds: image.Rect(38, 558, 100, 606)},
		{Control: "ok", Label: "OK", Bounds: image.Rect(100, 558, 162, 606)},
		{Control: "right", Label: "RIGHT", Bounds: image.Rect(162, 558, 224, 606)},
		{Control: "down", Label: "DOWN", Bounds: image.Rect(100, 606, 162, 650)},
		{Control: "soft-left", Label: "L", Bounds: image.Rect(432, 520, 492, 564)},
		{Control: "soft-right", Label: "R", Bounds: image.Rect(570, 520, 630, 564)},
		{Control: "back", Label: "BACK", Bounds: image.Rect(432, 576, 512, 626)},
		{Control: "menu", Label: "MENU", Bounds: image.Rect(522, 576, 582, 626)},
		{Control: "ok", Label: "OK", Bounds: image.Rect(592, 576, 652, 626)},
	}
}

func touchNavigationButtons() []touchButton {
	return []touchButton{
		{Label: "File", Bounds: image.Rect(700, 518, 810, 554)},
		{Label: "Emulation", Bounds: image.Rect(820, 518, 930, 554)},
		{Label: "View", Bounds: image.Rect(700, 564, 810, 600)},
		{Label: "Tools", Bounds: image.Rect(820, 564, 930, 600)},
		{Label: "Help", Bounds: image.Rect(700, 610, 810, 646)},
	}
}

func (s *Shell) handleTouch() {
	if !platformUsesTouchLayout() {
		return
	}
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		delete(s.touchControls, id)
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if s.panel == nil && s.activeMenu < 0 {
			if control, ok := touchControlAt(x, y); ok {
				s.touchControls[id] = control
				continue
			}
		}
		s.handlePointerPress(x, y)
	}
}

func touchControlAt(x, y int) (string, bool) {
	for _, button := range touchControlButtons() {
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
	for _, button := range touchControlButtons() {
		buttonColor := touchButtonColor
		if active[button.Control] {
			buttonColor = touchActiveColor
		}
		drawTouchButton(screen, button, buttonColor)
	}
	for index, button := range touchNavigationButtons() {
		buttonColor := touchButtonColor
		if s.activeMenu == index {
			buttonColor = touchActiveColor
		}
		drawTouchButton(screen, button, buttonColor)
	}
}

func drawTouchButton(screen *ebiten.Image, button touchButton, buttonColor color.Color) {
	bounds := button.Bounds
	ebitenutil.DrawRect(
		screen,
		float64(bounds.Min.X),
		float64(bounds.Min.Y),
		float64(bounds.Dx()),
		float64(bounds.Dy()),
		buttonColor,
	)
	textX := bounds.Min.X + (bounds.Dx()-len(button.Label)*6)/2
	textY := bounds.Min.Y + (bounds.Dy()-8)/2
	ebitenutil.DebugPrintAt(screen, button.Label, textX, textY)
}
