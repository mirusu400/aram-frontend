package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// touchLayoutOptions resolves the persisted touch-control customization,
// substituting the in-progress draft while the layout editor is open.
// touchDeckHeight reserves the on-screen control deck for the current
// options, which decides whether the numeric keypad is part of it.
func (s *Shell) touchDeckHeight(width, height int) int {
	if s.secondaryKeypadEnabled() {
		// A second physical panel owns the controls, so the game panel keeps
		// its whole height with no deck reserved.
		return 0
	}
	return touchDeckHeightWithOptions(width, height, s.touchLayoutOptions())
}

func (s *Shell) touchLayoutOptions() touchLayoutOptions {
	options := touchLayoutOptions{
		Scale:      touchScaleFactor(s.settings.TouchControlScale),
		Placements: s.settings.TouchLayout,
		Keypad:     s.touchKeypadVisible(),
		DeckRatio:  s.settings.TouchDeckRatio,
		Hidden:     s.settings.TouchHidden,
	}
	if s.touchLayoutEditing {
		// Everything the editor can change is drafted, so cancelling really
		// does leave the saved layout alone.
		options.Placements = s.touchLayoutDraft
		options.Hidden = s.touchHiddenDraft
		options.DeckRatio = s.touchDeckRatioDraft
		options.Scale = touchScaleFactor(s.touchScaleDraft)
	}
	return options
}

// touchEditorDeckRatio and touchEditorScale report the values the editor's
// steppers show and change.
func (s *Shell) touchEditorDeckRatio() int {
	width, height := s.viewportSize()
	if s.touchDeckRatioDraft > 0 {
		return clampInt(s.touchDeckRatioDraft, touchDeckRatioMin, touchDeckRatioMax)
	}
	return touchDeckRatioPercent(width, height, s.touchLayoutOptions())
}

func (s *Shell) touchEditorScale() int {
	if s.touchScaleDraft == 0 {
		return 100
	}
	return clampInt(s.touchScaleDraft, touchControlScaleMin, touchControlScaleMax)
}

func (s *Shell) adjustTouchDeckRatio(delta int) {
	s.touchDeckRatioDraft = clampInt(
		s.touchEditorDeckRatio()+delta,
		touchDeckRatioMin,
		touchDeckRatioMax,
	)
	s.setStatus(s.trf("Guest screen: %d%% of the display", 100-s.touchDeckRatioDraft))
}

func (s *Shell) adjustTouchEditorScale(delta int) {
	s.touchScaleDraft = clampInt(
		s.touchEditorScale()+delta,
		touchControlScaleMin,
		touchControlScaleMax,
	)
	s.setStatus(s.trf("Touch button size: %d%%", s.touchScaleDraft))
}

// touchEditorGridStep is the snap grid the editor draws and drags against, in
// screen pixels. Zero means the grid is off.
func (s *Shell) touchEditorGridStep() int {
	if s.touchGridStepDraft <= 0 {
		return 0
	}
	return clampInt(s.touchGridStepDraft, touchGridStepMin, touchGridStepMax)
}

// adjustTouchEditorGrid coarsens or refines the snap grid. Stepping the grid
// below its minimum turns snapping off, and the first step up from off arms it
// at the minimum, so one stepper doubles as the on/off switch.
func (s *Shell) adjustTouchEditorGrid(delta int) {
	step := s.touchEditorGridStep()
	switch {
	case step == 0:
		if delta > 0 {
			step = touchGridStepMin
		}
	default:
		step = clampInt(step+delta, 0, touchGridStepMax)
		if step < touchGridStepMin {
			step = 0
		}
	}
	s.touchGridStepDraft = step
	if step == 0 {
		s.setStatus(s.tr("Grid snap: off"))
		return
	}
	s.setStatus(s.trf("Grid snap: %d px", step))
}

// setTouchButtonHidden puts a button away or brings it back while editing.
func (s *Shell) setTouchButtonHidden(id string, hidden bool) {
	if s.touchHiddenDraft == nil {
		s.touchHiddenDraft = make(map[string]bool)
	}
	if hidden {
		s.touchHiddenDraft[id] = true
		return
	}
	delete(s.touchHiddenDraft, id)
}

