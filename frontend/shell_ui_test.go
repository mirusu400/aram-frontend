package frontend

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

type integratedWelcomeBackend struct {
	NullBackend
}

func (integratedWelcomeBackend) BackendName() string {
	return "aram-core"
}

type installingWelcomeBackend struct {
	integratedWelcomeBackend
	updates chan ProductUpdate
	err     error
}

func (backend *installingWelcomeBackend) InstallProductUpdate(
	update ProductUpdate,
) error {
	backend.updates <- update
	return backend.err
}

func isolateSettings(t *testing.T) {
	t.Helper()
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	t.Setenv("HOME", temporary)
}

// isolateSettledSettings isolates the settings directory and records Welcome as
// finished, which is the state of every launch after the first. A Shell built
// without it opens the Welcome panel, so a test that expects no panel passes
// only on a machine that already has ARAM settings and fails on a clean one.
func isolateSettledSettings(t *testing.T) {
	t.Helper()
	isolateSettings(t)
	settings := defaultSettings()
	settings.WelcomeCompleted = true
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}
}

func TestShellBuildsEbitenUIDesignSystem(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	if shell.design == nil {
		t.Fatal("ARAM design system was not initialized")
	}
	if shell.interfaceUI == nil || shell.interfaceUI.ui == nil {
		t.Fatal("EbitenUI shell was not initialized")
	}
	if got, want := len(shell.interfaceUI.menuButtons), len(defaultMenus()); got != want {
		t.Fatalf("menu button count = %d, want %d", got, want)
	}
	if shell.design.Type.Body == nil || shell.design.Type.Heading == nil {
		t.Fatal("typography hierarchy is incomplete")
	}
	if shell.design.Components.Surface == nil || shell.design.Components.PrimaryButton.Image == nil {
		t.Fatal("component styles are incomplete")
	}
}

func TestFirstIntegratedLaunchOpensWelcomeModal(t *testing.T) {
	isolateSettings(t)
	shell := NewShell(integratedWelcomeBackend{}, nil, "")
	if shell.panel == nil || shell.panel.Kind != "welcome" {
		t.Fatalf("first integrated launch panel = %#v", shell.panel)
	}
	view := shell.interfaceUI
	view.sync(shell)
	if view.panelWindow == nil || !view.panelWindow.Modal {
		t.Fatal("Welcome panel was not rendered as a modal")
	}
	if view.welcomeStableButton == nil ||
		view.welcomeNightlyButton == nil ||
		view.welcomeLaterButton == nil {
		t.Fatal("Welcome modal omitted a channel action")
	}
}

func TestWelcomeChannelSelectionPersistsAndDoesNotDownloadCoreTools(t *testing.T) {
	isolateSettings(t)
	downloader := &fakeUpdateDownloader{
		requests: make(chan updateDownload, 1),
	}
	shell := NewShell(integratedWelcomeBackend{}, nil, "")
	shell.updater = downloader

	shell.completeWelcome(updateChannelNightly)
	if !shell.settings.WelcomeCompleted ||
		shell.settings.UpdateChannel != string(updateChannelNightly) ||
		shell.panel != nil {
		t.Fatalf("completed Welcome state = settings:%#v panel:%#v", shell.settings, shell.panel)
	}
	select {
	case request := <-downloader.requests:
		t.Fatalf("Welcome unexpectedly downloaded a component: %#v", request)
	default:
	}
	reloaded := loadSettings()
	if !reloaded.WelcomeCompleted ||
		reloaded.UpdateChannel != string(updateChannelNightly) {
		t.Fatalf("reloaded Welcome settings = %#v", reloaded)
	}
}

