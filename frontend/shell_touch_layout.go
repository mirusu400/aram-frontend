package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// touchLayoutOptions resolves the persisted touch-control customization,
// substituting the in-progress draft while the layout editor is open.
func (s *Shell) touchLayoutOptions() touchLayoutOptions {
	options := touchLayoutOptions{
		Scale:      touchScaleFactor(s.settings.TouchControlScale),
		Placements: s.settings.TouchLayout,
	}
	if s.touchLayoutEditing {
		options.Placements = s.touchLayoutDraft
	}
	return options
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
	s.touchLayoutDrag = make(map[ebiten.TouchID]string)
	s.touchLayoutDragOffset = make(map[ebiten.TouchID]image.Point)
	s.touchLayoutEditing = true
	s.setStatus(s.tr("Drag the touch buttons, then save the layout"))
}

func (s *Shell) saveTouchLayoutEdit() {
	layout := s.touchLayoutDraft
	if len(layout) == 0 {
		layout = nil
	}
	s.settings.TouchLayout = layout
	s.endTouchLayoutEdit()
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Touch layout: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Touch layout saved"))
}

func (s *Shell) resetTouchLayoutDraft() {
	s.touchLayoutDraft = make(map[string]TouchPlacement)
	s.touchLayoutDrag = make(map[ebiten.TouchID]string)
	s.touchLayoutDragOffset = make(map[ebiten.TouchID]image.Point)
	s.setStatus(s.tr("Touch layout reset to defaults"))
}

func (s *Shell) cancelTouchLayoutEdit() {
	s.endTouchLayoutEdit()
	s.setStatus(s.tr("Touch layout unchanged"))
}

func (s *Shell) endTouchLayoutEdit() {
	s.touchLayoutEditing = false
	s.touchLayoutDraft = nil
	s.touchLayoutDrag = nil
	s.touchLayoutDragOffset = nil
}

func (s *Shell) handleTouchLayoutEditTouches() {
	width, height := s.viewportSize()
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		delete(s.touchLayoutDrag, id)
		delete(s.touchLayoutDragOffset, id)
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
			}
			return
		}
		button, ok := touchButtonAtWithOptions(x, y, width, height, s.touchLayoutOptions())
		if !ok {
			continue
		}
		center := button.Bounds.Min.Add(button.Bounds.Size().Div(2))
		s.touchLayoutDrag[id] = button.ID
		s.touchLayoutDragOffset[id] = center.Sub(image.Pt(x, y))
	}
	for id, buttonID := range s.touchLayoutDrag {
		x, y := ebiten.TouchPosition(id)
		offset := s.touchLayoutDragOffset[id]
		s.touchLayoutDraft[buttonID] = normalizedTouchPlacement(
			x+offset.X,
			y+offset.Y,
			width,
			height,
		)
	}
}

func copyTouchLayout(layout map[string]TouchPlacement) map[string]TouchPlacement {
	copied := make(map[string]TouchPlacement, len(layout))
	for id, placement := range layout {
		copied[id] = placement
	}
	return copied
}
