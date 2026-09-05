package frontend

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// homeNavRepeatDelay and homeNavRepeatInterval pace held-key auto-repeat while
// navigating the launcher (frames at 60fps: ~0.37s before repeat, ~12/s after).
const (
	homeNavRepeatDelay    = 22
	homeNavRepeatInterval = 5
)

// Home launcher: the idle-state screen drawn in the guest viewport in place of
// the bare "No guest frame" message. It offers three tabs — recently played,
// titles installed under the user's library folders, and starred favorites —
// so a title can be reopened without walking File > Open every time. It is not
// a modal panel: it is the background surface, so File/Settings dialogs float
// over it.

const (
	homeTabRecent    = "recent"
	homeTabInstalled = "installed"
	homeTabFavorites = "favorites"
)

// homeTabs is the fixed left-to-right tab order.
func homeTabs() []string {
	return []string{homeTabRecent, homeTabInstalled, homeTabFavorites}
}

// showHomeSurface reports whether the launcher takes the guest viewport: only
// when the shell is idle (no title, not loading, no load error) and no
// full-screen mode owns the surface instead.
func (s *Shell) showHomeSurface() bool {
	if s.input != nil || s.loading || s.problem != nil {
		return false
	}
	if s.focusModeActive() || s.touchChromeHiddenActive() || s.touchLayoutEditing {
		return false
	}
	return true
}

// guestViewportRect is the on-screen rectangle the windowed guest screen fills.
// drawWorkspace draws the guest (or the Home surface) here, and the UI positions
// the Home widgets to match, so both agree on the same rectangle. It mirrors the
// inset math in drawWorkspace.
func (s *Shell) guestViewportRect(width, height int) image.Rectangle {
	contentTop := menuHeight + applicationToolbarHeight + 12
	contentBottom := height - statusHeight - 12
	if platformUsesTouchLayout() {
		contentBottom -= s.touchDeckHeight(width, height)
	}
	contentRight := width - 12
	if s.virtualKeypadVisible() {
		contentRight -= virtualKeypadReservedWidthFor(width)
	}
	viewportPanel := image.Rect(12, contentTop, contentRight, contentBottom)
	return image.Rect(
		viewportPanel.Min.X+6,
		viewportPanel.Min.Y+6,
		viewportPanel.Max.X-6,
		viewportPanel.Max.Y-6,
	)
}

// setHomeTab switches the active launcher tab.
func (s *Shell) setHomeTab(tab string) {
	switch tab {
	case homeTabRecent, homeTabInstalled, homeTabFavorites:
		s.homeTab = tab
	}
}

// setHomeFilter narrows every Home tab to titles whose name contains query
// (case-insensitive). It applies across tab switches until cleared.
func (s *Shell) setHomeFilter(query string) {
	s.homeFilterQuery = query
}

// focusHomeSearch moves keyboard focus to the Home search field (Ctrl+F or
// "/", see handleShortcuts), so a keyboard-only user does not need the mouse
// to start typing. A no-op before Home has ever been shown (the field is
// built lazily by ensureHomeChrome in ui_home_search.go).
func (s *Shell) focusHomeSearch() {
	if s.interfaceUI == nil || s.interfaceUI.homeSearchInput == nil {
		return
	}
	s.interfaceUI.homeSearchInput.Focus(true)
}

// homeOpenPath loads a title chosen from any Home tab, mirroring openRecentPath.
func (s *Shell) homeOpenPath(path string) {
	if path == "" {
		s.setStatus(s.tr("Home: no title selected"))
		return
	}
	s.openRequest(OpenRequest{Path: path})
}

// homeTabEntries returns the rows shown for a tab, narrowed by homeFilterQuery
// when set. Recent and Favorites are stored as plain paths; Installed comes
// from the recursive scan.
func (s *Shell) homeTabEntries(tab string) []LibraryEntry {
	var entries []LibraryEntry
	switch tab {
	case homeTabInstalled:
		entries = s.libraryEntries
	case homeTabFavorites:
		entries = pathsToLibraryEntries(s.settings.FavoriteFiles)
	default:
		entries = recentToLibraryEntries(s.settings.RecentFiles)
	}
	return filterLibraryEntries(entries, s.homeFilterQuery)
}

// filterLibraryEntries keeps only entries whose name contains query,
// case-insensitively. An empty query returns entries unchanged.
func filterLibraryEntries(entries []LibraryEntry, query string) []LibraryEntry {
	query = strings.TrimSpace(query)
	if query == "" {
		return entries
	}
	query = strings.ToLower(query)
	filtered := make([]LibraryEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name), query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// homeLibraryFolders returns the configured scan roots for the Installed tab.
func (s *Shell) homeLibraryFolders() []string {
	return s.settings.GameLibraryFolders
}

// pathsToLibraryEntries wraps stored paths as display rows.
func pathsToLibraryEntries(paths []string) []LibraryEntry {
	if len(paths) == 0 {
		return nil
	}
	entries := make([]LibraryEntry, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		entries = append(entries, LibraryEntry{Path: path, Name: libraryEntryName(path)})
	}
	return entries
}

// recentToLibraryEntries wraps recent entries as display rows, preferring the
// display name captured when each was opened (see RecentEntry) over a name
// derived from the path — the path itself is a private cache copy for a
// desktop drop or an opaque content:// URI on Android, so it is not always
// readable on its own.
func recentToLibraryEntries(entries []RecentEntry) []LibraryEntry {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]LibraryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		name := entry.Name
		if name == "" {
			name = libraryEntryName(entry.Path)
		}
		rows = append(rows, LibraryEntry{Path: entry.Path, Name: name})
	}
	return rows
}