func TestWelcomeNightlyDownloadsInstallsAndRestartsIntegratedProduct(t *testing.T) {
	isolateSettings(t)
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
	}
	downloader := &fakeUpdateDownloader{
		download: updateDownload{
			Component: updateComponentProduct,
			Channel:   updateChannelNightly,
			Version:   "Nightly 10261b2",
			Path:      filepath.Join(t.TempDir(), "aram-windows-amd64.zip"),
		},
		requests: make(chan updateDownload, 1),
	}
	shell := NewShell(backend, nil, "")
	shell.updater = downloader

	shell.completeWelcome(updateChannelNightly)
	if !shell.welcomeInstalling || shell.settings.WelcomeCompleted ||
		shell.panel == nil {
		t.Fatalf(
			"installing Welcome state = installing:%t settings:%#v panel:%#v",
			shell.welcomeInstalling,
			shell.settings,
			shell.panel,
		)
	}
	select {
	case request := <-downloader.requests:
		if request.Component != updateComponentProduct ||
			request.Channel != updateChannelNightly {
			t.Fatalf("Welcome update request = %#v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("Welcome did not request the integrated Nightly product")
	}
	var result updateResult
	select {
	case result = <-shell.updateResults:
	case <-time.After(time.Second):
		t.Fatal("Welcome product download did not complete")
	}
	shell.consumeUpdateResult(result)

	select {
	case installed := <-backend.updates:
		if installed.Channel != string(updateChannelNightly) ||
			installed.Version != downloader.download.Version ||
			installed.ArchivePath != downloader.download.Path {
			t.Fatalf("installed product = %#v", installed)
		}
	case <-time.After(time.Second):
		t.Fatal("Welcome did not install the downloaded product")
	}
	if !shell.settings.WelcomeCompleted || shell.panel != nil ||
		!shell.quitting || shell.welcomeInstalling {
		t.Fatalf(
			"installed Welcome state = settings:%#v panel:%#v quitting:%t installing:%t",
			shell.settings,
			shell.panel,
			shell.quitting,
			shell.welcomeInstalling,
		)
	}
}

func TestWelcomeStableUsesBundledBuildBeforeFirstStableRelease(t *testing.T) {
	isolateSettings(t)
	backend := &installingWelcomeBackend{
		updates: make(chan ProductUpdate, 1),
	}
	downloader := &fakeUpdateDownloader{
		err: &releaseNotFoundError{
			repository: "aram-emu",
			channel:    updateChannelStable,
		},
		requests: make(chan updateDownload, 1),
	}
	shell := NewShell(backend, nil, "")
	shell.updater = downloader

	shell.completeWelcome(updateChannelStable)
	select {
	case <-downloader.requests:
	case <-time.After(time.Second):
		t.Fatal("Welcome did not check for a Stable product")
	}
	select {
	case result := <-shell.updateResults:
		shell.consumeUpdateResult(result)
	case <-time.After(time.Second):
		t.Fatal("Stable lookup did not complete")
	}

	if !shell.settings.WelcomeCompleted || shell.panel != nil ||
		shell.quitting || shell.welcomeInstalling {
		t.Fatalf(
			"Stable fallback state = settings:%#v panel:%#v quitting:%t installing:%t",
			shell.settings,
			shell.panel,
			shell.quitting,
			shell.welcomeInstalling,
		)
	}
	select {
	case installed := <-backend.updates:
		t.Fatalf("Stable fallback unexpectedly installed %#v", installed)
	default:
	}
}

func TestWelcomeDecideLaterKeepsFirstRunIncomplete(t *testing.T) {
	isolateSettings(t)
	shell := NewShell(integratedWelcomeBackend{}, nil, "")
	shell.dismissWelcome()
	if shell.settings.WelcomeCompleted || shell.panel != nil {
		t.Fatalf("deferred Welcome state = settings:%#v panel:%#v", shell.settings, shell.panel)
	}
}

func TestWelcomeModalAndActionsStayInsideResponsiveViewport(t *testing.T) {
	for _, size := range [][2]int{{390, 844}, {480, 480}, {720, 540}, {960, 720}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			isolateSettings(t)
			shell := NewShell(integratedWelcomeBackend{}, nil, "")
			shell.Layout(size[0], size[1])
			view := shell.interfaceUI
			view.sync(shell)
			view.ui.Update()

			windowRect := view.panelWindow.GetContainer().GetWidget().Rect
			viewport := image.Rect(0, 0, size[0], size[1])
			if !windowRect.In(viewport) {
				t.Fatalf("Welcome modal %v outside viewport %v", windowRect, viewport)
			}
			buttons := []*widget.Button{
				view.welcomeStableButton,
				view.welcomeNightlyButton,
				view.welcomeLaterButton,
			}
			for index, button := range buttons {
				rect := button.GetWidget().Rect
				if !rect.In(windowRect) {
					t.Errorf("Welcome action %d at %v outside modal %v", index, rect, windowRect)
				}
				for previous := 0; previous < index; previous++ {
					if rect.Overlaps(buttons[previous].GetWidget().Rect) {
						t.Errorf(
							"Welcome actions %d and %d overlap: %v / %v",
							previous,
							index,
							buttons[previous].GetWidget().Rect,
							rect,
						)
					}
				}
			}
		})
	}
}

