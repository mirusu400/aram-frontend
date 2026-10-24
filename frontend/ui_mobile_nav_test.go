package frontend

import (
	"reflect"
	"testing"
)

type mobileQuickActionBackend struct {
	NullBackend
	state     BackendState
	supported map[BackendCommand]bool
}

func (b *mobileQuickActionBackend) State() BackendState {
	return b.state
}

func (b *mobileQuickActionBackend) Supports(command BackendCommand) bool {
	return b.supported[command]
}

func (b *mobileQuickActionBackend) Capability(command BackendCommand) Capability {
	return Capability{Supported: b.Supports(command)}
}

func newMobileQuickActionShell(state FrontendState, input bool) *Shell {
	backend := &mobileQuickActionBackend{
		state: StateReady,
		supported: map[BackendCommand]bool{
			CommandStart:       true,
			CommandPauseResume: true,
			CommandReset:       true,
			CommandFastForward: true,
			CommandLoadState:   true,
			CommandSaveState:   true,
		},
	}
	shell := &Shell{
		backend:  backend,
		state:    state,
		settings: defaultSettings(),
		menus:    defaultMenus(),
	}
	if input {
		shell.input = &InputInfo{DisplayName: "game.zip"}
	}
	return shell
}

func quickCommandIDs(commands []Command) []string {
	ids := make([]string, 0, len(commands))
	for _, command := range commands {
		ids = append(ids, command.ID)
	}
	return ids
}

func TestMobileQuickActionsFollowFrontendState(t *testing.T) {
	tests := []struct {
		name  string
		state FrontendState
		input bool
		want  []string
	}{
		{"empty", FrontendEmpty, false, []string{"file.open", "emu.configure"}},
		{"ready", FrontendReady, true, []string{"emu.start", "emu.load_state", "emu.reset", "emu.configure"}},
		{"running", FrontendRunning, true, []string{"emu.pause", "emu.save_state", "emu.load_state", "emu.fast_forward"}},
		{"paused", FrontendPaused, true, []string{"emu.pause", "emu.load_state", "emu.reset"}},
		{"stopped", FrontendStopped, true, []string{"emu.start", "emu.load_state", "emu.reset", "emu.configure"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shell := newMobileQuickActionShell(test.state, test.input)
			got := quickCommandIDs(mobileQuickCommands(shell))
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("quick actions = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMobileQuickActionsIncludeRecentOnlyWhenAvailable(t *testing.T) {
	shell := newMobileQuickActionShell(FrontendEmpty, false)
	shell.settings.RecentFiles = []RecentEntry{{Path: "recent.zip"}}

	got := quickCommandIDs(mobileQuickCommands(shell))
	want := []string{"file.open", "file.recent", "emu.configure"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quick actions = %v, want %v", got, want)
	}
}

func TestMobileQuickActionsOmitUnsupportedBackendCommands(t *testing.T) {
	shell := newMobileQuickActionShell(FrontendRunning, true)
	backend := shell.backend.(*mobileQuickActionBackend)
	backend.supported = map[BackendCommand]bool{CommandPauseResume: true}

	commands := mobileQuickCommands(shell)
	if got, want := quickCommandIDs(commands), []string{"emu.pause"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("quick actions = %v, want %v", got, want)
	}
	if commands[0].Label != "Pause" || commands[0].Shortcut != "" {
		t.Fatalf("running pause presentation = label %q, shortcut %q", commands[0].Label, commands[0].Shortcut)
	}

	shell.state = FrontendPaused
	commands = mobileQuickCommands(shell)
	if got, want := quickCommandIDs(commands), []string{"emu.pause", "emu.reset"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("paused quick actions = %v, want %v", got, want)
	}
	if commands[0].Label != "Resume" || commands[0].Shortcut != "" {
		t.Fatalf("paused resume presentation = label %q, shortcut %q", commands[0].Label, commands[0].Shortcut)
	}
}

func TestTopChromeHeightUsesOneCompactRowOnTouch(t *testing.T) {
	if got := topChromeHeightForLayout(true); got != mobileAppBarHeight {
		t.Fatalf("touch top chrome = %d, want %d", got, mobileAppBarHeight)
	}
	if got := topChromeHeightForLayout(false); got != menuBarHeight+applicationToolbarHeight {
		t.Fatalf("desktop top chrome = %d", got)
	}
}

func TestMobileDrawerStaysInsideSmallPortraitAndLandscapeViews(t *testing.T) {
	for _, size := range [][2]int{{320, 568}, {568, 320}, {411, 914}} {
		got := mobileDrawerRect(size[0], size[1], 1)
		if got.Min.X != 0 || got.Min.Y != 0 || got.Max.Y != size[1] || got.Max.X > size[0] {
			t.Fatalf("drawer %v escapes %dx%d", got, size[0], size[1])
		}
		minimumUsefulWidth := min(size[0]*3/4, mobileDrawerMaxWidth)
		if got.Dx() < minimumUsefulWidth {
			t.Fatalf("drawer %v is too narrow for %dx%d", got, size[0], size[1])
		}
	}
}

func TestHiddenTouchChromeOpensUnifiedMenuAndReturnsToGame(t *testing.T) {
	shell := &Shell{
		menus:             defaultMenus(),
		activeMenu:        -1,
		touchChromeHidden: true,
		input:             &InputInfo{},
	}
	shell.toggleTouchChromeForLayout(true)
	if shell.touchChromeHidden {
		t.Fatal("opening the mobile menu left the UI hidden")
	}
	if shell.activeMenu != mobileMenuRootIndex(shell.menus) {
		t.Fatalf("active menu = %d, want unified mobile root", shell.activeMenu)
	}
	if !shell.touchMenuImmersive {
		t.Fatal("menu did not remember the immersive game state")
	}

	shell.activeMenu = -1
	shell.finishTouchMenu()
	if !shell.touchChromeHidden || shell.touchMenuImmersive {
		t.Fatalf("dismissed menu = hidden:%t return:%t", shell.touchChromeHidden, shell.touchMenuImmersive)
	}
}

func TestTouchMenuDoesNotHideHomeAfterTitleCloses(t *testing.T) {
	shell := &Shell{
		touchMenuImmersive: true,
		input:              nil,
	}
	shell.finishTouchMenu()
	if shell.touchChromeHidden || shell.touchMenuImmersive {
		t.Fatalf("closed title left immersive state = hidden:%t return:%t",
			shell.touchChromeHidden, shell.touchMenuImmersive)
	}
}
