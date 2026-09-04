package frontend

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIdleLaunchShowsHomeSurface(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(&openRecordingBackend{requests: make(chan OpenRequest, 1)}, nil, "")
	if shell.panel != nil {
		t.Fatalf("idle launch opened a modal panel = %#v", shell.panel)
	}
	if !shell.showHomeSurface() {
		t.Fatal("idle launch does not show the Home surface")
	}
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	if shell.interfaceUI.homeScroll == nil {
		t.Fatal("Home surface was not populated on an idle launch")
	}
}

func TestHomeTabsListTheirOwnEntries(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.RecentFiles = []string{
		filepath.Join("games", "a.dat"),
		filepath.Join("games", "b.dat"),
		filepath.Join("games", "c.dat"),
	}
	shell.settings.FavoriteFiles = []string{filepath.Join("games", "b.dat")}
	shell.libraryEntries = []LibraryEntry{
		{Path: filepath.Join("lib", "x.jar"), Name: "x"},
		{Path: filepath.Join("lib", "y.jar"), Name: "y"},
	}

	shell.setHomeTab(homeTabRecent)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	if shell.interfaceUI.homeScroll == nil {
		t.Fatal("home rows were not created for the Recent tab")
	}
	if got := len(shell.interfaceUI.homeRowPaths); got != 3 {
		t.Fatalf("Recent tab entries = %d, want 3", got)
	}

	shell.setHomeTab(homeTabInstalled)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	if got := len(shell.interfaceUI.homeRowPaths); got != 2 {
		t.Fatalf("Installed tab entries = %d, want 2", got)
	}

	shell.setHomeTab(homeTabFavorites)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	if got := len(shell.interfaceUI.homeRowPaths); got != 1 {
		t.Fatalf("Favorites tab entries = %d, want 1", got)
	}
}

func TestHomeSurfaceHiddenWhileTitleLoaded(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.input = &InputInfo{DisplayName: "loaded.dat"}
	if shell.showHomeSurface() {
		t.Fatal("Home surface still shows while a title is loaded")
	}
}

func TestHomeOpenPathReachesBackendOpen(t *testing.T) {
	isolateSettledSettings(t)
	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, nil, "")

	path := filepath.Join(t.TempDir(), "synthetic.dat")
	shell.homeOpenPath(path)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if request.Path != path || request.Firmware {
				t.Fatalf("OpenRequest = %+v", request)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Home open did not reach Backend.Open")
}

func TestHomeNavFiresOnEdgeAndRepeat(t *testing.T) {
	if !homeNavFires(1, true) || !homeNavFires(1, false) {
		t.Fatal("first frame of a press must fire")
	}
	if homeNavFires(2, true) || homeNavFires(homeNavRepeatDelay-1, true) {
		t.Fatal("held key fired before the repeat delay")
	}
	if !homeNavFires(homeNavRepeatDelay, true) {
		t.Fatal("held key did not begin repeating at the delay")
	}
	if !homeNavFires(homeNavRepeatDelay+homeNavRepeatInterval, true) {
		t.Fatal("held key did not keep repeating on the interval")
	}
	if homeNavFires(50, false) {
		t.Fatal("a non-repeating control fired while held")
	}
}

func TestHomeDirectionalNavigationMovesSelection(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.RecentFiles = []string{
		filepath.Join("games", "a.dat"),
		filepath.Join("games", "b.dat"),
		filepath.Join("games", "c.dat"),
	}
	shell.setHomeTab(homeTabRecent)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()

	first := shell.interfaceUI.homeSelectedPath
	if first == "" || first != shell.interfaceUI.homeRowPaths[0] {
		t.Fatalf("initial selection = %q", first)
	}
	shell.interfaceUI.moveHomeSelection(1)
	if got := shell.interfaceUI.homeSelectedPath; got != shell.interfaceUI.homeRowPaths[1] {
		t.Fatalf("after down, selection = %q", got)
	}
	shell.interfaceUI.moveHomeSelection(1)
	shell.interfaceUI.moveHomeSelection(1) // clamps at the last row
	if got := shell.interfaceUI.homeSelectedPath; got != shell.interfaceUI.homeRowPaths[2] {
		t.Fatalf("selection did not clamp to the last row: %q", got)
	}
	shell.interfaceUI.moveHomeSelection(-1)
	if got := shell.interfaceUI.homeSelectedPath; got != shell.interfaceUI.homeRowPaths[1] {
		t.Fatalf("after up, selection = %q", got)
	}
}

func TestHomeSwitchTabWraps(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.setHomeTab(homeTabRecent)
	shell.interfaceUI.switchHomeTab(shell, 1)
	if shell.homeTab != homeTabInstalled {
		t.Fatalf("right from Recent = %q", shell.homeTab)
	}
	shell.interfaceUI.switchHomeTab(shell, -1)
	if shell.homeTab != homeTabRecent {
		t.Fatalf("left back to Recent = %q", shell.homeTab)
	}
	shell.interfaceUI.switchHomeTab(shell, -1)
	if shell.homeTab != homeTabFavorites {
		t.Fatalf("left from Recent should wrap to Favorites, got %q", shell.homeTab)
	}
}

func TestHomeConfirmOpensSelection(t *testing.T) {
	isolateSettledSettings(t)
	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, nil, "")
	want := filepath.Join(t.TempDir(), "pick.dat")
	shell.settings.RecentFiles = []string{want}
	shell.setHomeTab(homeTabRecent)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()

	shell.interfaceUI.activateHomeSelection(shell)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if request.Path != want {
				t.Fatalf("confirm opened %q, want %q", request.Path, want)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("confirm did not open the selected title")
}

func TestToggleFavoriteFromHome(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	path := filepath.Join("games", "star.dat")

	shell.toggleFavoritePath(path)
	if !shell.settings.isFavorite(path) {
		t.Fatal("first toggle did not add the favorite")
	}
	shell.toggleFavoritePath(path)
	if shell.settings.isFavorite(path) {
		t.Fatal("second toggle did not remove the favorite")
	}
}