func TestEbitenUIDropdownPreservesStableCommandIDs(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	view := shell.interfaceUI
	view.openMenu(0)
	t.Cleanup(view.closeMenu)

	if shell.activeMenu != 0 || view.menuIndex != 0 {
		t.Fatalf("active menu = shell:%d ui:%d, want 0", shell.activeMenu, view.menuIndex)
	}
	if view.menuWindow == nil || !view.menuWindow.Modal {
		t.Fatal("dropdown does not block click-through into the guest surface")
	}
	for _, command := range shell.menus[0].Commands {
		if view.commandButtons[command.ID] == nil {
			t.Errorf("dropdown omitted stable command ID %q", command.ID)
		}
	}
}

func TestEbitenUIModalTracksPanelLifecycle(t *testing.T) {
	previousVersion := BuildVersion
	BuildVersion = "Nightly-7654321"
	t.Cleanup(func() { BuildVersion = previousVersion })

	shell := NewShell(NullBackend{}, nil, "")
	shell.showAbout()
	if !strings.Contains(
		strings.Join(shell.panel.Lines, "\n"),
		"Nightly 7654321",
	) {
		t.Fatalf("about lines omit current version: %#v", shell.panel.Lines)
	}
	shell.interfaceUI.sync(shell)
	if shell.interfaceUI.panelWindow == nil || !shell.interfaceUI.panelWindow.Modal {
		t.Fatal("about panel did not open as an EbitenUI modal")
	}

	shell.panel = nil
	shell.interfaceUI.sync(shell)
	if shell.interfaceUI.panelWindow != nil {
		t.Fatal("panel window remained open after the panel state was cleared")
	}
}

func TestConfigureCommandOpensCategorySettingsWindow(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.dispatchCommand("emu.configure")
	shell.interfaceUI.sync(shell)

	if shell.panel == nil || shell.panel.Kind != "settings" {
		t.Fatalf("configure panel = %#v", shell.panel)
	}
	if shell.interfaceUI.panelWindow == nil || !shell.interfaceUI.panelWindow.Modal {
		t.Fatal("configure command did not open a modal settings window")
	}
	if shell.interfaceUI.settingsSection != "General" {
		t.Fatalf("initial settings section = %q", shell.interfaceUI.settingsSection)
	}
	for _, id := range []string{"file.open", "emu.start", "emu.pause", "emu.stop", "emu.configure"} {
		if shell.interfaceUI.toolbarButtons[id] == nil {
			t.Errorf("application toolbar omitted %q", id)
		}
	}
}