// chooseLibraryFolder opens the native folder picker; the chosen folder is
// added to the scan roots when it returns (see consumePickerResult).
func (s *Shell) chooseLibraryFolder() {
	if s.dialogOpen || s.loading {
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	s.setStatus(s.tr("Waiting for game library folder selection..."))
	previous := ""
	if folders := s.settings.GameLibraryFolders; len(folders) > 0 {
		previous = folders[len(folders)-1]
	}
	go func() {
		path, err := s.picker.OpenGameDirectory(previous)
		s.pickerResults <- pickerResult{operation: operationLibraryFolder, path: path, err: err}
	}()
}

// addLibraryFolderPath records a newly chosen scan root and rescans.
func (s *Shell) addLibraryFolderPath(path string) {
	if !s.settings.addLibraryFolder(path) {
		s.setStatus(s.tr("That folder is already in the library"))
		return
	}
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Library folder: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Library folder added"))
	s.rescanLibrary()
}

// removeLibraryFolderPath drops a scan root and rescans.
func (s *Shell) removeLibraryFolderPath(path string) {
	if !s.settings.removeLibraryFolder(path) {
		return
	}
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Library folder: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Library folder removed"))
	s.rescanLibrary()
}

// toggleFavoritePath stars or unstars a title.
func (s *Shell) toggleFavoritePath(path string) {
	if path == "" {
		return
	}
	starred := s.settings.toggleFavorite(path)
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Favorite: ") + err.Error())
		return
	}
	if starred {
		s.setStatus(s.tr("Added to favorites"))
	} else {
		s.setStatus(s.tr("Removed from favorites"))
	}
}

// handleHomeNavigation reads the launcher's directional and confirm controls —
// the same keyboard, gamepad, and on-screen keypad bindings the guest uses —
// and drives the Home selection: up/down move, left/right switch tabs, ok opens.
func (s *Shell) handleHomeNavigation() {
	if s.interfaceUI == nil {
		return
	}
	next := s.collectHomeNavControls()
	if s.homeNavHold == nil {
		s.homeNavHold = make(map[string]int)
	}
	for _, control := range []string{"up", "down", "left", "right", "ok"} {
		if next[control] {
			s.homeNavHold[control]++
		} else {
			s.homeNavHold[control] = 0
		}
	}
	if homeNavFires(s.homeNavHold["up"], true) {
		s.interfaceUI.moveHomeSelection(-1)
	}
	if homeNavFires(s.homeNavHold["down"], true) {
		s.interfaceUI.moveHomeSelection(1)
	}
	if homeNavFires(s.homeNavHold["left"], false) {
		s.interfaceUI.switchHomeTab(s, -1)
	}
	if homeNavFires(s.homeNavHold["right"], false) {
		s.interfaceUI.switchHomeTab(s, 1)
	}
	if s.homeNavHold["ok"] == 1 {
		s.interfaceUI.activateHomeSelection(s)
	}
}

// homeNavFires reports whether a control held for hold frames should act this
// frame: always on the first frame, then on the repeat cadence when repeat is
// allowed (up/down scroll; tab switch and confirm fire on the edge only).
func homeNavFires(hold int, repeat bool) bool {
	if hold == 1 {
		return true
	}
	if repeat && hold >= homeNavRepeatDelay && (hold-homeNavRepeatDelay)%homeNavRepeatInterval == 0 {
		return true
	}
	return false
}

// collectHomeNavControls samples the pressed navigation controls from the
// keyboard bindings, gamepad, on-screen keypad, and touch dpad.
func (s *Shell) collectHomeNavControls() map[string]bool {
	next := make(map[string]bool)
	profile := s.controllerProfile()
	if !homeNavModifierPressed() {
		for _, binding := range keyboardBindingsForProfile(profile) {
			if ebiten.IsKeyPressed(binding.Key) {
				next[binding.Control] = true
			}
		}
	}
	s.collectGamepadState(next, profile)
	s.collectVirtualKeypadState(next)
	s.collectTouchState(next)
	return next
}

// homeNavModifierPressed reports whether a Ctrl/Alt chord is held, so a shortcut
// like Ctrl+O is not also read as a bare navigation key.
func homeNavModifierPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight) ||
		ebiten.IsKeyPressed(ebiten.KeyAlt) ||
		ebiten.IsKeyPressed(ebiten.KeyAltLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyAltRight)
}

// rescanLibrary walks the configured roots on a background goroutine and
// delivers the result to the Update loop through libraryResults. A snapshot of
// the roots is passed to the goroutine so a concurrent settings edit cannot
// race the walk.
func (s *Shell) rescanLibrary() {
	roots := append([]string(nil), s.settings.GameLibraryFolders...)
	if len(roots) == 0 {
		s.libraryScanning = false
		s.libraryEntries = nil
		return
	}
	s.libraryScanning = true
	patterns := supportedInputPatterns()
	go func() {
		s.libraryResults <- scanLibraryFolders(roots, patterns, libraryScanLimit)
	}()
}