func (s *Shell) touchScale() float64 {
	return touchScaleFactor(s.settings.TouchControlScale)
}

func (s *Shell) touchControlScalePercent() int {
	if s.settings.TouchControlScale == 0 {
		return 100
	}
	return clampInt(
		s.settings.TouchControlScale,
		touchControlScaleMin,
		touchControlScaleMax,
	)
}

func (s *Shell) setTouchControlScale(percent int) {
	s.settings.TouchControlScale = clampInt(
		percent,
		touchControlScaleMin,
		touchControlScaleMax,
	)
	_ = s.settings.save()
	s.setStatus(s.trf("Touch button size: %d%%", s.touchControlScalePercent()))
}

func (s *Shell) beginTouchLayoutEdit() {
	if !platformUsesTouchLayout() {
		return
	}
	s.panel = nil
	s.activeMenu = -1
	s.focusMode = false
	s.touchLayoutDraft = copyTouchLayout(s.settings.TouchLayout)
	s.touchHiddenDraft = copyTouchHidden(s.settings.TouchHidden)
	s.touchDeckRatioDraft = s.settings.TouchDeckRatio
	s.touchScaleDraft = s.touchControlScalePercent()
	s.touchGridStepDraft = s.settings.TouchGridStep
	s.touchLayoutDrag = make(map[ebiten.TouchID]string)
	s.touchLayoutDragOffset = make(map[ebiten.TouchID]image.Point)
	s.touchLayoutDragPoint = make(map[ebiten.TouchID]image.Point)
	s.touchLayoutEditing = true
	s.setStatus(s.tr("Drag the touch buttons, then save the layout"))
}

func (s *Shell) saveTouchLayoutEdit() {
	layout := s.touchLayoutDraft
	if len(layout) == 0 {
		layout = nil
	}
	hidden := s.touchHiddenDraft
	if len(hidden) == 0 {
		hidden = nil
	}
	s.settings.TouchLayout = layout
	s.settings.TouchHidden = hidden
	s.settings.TouchDeckRatio = s.touchDeckRatioDraft
	s.settings.TouchControlScale = s.touchScaleDraft
	s.settings.TouchGridStep = s.touchEditorGridStep()
	s.endTouchLayoutEdit()
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Touch layout: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Touch layout saved"))
}

func (s *Shell) resetTouchLayoutDraft() {
	s.touchLayoutDraft = make(map[string]TouchPlacement)
	s.touchHiddenDraft = make(map[string]bool)
	s.touchDeckRatioDraft = 0
	s.touchScaleDraft = 100
	s.touchGridStepDraft = 0
	s.touchLayoutDrag = make(map[ebiten.TouchID]string)
	s.touchLayoutDragOffset = make(map[ebiten.TouchID]image.Point)
	s.touchLayoutDragPoint = make(map[ebiten.TouchID]image.Point)
	s.setStatus(s.tr("Touch layout reset to defaults"))
}

func (s *Shell) cancelTouchLayoutEdit() {
	s.endTouchLayoutEdit()
	s.setStatus(s.tr("Touch layout unchanged"))
}

func (s *Shell) endTouchLayoutEdit() {
	s.touchLayoutEditing = false
	s.touchLayoutDraft = nil
	s.touchHiddenDraft = nil
	s.touchDeckRatioDraft = 0
	s.touchScaleDraft = 0
	s.touchGridStepDraft = 0
	s.touchLayoutDrag = nil
	s.touchLayoutDragOffset = nil
	s.touchLayoutDragPoint = nil
}

