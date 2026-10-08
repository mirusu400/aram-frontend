package frontend

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ebitenui/ebitenui/widget"
)

func TestHomeSingleClickNeverLaunchesPreselectedRow(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	path := filepath.Join(t.TempDir(), "game.dat")
	shell.settings.RecentFiles = recentEntriesFromPaths(path)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.onHomeRowClicked(shell, path)
	if shell.loading {
		t.Fatal("a single click on the automatically selected row launched the game")
	}
}

func TestHomeDesktopClickTimingAndRowIdentity(t *testing.T) {
	for _, test := range []struct {
		name       string
		secondPath string
		delay      time.Duration
		launch     bool
	}{
		{"double click", "a.dat", 200 * time.Millisecond, true},
		{"slow second click", "a.dat", time.Second, false},
		{"different row", "b.dat", 200 * time.Millisecond, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettledSettings(t)
			shell := NewShell(NullBackend{}, nil, "")
			view := shell.interfaceUI
			now := time.Unix(10, 0)
			view.handleHomeRowClick(shell, "a.dat", false, now)
			if shell.loading {
				t.Fatal("first click launched")
			}
			view.handleHomeRowClick(shell, test.secondPath, false, now.Add(test.delay))
			if shell.loading != test.launch {
				t.Fatalf("launch = %t, want %t", shell.loading, test.launch)
			}
		})
	}
}

func TestHomeTouchTapOpensUnselectedRow(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.interfaceUI.homeSelectedPath = "a.dat"
	shell.interfaceUI.handleHomeRowClick(shell, "b.dat", true, time.Unix(10, 0))
	if !shell.loading || shell.interfaceUI.homeSelectedPath != "b.dat" {
		t.Fatal("one tap did not select and open the tapped game")
	}
}

func TestHomeEmptyActionsReachOrdinaryFilePicker(t *testing.T) {
	isolateSettledSettings(t)
	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	path := filepath.Join(t.TempDir(), "game.dat")
	shell := NewShell(backend, fixedPicker{path: path}, "")
	empty := shell.interfaceUI.homeEmptyContent(shell, homeTabRecent, nil, 320)
	var open *widget.Button
	for _, child := range empty.Children() {
		if button, ok := child.(*widget.Button); ok && button.GetWidget().CustomData == "file.open" {
			open = button
		}
	}
	if open == nil {
		t.Fatal("empty launcher has no file-open action")
	}
	open.Click()
	shell.interfaceUI.ui.Update()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if request.Path != path {
				t.Fatalf("opened %q, want %q", request.Path, path)
			}
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
	t.Fatal("empty-state action did not reach the ordinary open pipeline")
}

func TestHomeFavoriteLabelTracksSelectionAndToggle(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.RecentFiles = recentEntriesFromPaths("a.dat", "b.dat")
	shell.toggleFavoritePath("b.dat")
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	view := shell.interfaceUI
	if got := view.homeFavButton.Text().Label; got != shell.tr("Add favorite") {
		t.Fatalf("initial label = %q", got)
	}
	view.moveHomeSelection(1)
	if got := view.homeFavButton.Text().Label; got != shell.tr("Remove favorite") {
		t.Fatalf("favorite selection label = %q", got)
	}
	shell.toggleFavoritePath(view.homeSelectedPath)
	view.sync(shell)
	view.ui.Update()
	if got := view.homeFavButton.Text().Label; got != shell.tr("Add favorite") {
		t.Fatalf("removed favorite label = %q", got)
	}
}