func TestControllerCommandOpensControlsSettings(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.dispatchCommand("tools.controller")
	shell.interfaceUI.sync(shell)

	if shell.panel == nil || shell.panel.Kind != "settings" {
		t.Fatalf("controller panel = %#v", shell.panel)
	}
	if shell.settingsSection != "Controls" || shell.interfaceUI.settingsSection != "Controls" {
		t.Fatalf(
			"controller settings section = shell:%q ui:%q",
			shell.settingsSection,
			shell.interfaceUI.settingsSection,
		)
	}
	if shell.interfaceUI.panelWindow == nil || !shell.interfaceUI.panelWindow.Modal {
		t.Fatal("controller command did not open the settings modal")
	}
}

func TestAudioCommandOpensAudioSettings(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.dispatchCommand("tools.audio")
	shell.interfaceUI.sync(shell)

	if shell.panel == nil || shell.panel.Kind != "settings" {
		t.Fatalf("audio panel = %#v", shell.panel)
	}
	if shell.settingsSection != "Audio" || shell.interfaceUI.settingsSection != "Audio" {
		t.Fatalf(
			"audio settings section = shell:%q ui:%q",
			shell.settingsSection,
			shell.interfaceUI.settingsSection,
		)
	}
}

func TestUpdatesSettingsSectionRendersAllPublicComponents(t *testing.T) {
	previousVersion := BuildVersion
	BuildVersion = "v1.2.3"
	t.Cleanup(func() { BuildVersion = previousVersion })

	shell := NewShell(NullBackend{}, nil, "")
	shell.dispatchCommand("help.updates")
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(logicalWidth, logicalHeight))

	if shell.interfaceUI.settingsSection != "Updates" {
		t.Fatalf("settings section = %q", shell.interfaceUI.settingsSection)
	}
	if shell.interfaceUI.panelWindow == nil {
		t.Fatal("updates settings window was not created")
	}
	rows := updateSettingsRowModels(shell)
	if len(rows) == 0 ||
		rows[0].label != "Current version" ||
		rows[0].value != "v1.2.3" ||
		rows[0].action != nil {
		t.Fatalf("current version row = %#v", rows)
	}
	if _, ok := updateInfo(updateComponentProduct); !ok {
		t.Fatal("aram-emu product update is missing")
	}
	if _, ok := updateInfo(updateComponentCore); !ok {
		t.Fatal("aram-core update is missing")
	}
	if _, ok := updateInfo(updateComponentFrontend); !ok {
		t.Fatal("aram-frontend update is missing")
	}
}

func TestResponsiveModalStaysInsideViewport(t *testing.T) {
	for _, size := range [][2]int{{390, 844}, {720, 540}, {960, 720}, {1600, 900}} {
		width, height := size[0], size[1]
		rect := centeredWindowRect(width, height, 740, 580)
		if rect.Min.X < 0 || rect.Min.Y < 0 || rect.Max.X > width || rect.Max.Y > height {
			t.Fatalf("modal %v outside %dx%d", rect, width, height)
		}
		if rect.Min.X != (width-rect.Dx())/2 || rect.Min.Y != (height-rect.Dy())/2 {
			t.Fatalf("modal %v is not centered in %dx%d", rect, width, height)
		}
	}
}

func TestARAMRoundedRectHitModel(t *testing.T) {
	if !insideRoundedRect(5, 5, 0, 0, 11, 11, 4) {
		t.Fatal("rounded rectangle excludes its center")
	}
	if insideRoundedRect(0, 0, 0, 0, 11, 11, 4) {
		t.Fatal("rounded rectangle includes a clipped corner")
	}
	if !insideRoundedRect(0, 0, 0, 0, 11, 11, 0) {
		t.Fatal("square rectangle excludes its corner")
	}
}

