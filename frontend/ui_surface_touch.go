package frontend

import (
	"image"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type surfaceTouchScroll struct {
	scroll                *widget.ScrollContainer
	id                    ebiten.TouchID
	active, dragged       bool
	startX, startY, lastY int
}

// Cancel the pointer as soon as a vertical drag starts, so scrolling a game
// list cannot also launch the row under the finger when it is released.
func (u *shellUI) updateSurfaceTouchScroll(shell *Shell) {
	var scroll *widget.ScrollContainer
	if platformUsesTouchLayout() && shell.panel == nil && shell.activeMenu < 0 && !shell.dialogOpen &&
		!shell.focusModeActive() && !shell.touchChromeHiddenActive() && !shell.touchLayoutEditing {
		if shell.problem != nil && !shell.loading {
			scroll = u.problemScroll
		} else if shell.showHomeSurface() {
			scroll = u.homeScroll
		}
	}
	gesture := &u.surfaceTouch
	if scroll == nil || gesture.scroll != scroll {
		*gesture = surfaceTouchScroll{scroll: scroll}
	}
	if scroll == nil {
		return
	}
	if gesture.active {
		if inpututil.IsTouchJustReleased(gesture.id) || !touchIDActive(gesture.id) {
			gesture.active = false
			return
		}
		x, y := ebiten.TouchPosition(gesture.id)
		if !gesture.dragged && touchScrollDragStarted(x-gesture.startX, y-gesture.startY, shell.px(8)) {
			gesture.dragged = true
			u.touchCursor.CancelTouch(gesture.id)
		}
		delta := y - gesture.lastY
		gesture.lastY = y
		if gesture.dragged {
			overflow := scroll.ContentRect().Dy() - scroll.ViewRect().Dy()
			if overflow > 0 {
				scroll.ScrollTop = min(1, max(0, scroll.ScrollTop-float64(delta)/float64(overflow)))
			}
		}
		return
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if image.Pt(x, y).In(scroll.ViewRect()) {
			*gesture = surfaceTouchScroll{scroll: scroll, id: id, active: true, startX: x, startY: y, lastY: y}
			return
		}
	}
}
