package frontend

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The touch-layout editor replaces the whole interface while it is open:
// every on-screen control is draggable and the only other targets are the
// three actions below.

const (
	touchEditorSave   = "editor-save"
	touchEditorReset  = "editor-reset"
	touchEditorCancel = "editor-cancel"
)

const (
	touchEditorActionHeight = 44
	touchEditorActionGap    = 16
)

func touchLayoutEditorActions(width int) []touchButton {
	buttonWidth := max(80, min(150, (width-touchEditorActionGap*4)/3))
	total := buttonWidth*3 + touchEditorActionGap*2
	x := max(touchEditorActionGap, (width-total)/2)
	y := touchEditorActionGap
	step := buttonWidth + touchEditorActionGap
	return []touchButton{
		{ID: touchEditorSave, Label: "Save", Bounds: rectAt(x, y, buttonWidth, touchEditorActionHeight)},
		{ID: touchEditorReset, Label: "Reset", Bounds: rectAt(x+step, y, buttonWidth, touchEditorActionHeight)},
		{ID: touchEditorCancel, Label: "Cancel", Bounds: rectAt(x+step*2, y, buttonWidth, touchEditorActionHeight)},
	}
}

func touchLayoutEditorActionAt(x, y, width int) (string, bool) {
	for _, button := range touchLayoutEditorActions(width) {
		if pointInRect(x, y, button.Bounds) {
			return button.ID, true
		}
	}
	return "", false
}

func (s *Shell) drawTouchLayoutEditor(screen *ebiten.Image) {
	s.drawWorkspace(screen)
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	ebitenutil.DrawRect(
		screen,
		0,
		0,
		float64(width),
		float64(height),
		color.NRGBA{A: 110},
	)

	dragging := make(map[string]bool, len(s.touchLayoutDrag))
	for _, buttonID := range s.touchLayoutDrag {
		dragging[buttonID] = true
	}
	buttons := touchControlButtonsWithOptions(width, height, s.touchLayoutOptions())
	for _, button := range buttons {
		s.drawTouchButton(screen, button, dragging[button.ID])
	}
	for _, button := range touchLayoutEditorActions(width) {
		s.drawTouchButton(screen, button, false)
	}

	if s.design != nil {
		hintBounds := rectAt(0, touchEditorActionGap*2+touchEditorActionHeight, width, 20)
		drawCenteredText(
			screen,
			s.tr("Drag the touch buttons, then save the layout"),
			s.design.Type.Caption,
			s.design.Palette.Text,
			hintBounds,
			hintBounds.Min.Y,
		)
	}
}
