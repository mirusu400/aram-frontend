package frontend

import (
	"time"

	euiimage "github.com/ebitenui/ebitenui/image"
)

// Home launcher selection and navigation. Selection is driven both by mouse
// clicks (onHomeRowClicked) and by directional/confirm input the Shell forwards
// here (moveHomeSelection, switchHomeTab, activateHomeSelection), so the picker
// works like a handset's — keyboard, gamepad, or on-screen keypad.

const homeDoubleClickInterval = 500 * time.Millisecond

// A desktop click selects; a second click within the interval opens. A touch
// tap opens immediately, independently of the row's previous selection.
func (u *shellUI) onHomeRowClicked(shell *Shell, path string) {
	u.handleHomeRowClick(shell, path, platformUsesTouchLayout(), time.Now())
}

func (u *shellUI) handleHomeRowClick(shell *Shell, path string, touch bool, now time.Time) {
	if path == "" {
		return
	}
	elapsed := now.Sub(u.homeLastClickAt)
	activate := touch || (u.homeSelectedPath == path && u.homeLastClickPath == path &&
		!u.homeLastClickAt.IsZero() && elapsed >= 0 && elapsed <= homeDoubleClickInterval)
	u.highlightHomeRow(path)
	if activate {
		u.homeLastClickPath = ""
		u.homeLastClickAt = time.Time{}
		shell.homeOpenPath(path)
		return
	}
	u.homeLastClickPath = path
	u.homeLastClickAt = now
}

// highlightHomeRow moves the selection to path, swapping the row backgrounds and
// enabling the soft keys. It does not rebuild, so scroll position is kept.
func (u *shellUI) highlightHomeRow(path string) {
	if u.homeSelectedPath == path {
		return
	}
	if previous, ok := u.homeRowContainers[u.homeSelectedPath]; ok {
		previous.SetBackgroundImage(euiimage.NewNineSliceColor(homeColorTransparent))
	}
	if current, ok := u.homeRowContainers[path]; ok {
		current.SetBackgroundImage(euiimage.NewNineSliceColor(homeColorRowSelect))
	}
	u.homeSelectedPath = path
	enabled := path != ""
	if u.homeOpenButton != nil {
		u.homeOpenButton.GetWidget().Disabled = !enabled
	}
	if u.homeFavButton != nil {
		u.homeFavButton.GetWidget().Disabled = !enabled
		u.homeFavButton.SetText(u.owner.homeFavoriteActionLabel(path))
	}
	if u.homeShortcutButton != nil {
		u.homeShortcutButton.GetWidget().Disabled = !enabled
	}
}

// homeSelectedIndex is the row index of the current selection, or -1.
func (u *shellUI) homeSelectedIndex() int {
	for index, path := range u.homeRowPaths {
		if path == u.homeSelectedPath {
			return index
		}
	}
	return -1
}

// moveHomeSelection moves the highlighted row by delta, clamped to the ends, and
// scrolls it into view. Driven by up/down from keyboard, gamepad, or keypad.
func (u *shellUI) moveHomeSelection(delta int) {
	if len(u.homeRowPaths) == 0 {
		return
	}
	index := u.homeSelectedIndex()
	if index < 0 {
		index = 0
	} else {
		index += delta
	}
	if index < 0 {
		index = 0
	}
	if index >= len(u.homeRowPaths) {
		index = len(u.homeRowPaths) - 1
	}
	u.highlightHomeRow(u.homeRowPaths[index])
	u.scrollHomeToIndex(index)
}

// switchHomeTab moves to the previous/next tab, wrapping around.
func (u *shellUI) switchHomeTab(shell *Shell, delta int) {
	tabs := homeTabs()
	current := 0
	for index, tab := range tabs {
		if tab == shell.homeTab {
			current = index
		}
	}
	current = (current + delta + len(tabs)) % len(tabs)
	shell.setHomeTab(tabs[current])
}

// activateHomeSelection opens the highlighted title (the confirm/OK key).
func (u *shellUI) activateHomeSelection(shell *Shell) {
	if u.homeSelectedPath != "" {
		shell.homeOpenPath(u.homeSelectedPath)
	}
}

// scrollHomeToIndex nudges the scroll container so row index stays visible.
func (u *shellUI) scrollHomeToIndex(index int) {
	if u.homeScroll == nil {
		return
	}
	overflow := float64(u.homeScroll.ContentRect().Dy() - u.homeScroll.ViewRect().Dy())
	if overflow <= 0 {
		u.homeScroll.ScrollTop = 0
		return
	}
	viewHeight := float64(u.homeScroll.ViewRect().Dy())
	rowHeight := homeRowHeight
	if u.design != nil {
		rowHeight = u.design.px(homeRowHeight)
	}
	rowTop := float64(index * rowHeight)
	rowBottom := rowTop + float64(rowHeight)
	top := u.homeScroll.ScrollTop * overflow
	if rowTop < top {
		top = rowTop
	} else if rowBottom > top+viewHeight {
		top = rowBottom - viewHeight
	}
	u.homeScroll.ScrollTop = top / overflow
}