func TestAppearanceSwitchRebuildsNeutralSquareDesign(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.ThemeMode = "light"
	shell.syncDesignSystem()
	lightUI := shell.interfaceUI
	if shell.design.Mode != "light" {
		t.Fatalf("initial design mode = %q", shell.design.Mode)
	}
	if shell.design.Radius.Small != 0 ||
		shell.design.Radius.Medium != 0 ||
		shell.design.Radius.Large != 0 ||
		shell.design.Radius.Pill != 0 {
		t.Fatalf("design still uses rounded corners: %#v", shell.design.Radius)
	}

	shell.settings.ThemeMode = "dark"
	shell.syncDesignSystem()
	if shell.design.Mode != "dark" {
		t.Fatalf("switched design mode = %q", shell.design.Mode)
	}
	if shell.interfaceUI == lightUI {
		t.Fatal("theme switch did not rebuild EbitenUI component styles")
	}
	canvas := shell.design.Palette.Canvas
	if canvas.R != canvas.G || canvas.G != canvas.B {
		t.Fatalf("dark canvas is not neutral: %#v", canvas)
	}
}

func TestEbitenUIDrawsPersistentApplicationChrome(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	if err := shell.Update(); err != nil {
		t.Fatalf("Update returned %v", err)
	}
	screen := ebiten.NewImage(logicalWidth, logicalHeight)
	shell.Draw(screen)
}

func TestResponsiveSettingsRenderAtCompactAndLargeSizes(t *testing.T) {
	for _, size := range [][2]int{{390, 844}, {720, 540}, {1440, 900}} {
		width, height := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
			shell := NewShell(NullBackend{}, nil, "")
			shell.Layout(width, height)
			shell.openSettingsSection("Bindings")
			shell.interfaceUI.sync(shell)
			shell.interfaceUI.ui.Update()

			screen := ebiten.NewImage(width, height)
			shell.Draw(screen)
			if shell.interfaceUI.panelWindow == nil {
				t.Fatal("responsive settings window was not created")
			}
			if shell.interfaceUI.compact != (width < 820 || height < 620) {
				t.Fatalf("compact mode = %t", shell.interfaceUI.compact)
			}
		})
	}
}

func TestOpenRecentUsesScrollableFilenameFirstList(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.RecentFiles = make([]string, recentFileLimit)
	for index := range shell.settings.RecentFiles {
		shell.settings.RecentFiles[index] = filepath.Join(
			`C:\very\long\archive\directory\with\identical\prefixes`,
			fmt.Sprintf("distinct-game-%02d.dat", index),
		)
	}

	shell.chooseRecent()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()

	if shell.dialogOpen {
		t.Fatal("Open Recent still opened a platform-owned dialog")
	}
	if shell.panel == nil || shell.panel.Kind != "recent" {
		t.Fatalf("Open Recent panel = %#v", shell.panel)
	}
	if shell.interfaceUI.panelWindow == nil || shell.interfaceUI.recentList == nil {
		t.Fatal("scrollable recent-input list was not created")
	}
	if got := len(shell.interfaceUI.recentList.Entries()); got != recentFileLimit {
		t.Fatalf("recent list entries = %d, want %d", got, recentFileLimit)
	}
	first := shell.settings.RecentFiles[0]
	label := recentEntryLabel(first, 70)
	if !strings.HasPrefix(label, "distinct-game-00.dat") {
		t.Fatalf("recent entry does not lead with its filename: %q", label)
	}
	details := recentPathDetails(first, 26, 8)
	if !strings.Contains(strings.ReplaceAll(details, "\n", ""), "distinct-game-00.dat") {
		t.Fatalf("selected-path details omit the filename: %q", details)
	}
}

func TestOpenRecentListRendersAtMinimumWindowSize(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.Layout(720, 540)
	for index := 0; index < recentFileLimit; index++ {
		shell.settings.RecentFiles = append(
			shell.settings.RecentFiles,
			filepath.Join("archive", fmt.Sprintf("game-%02d.dat", index)),
		)
	}
	shell.chooseRecent()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(720, 540))

	if shell.interfaceUI.panelWindow == nil || shell.interfaceUI.recentList == nil {
		t.Fatal("compact Open Recent modal did not render its list")
	}
}

