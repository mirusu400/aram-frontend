package frontend

import "github.com/hajimehoshi/ebiten/v2"

// Touch layouts hide the menu bar, toolbar, and status bar while a title is
// playing so the guest screen can take the whole area above the control deck.
// A small floating toggle at the top-right brings the chrome back; panels and
// the layout editor always force the full interface.

// touchChromeHiddenActive reports whether the shell chrome is currently
// stripped in favor of the full-bleed guest viewport.
func (s *Shell) touchChromeHiddenActive() bool {
	return s.touchChromeHidden &&
		platformUsesTouchLayout() &&
		s.panel == nil &&
		!s.touchLayoutEditing &&
		!s.focusModeActive()
}

// touchChromeToggleAvailable reports whether the floating MENU/HIDE toggle is
// interactive. Focus mode keeps its own EXIT button, the layout editor owns
// the whole screen, and open panels or menus already cover the toggle's spot.
func (s *Shell) touchChromeToggleAvailable() bool {
	return platformUsesTouchLayout() &&
		s.panel == nil &&
		!s.touchLayoutEditing &&
		!s.focusModeActive() &&
		(s.touchChromeHidden || s.activeMenu < 0)
}

func (s *Shell) toggleTouchChrome() {
	s.touchChromeHidden = !s.touchChromeHidden
	s.activeMenu = -1
	// The tap that revealed the chrome must not also press whatever chrome
	// widget now sits under the same finger, so the interface UI stays
	// deaf until that touch is released.
	s.uiPointerSuppressed = true
}

// syncUIPointerSuppression lifts the post-toggle input hold once every
// pointer has been released.
func (s *Shell) syncUIPointerSuppression() {
	if !s.uiPointerSuppressed {
		return
	}
	if len(ebiten.AppendTouchIDs(nil)) == 0 &&
		!ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.uiPointerSuppressed = false
	}
}

// syncTouchChrome hides the chrome when a title enters the running state, so
// starting or resuming play always returns to the full-size guest screen.
func (s *Shell) syncTouchChrome() {
	previous := s.touchChromeSyncState
	s.touchChromeSyncState = s.state
	if !platformUsesTouchLayout() {
		return
	}
	if s.state == FrontendRunning && previous != FrontendRunning {
		s.touchChromeHidden = true
		s.activeMenu = -1
	}
}
