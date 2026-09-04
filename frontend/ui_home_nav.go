package frontend

import euiimage "github.com/ebitenui/ebitenui/image"

// Home launcher selection and navigation. Selection is driven both by mouse
// clicks (onHomeRowClicked) and by directional/confirm input the Shell forwards
// here (moveHomeSelection, switchHomeTab, activateHomeSelection), so the picker
// works like a handset's — keyboard, gamepad, or on-screen keypad.

// onHomeRowClicked selects a row, or opens it when it is already selected.
func (u *shellUI) onHomeRowClicked(shell *Shell, path string) {
	if path == "" {
		return
	}
	if u.homeSelectedPath == path {
		shell.homeOpenPath(path)
		return
	}
	u.highlightHomeRow(path)
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
	rowTop := float64(index * homeRowHeight)
	rowBottom := rowTop + float64(homeRowHeight)
	top := u.homeScroll.ScrollTop * overflow
	if rowTop < top {
		top = rowTop
	} else if rowBottom > top+viewHeight {
		top = rowBottom - viewHeight
	}
	u.homeScroll.ScrollTop = top / overflow
}