func TestInteractiveToolFormRendersAtCompactSize(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.Layout(390, 844)
	shell.panel = &Panel{
		Kind:  "tool",
		Tool:  ToolMemory,
		Title: "Memory Search",
		Lines: []string{"Checked searches execute behind the backend boundary."},
		Fields: []ToolField{{
			ID:          "value",
			Label:       "Value",
			Placeholder: "0x1234",
		}},
		FieldValues: map[string]string{"value": "42"},
		Actions: []ToolAction{{
			ID:      "search",
			Label:   "Search",
			Enabled: true,
		}},
	}
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(390, 844))
	if shell.interfaceUI.panelWindow == nil {
		t.Fatal("interactive tool window was not created")
	}
}

func TestIssueReportFormRendersAtCompactSize(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.Layout(390, 844)
	shell.openIssueTracker()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(390, 844))

	if shell.interfaceUI.panelWindow == nil {
		t.Fatal("compact issue report window was not created")
	}
	if shell.panel == nil ||
		shell.panel.Kind != "issue-report" ||
		len(shell.panel.Fields) != 5 {
		t.Fatalf("compact issue report panel = %#v", shell.panel)
	}
	dropdown := shell.interfaceUI.panelDropdowns["repository"]
	if dropdown == nil {
		t.Fatal("repository dropdown was not created")
	}
	core := shell.panel.Fields[3].Options[2]
	dropdown.SetSelectedEntry(core)
	shell.interfaceUI.ui.Update()
	if shell.panel.FieldValues["repository"] != "aram-core" {
		t.Fatalf(
			"repository selection = %q",
			shell.panel.FieldValues["repository"],
		)
	}
	screenshot := shell.interfaceUI.panelCheckboxes[issueReportScreenshotField]
	if screenshot == nil || screenshot.State() != widget.WidgetChecked {
		t.Fatalf("screenshot checkbox = %#v", screenshot)
	}
	screenshot.Click()
	shell.interfaceUI.ui.Update()
	if shell.panel.FieldValues[issueReportScreenshotField] != "false" {
		t.Fatalf(
			"screenshot selection = %q, state = %v, disabled = %t",
			shell.panel.FieldValues[issueReportScreenshotField],
			screenshot.State(),
			screenshot.GetWidget().Disabled,
		)
	}
}

func TestSubmittedReportHistoryRendersScrollableDropdownAtCompactSize(
	t *testing.T,
) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.Layout(390, 844)
	for index := 0; index < issueReportHistoryLimit; index++ {
		shell.settings.rememberIssueReport(IssueReportRecord{
			ReportID: fmt.Sprintf(
				"%08x-1111-4111-8111-111111111111",
				index,
			),
			IssueURL: fmt.Sprintf(
				"https://github.com/mirusu400/aram-frontend/issues/%d",
				index+1,
			),
			Capability: "aram_rpt_" + strings.Repeat("A", 43),
			Repository: "aram-frontend",
			Situation:  fmt.Sprintf("Report %d", index),
			CreatedAt:  time.Unix(int64(index+1), 0).UTC(),
		})
	}
	shell.openIssueReportHistory()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(390, 844))

	dropdown := shell.interfaceUI.panelDropdowns[issueReportHistoryField]
	if shell.interfaceUI.panelWindow == nil || dropdown == nil {
		t.Fatal("compact submitted-report history was not created")
	}
	if got := len(shell.panel.Fields[0].Options); got != issueReportHistoryLimit {
		t.Fatalf(
			"submitted-report options = %d, want %d",
			got,
			issueReportHistoryLimit,
		)
	}
	selected := shell.panel.Fields[0].Options[5]
	dropdown.SetSelectedEntry(selected)
	shell.interfaceUI.ui.Update()
	if shell.panel.FieldValues[issueReportHistoryField] != selected.Value {
		t.Fatalf(
			"selected report = %q, want %q",
			shell.panel.FieldValues[issueReportHistoryField],
			selected.Value,
		)
	}
}
