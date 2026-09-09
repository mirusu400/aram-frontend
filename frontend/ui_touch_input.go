package frontend

import (
	"image"

	euiinput "github.com/ebitenui/ebitenui/input"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// touchCursorUpdater is EbitenUI's single-pointer view of the first finger.
// It adds one capability the stock mobile adapter lacks: a scroll recognizer
// can cancel a finger, releasing the pressed widget outside its bounds so the
// eventual finger-up cannot be reported as a click.
type touchCursorUpdater struct {
	touchActive bool
	touchID     ebiten.TouchID
	canceled    map[ebiten.TouchID]bool
	position    image.Point

	leftPressed      bool
	leftJustPressed  bool
	leftJustReleased bool

	mousePressed      map[ebiten.MouseButton]bool
	mouseJustPressed  map[ebiten.MouseButton]bool
	mouseJustReleased map[ebiten.MouseButton]bool
}

func newTouchCursorUpdater() *touchCursorUpdater {
	return &touchCursorUpdater{
		canceled:          make(map[ebiten.TouchID]bool),
		position:          image.Pt(-1, -1),
		mousePressed:      make(map[ebiten.MouseButton]bool),
		mouseJustPressed:  make(map[ebiten.MouseButton]bool),
		mouseJustReleased: make(map[ebiten.MouseButton]bool),
	}
}

func installTouchCursorUpdater() *touchCursorUpdater {
	if !platformUsesTouchLayout() {
		return nil
	}
	updater := newTouchCursorUpdater()
	euiinput.SetCursorUpdater(updater)
	return updater
}

func (u *touchCursorUpdater) CancelTouch(id ebiten.TouchID) {
	if u != nil {
		u.canceled[id] = true
	}
}

func (u *touchCursorUpdater) Update() {
	for _, button := range []ebiten.MouseButton{
		ebiten.MouseButtonLeft,
		ebiten.MouseButtonMiddle,
		ebiten.MouseButtonRight,
	} {
		u.mousePressed[button] = ebiten.IsMouseButtonPressed(button)
		u.mouseJustPressed[button] = inpututil.IsMouseButtonJustPressed(button)
		u.mouseJustReleased[button] = inpututil.IsMouseButtonJustReleased(button)
	}

	touchIDs := ebiten.AppendTouchIDs(nil)
	hadTouch := u.touchActive
	desiredPressed := false
	if u.touchActive {
		if touchIDIn(u.touchID, touchIDs) {
			u.position.X, u.position.Y = ebiten.TouchPosition(u.touchID)
			desiredPressed = !u.canceled[u.touchID]
			if !desiredPressed {
				u.position = image.Pt(-1, -1)
			}
		} else {
			if u.canceled[u.touchID] {
				u.position = image.Pt(-1, -1)
			}
			delete(u.canceled, u.touchID)
			u.touchActive = false
		}
	} else if len(touchIDs) > 0 {
		u.touchActive = true
		u.touchID = touchIDs[0]
		u.position.X, u.position.Y = ebiten.TouchPosition(u.touchID)
		desiredPressed = !u.canceled[u.touchID]
		if !desiredPressed {
			u.position = image.Pt(-1, -1)
		}
	} else {
		u.position.X, u.position.Y = ebiten.CursorPosition()
		desiredPressed = u.mousePressed[ebiten.MouseButtonLeft]
	}

	// Do not turn a second finger or a mouse button into a new press on the
	// exact frame the prior primary finger ended.
	if hadTouch && !u.touchActive {
		desiredPressed = false
	}
	u.leftJustPressed = desiredPressed && !u.leftPressed
	u.leftJustReleased = !desiredPressed && u.leftPressed
	u.leftPressed = desiredPressed
}

func touchIDIn(id ebiten.TouchID, ids []ebiten.TouchID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func (u *touchCursorUpdater) AfterUpdate()            {}
func (u *touchCursorUpdater) Draw(*ebiten.Image)      {}
func (u *touchCursorUpdater) AfterDraw(*ebiten.Image) {}

func (u *touchCursorUpdater) MouseButtonPressed(button ebiten.MouseButton) bool {
	if button == ebiten.MouseButtonLeft {
		return u.leftPressed
	}
	return u.mousePressed[button]
}

func (u *touchCursorUpdater) MouseButtonJustPressed(button ebiten.MouseButton) bool {
	if button == ebiten.MouseButtonLeft {
		return u.leftJustPressed
	}
	return u.mouseJustPressed[button]
}

func (u *touchCursorUpdater) MouseButtonJustReleased(button ebiten.MouseButton) bool {
	if button == ebiten.MouseButtonLeft {
		return u.leftJustReleased
	}
	return u.mouseJustReleased[button]
}

func (u *touchCursorUpdater) CursorPosition() (int, int) {
	return u.position.X, u.position.Y
}

func (u *touchCursorUpdater) GetCursorImage(string) *ebiten.Image { return nil }
func (u *touchCursorUpdater) GetCursorOffset(string) image.Point  { return image.Point{} }