func (s *Shell) handleTouchLayoutEditTouches() {
	width, height := s.viewportSize()
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		s.finishTouchLayoutDrag(id, width, height)
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if action, ok := touchLayoutEditorActionAt(x, y, width); ok {
			switch action {
			case touchEditorSave:
				s.saveTouchLayoutEdit()
			case touchEditorReset:
				s.resetTouchLayoutDraft()
			case touchEditorCancel:
				s.cancelTouchLayoutEdit()
			// The reading is the guest screen's share, so growing it has to
			// take height off the deck.
			case touchEditorGuestSmaller:
				s.adjustTouchDeckRatio(touchEditorDeckStep)
			case touchEditorGuestLarger:
				s.adjustTouchDeckRatio(-touchEditorDeckStep)
			case touchEditorSizeSmaller:
				s.adjustTouchEditorScale(-touchEditorSizeStep)
			case touchEditorSizeLarger:
				s.adjustTouchEditorScale(touchEditorSizeStep)
			case touchEditorGridFiner:
				s.adjustTouchEditorGrid(-touchEditorGridStepDelta)
			case touchEditorGridCoarser:
				s.adjustTouchEditorGrid(touchEditorGridStepDelta)
			}
			return
		}
		button, ok := s.touchEditorButtonAt(x, y, width, height)
		if !ok {
			continue
		}
		center := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
		s.touchLayoutDrag[id] = button.ID
		s.touchLayoutDragOffset[id] = center.Sub(image.Pt(x, y))
		s.touchLayoutDragPoint[id] = image.Pt(x, y)
	}
	for id, buttonID := range s.touchLayoutDrag {
		x, y := ebiten.TouchPosition(id)
		s.touchLayoutDragPoint[id] = image.Pt(x, y)
		if s.touchHiddenDraft[buttonID] {
			// A button still in the tray follows the finger only once it
			// leaves; until then it keeps its slot in the tray grid.
			continue
		}
		offset := s.touchLayoutDragOffset[id]
		step := s.touchEditorGridStep()
		s.touchLayoutDraft[buttonID] = normalizedTouchPlacement(
			snapToGrid(x+offset.X, step),
			snapToGrid(y+offset.Y, step),
			width,
			height,
		)
	}
}

// touchEditorButtonAt resolves a press to a deck button or to one already in
// the tray, so both directions of the hide/restore gesture start the same way.
func (s *Shell) touchEditorButtonAt(x, y, width, height int) (touchButton, bool) {
	options := s.touchLayoutOptions()
	for _, button := range touchEditorTrayButtons(width, height, options) {
		if pointInRect(x, y, button.Bounds) {
			return button, true
		}
	}
	return touchButtonAtWithOptions(x, y, width, height, options)
}

// finishTouchLayoutDrag decides what a released drag meant: dropped in the
// tray it puts the button away, dropped outside it brings a tray button back
// where the finger left it.
func (s *Shell) finishTouchLayoutDrag(id ebiten.TouchID, width, height int) {
	buttonID, dragging := s.touchLayoutDrag[id]
	point, tracked := s.touchLayoutDragPoint[id]
	offset := s.touchLayoutDragOffset[id]
	delete(s.touchLayoutDrag, id)
	delete(s.touchLayoutDragOffset, id)
	delete(s.touchLayoutDragPoint, id)
	if !dragging || !tracked {
		return
	}
	options := s.touchLayoutOptions()
	tray := touchEditorTrayBounds(width, height, options)
	inTray := pointInRect(point.X, point.Y, tray)
	wasHidden := s.touchHiddenDraft[buttonID]
	switch {
	case inTray && !wasHidden:
		s.setTouchButtonHidden(buttonID, true)
		s.setStatus(s.trf("%s hidden", s.touchButtonName(buttonID)))
	case !inTray && wasHidden:
		s.setTouchButtonHidden(buttonID, false)
		step := s.touchEditorGridStep()
		s.touchLayoutDraft[buttonID] = normalizedTouchPlacement(
			snapToGrid(point.X+offset.X, step),
			snapToGrid(point.Y+offset.Y, step),
			width,
			height,
		)
		s.setStatus(s.trf("%s restored", s.touchButtonName(buttonID)))
	}
}

// touchButtonName is the label a status message uses for a slot.
func (s *Shell) touchButtonName(id string) string {
	width, height := s.viewportSize()
	options := s.touchLayoutOptions()
	for _, button := range touchButtonCatalog(width, height, options) {
		if button.ID == id {
			return s.tr(button.Label)
		}
	}
	return id
}

func copyTouchHidden(hidden map[string]bool) map[string]bool {
	copied := make(map[string]bool, len(hidden))
	for id, value := range hidden {
		if value {
			copied[id] = true
		}
	}
	return copied
}

func copyTouchLayout(layout map[string]TouchPlacement) map[string]TouchPlacement {
	copied := make(map[string]TouchPlacement, len(layout))
	for id, placement := range layout {
		copied[id] = placement
	}
	return copied
}
