package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type guestPointerMapping struct {
	source, destination image.Rectangle
	rotation            int
	generation          uint64
}

// guestPoint reverses textureDrawOptions, including its rotation and scaling.
// Captured releases clamp to the screen so dragging outside cannot leave a press.
func (m guestPointerMapping) guestPoint(host image.Point, clamp bool) (image.Point, bool) {
	if m.source.Empty() || m.destination.Empty() {
		return image.Point{}, false
	}
	if !clamp && !host.In(m.destination) {
		return image.Point{}, false
	}
	host.X = min(max(host.X, m.destination.Min.X), m.destination.Max.X-1)
	host.Y = min(max(host.Y, m.destination.Min.Y), m.destination.Max.Y-1)
	w, h := m.source.Dx(), m.source.Dy()
	rw, rh := w, h
	if m.rotation == 90 || m.rotation == 270 {
		rw, rh = h, w
	}
	u := (host.X - m.destination.Min.X) * rw / m.destination.Dx()
	v := (host.Y - m.destination.Min.Y) * rh / m.destination.Dy()
	switch m.rotation {
	case 90:
		return image.Pt(v, h-1-u), true
	case 180:
		return image.Pt(w-1-u, h-1-v), true
	case 270:
		return image.Pt(w-1-v, u), true
	default:
		return image.Pt(u, v), true
	}
}

type guestPointerCapture struct {
	active, mouse bool
	touchID       ebiten.TouchID
	point         image.Point
	generation    uint64
}

func (s *Shell) finishGuestPointer(backend PointerBackend) bool {
	if !s.guestPointer.active {
		return true
	}
	event := PointerInputEvent{X: s.guestPointer.point.X, Y: s.guestPointer.point.Y}
	if err := backend.QueuePointer(event); err != nil {
		s.setStatus(s.trf("Input %s: %s", "touch", err.Error()))
		return false
	}
	s.guestPointer = guestPointerCapture{}
	s.requestInputFrame()
	return true
}

func (s *Shell) handleGuestPointer() {
	backend, ok := s.backend.(PointerBackend)
	if !ok || !backend.SupportsPointer() || s.guestPointer.generation != s.frameGeneration {
		s.guestPointer = guestPointerCapture{generation: s.frameGeneration}
	}
	if !ok || !backend.SupportsPointer() {
		return
	}
	allowed := s.input != nil && !s.loading && !s.dialogOpen && !s.quitting &&
		s.panel == nil && !s.guestPointerUIBlocked && s.activeMenu < 0 && !s.touchLayoutEditing &&
		s.bindingCapture == nil && !s.uiPointerSuppressed && s.hostActive &&
		s.guestPointerMap.generation == s.frameGeneration
	if s.interfaceUI != nil && s.interfaceUI.menuWindow != nil {
		allowed = false
	}
	if s.guestPointer.active {
		if !allowed {
			s.finishGuestPointer(backend)
			return
		}
		var host image.Point
		held := false
		if s.guestPointer.mouse {
			x, y := ebiten.CursorPosition()
			host, held = image.Pt(x, y), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		} else {
			held = touchIDActive(s.guestPointer.touchID)
			if held {
				x, y := ebiten.TouchPosition(s.guestPointer.touchID)
				host = image.Pt(x, y)
			}
		}
		// Ebitengine no longer exposes the position of a released finger.
		if held || s.guestPointer.mouse {
			if point, valid := s.guestPointerMap.guestPoint(host, true); valid {
				s.guestPointer.point = point
			}
		}
		if !held {
			s.finishGuestPointer(backend)
		}
		return
	}
	if !allowed {
		return
	}
	press := func(host image.Point, mouse bool, id ebiten.TouchID) bool {
		if s.guestPointerUIAt(host) {
			return false
		}
		point, valid := s.guestPointerMap.guestPoint(host, false)
		if !valid {
			return false
		}
		if err := backend.QueuePointer(PointerInputEvent{X: point.X, Y: point.Y, Pressed: true}); err != nil {
			s.setStatus(s.trf("Input %s: %s", "touch", err.Error()))
			return false
		}
		s.guestPointer = guestPointerCapture{active: true, mouse: mouse, touchID: id,
			point: point, generation: s.frameGeneration}
		s.requestInputFrame()
		return true
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		if _, used := s.touchControls[id]; used || s.uiTouchPointers[id] ||
			s.padTouchActive && s.padTouchID == id {
			continue
		}
		x, y := ebiten.TouchPosition(id)
		if press(image.Pt(x, y), false, id) {
			return
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		press(image.Pt(x, y), true, 0)
	}
}

func (s *Shell) guestPointerUIAt(point image.Point) bool {
	w, h := s.viewportSize()
	if s.focusModeActive() {
		if point.In(focusExitBoundsForScale(w, h, s.renderScale)) {
			return true
		}
		_, control := focusControlAtRenderScale(point.X, point.Y, w, h, s.touchScale(), s.renderScale)
		if control {
			return true
		}
	}
	if s.virtualKeypadVisible() {
		if _, control := virtualKeypadControlAtSize(point.X, point.Y, w, h, s.sideKeysAvailable()); control {
			return true
		}
	}
	if platformUsesTouchLayout() && !s.onScreenControlsHidden() {
		if s.touchDpadCircular() && s.circularPadContains(point.X, point.Y) {
			return true
		}
		if _, control := s.touchControlAt(point.X, point.Y); control {
			return true
		}
	}
	return false
}
