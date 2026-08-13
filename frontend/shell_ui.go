package frontend

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"github.com/ebitenui/ebitenui"
	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

const (
	menuBarHeight            = 36
	applicationToolbarHeight = 48
	statusBarHeight          = 28
	settingsNavWidth         = 168
)

type shellUI struct {
	owner                *Shell
	ui                   *ebitenui.UI
	design               *ARAMDesignSystem
	menuButtons          []*widget.Button
	menuWindow           *widget.Window
	menuIndex            int
	commandButtons       map[string]*widget.Button
	toolbarButtons       map[string]*widget.Button
	toolbarTitle         *widget.Text
	buildStampText       *widget.Text
	statusText           *widget.Text
	statusMeta           *widget.Text
	panelWindow          *widget.Window
	panelSignature       string
	panelDropdowns       map[string]*widget.ListComboButton
	panelCheckboxes      map[string]*widget.Checkbox
	panelTextInputs      map[string]*imeTextInput
	settingsSection      string
	bindingDevice        bindingDevice
	scrim                *widget.Container
	viewportWidth        int
	viewportHeight       int
	compact              bool
	settingsScroll       *widget.ScrollContainer
	settingsSliders      []settingsSliderBinding
	settingsDropdowns    []settingsDropdownBinding
	settingsOffsets      map[string]float64
	recentList           *widget.List
	welcomeStableButton  *widget.Button
	welcomeNightlyButton *widget.Button
	welcomeLaterButton   *widget.Button
}

func newShellUI(shell *Shell, design *ARAMDesignSystem) *shellUI {
	view := &shellUI{
		owner:           shell,
		design:          design,
		menuIndex:       -1,
		commandButtons:  make(map[string]*widget.Button),
		toolbarButtons:  make(map[string]*widget.Button),
		settingsSection: shell.settingsSection,
		bindingDevice:   bindingDeviceKeyboard,
		settingsOffsets: make(map[string]float64),
	}

	root := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewAnchorLayout()))
	topBar := view.buildTopBar(shell)
	toolbar := view.buildApplicationToolbar(shell)
	statusBar := view.buildStatusBar()
	view.scrim = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.Scrim),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true,
			StretchVertical:   true,
		})),
	)
	view.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)

	root.AddChild(topBar, toolbar, statusBar, view.scrim)
	view.ui = &ebitenui.UI{
		Container:           root,
		DisableDefaultFocus: false,
		PrimaryTheme:        design.Theme,
	}
	view.sync(shell)
	return view
}

func (u *shellUI) buildTopBar(shell *Shell) *widget.Container {
	design := u.design
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.MenuBar),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
			}),
			widget.WidgetOpts.MinSize(0, menuBarHeight),
		),
	)

	menuRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Padding(&widget.Insets{
				Left:   design.Space.M,
				Top:    design.Space.XS,
				Bottom: design.Space.XS,
			}),
			widget.RowLayoutOpts.Spacing(design.Space.XS),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
		})),
	)
	widths := menuWidths(shell.menus)
	for index, menu := range shell.menus {
		menuIndex := index
		button := design.button(
			shell.tr(menu.Label),
			design.Components.MenuButton,
			design.Type.Strong,
			widths[index],
			design.Components.MenuButton.MinHeight,
			widget.TextPositionCenter,
			func() { u.toggleMenu(menuIndex) },
		)
		button.GetWidget().CustomData = "menu:" + menu.Label
		u.menuButtons = append(u.menuButtons, button)
		menuRow.AddChild(button)
	}
	bar.AddChild(menuRow)
	if stamp := currentNightlyBuildStamp(); stamp != "" && !platformUsesTouchLayout() {
		u.buildStampText = design.text(
			shell.trf("Nightly build %s", stamp),
			design.Type.Caption,
			design.Palette.TextMuted,
			widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionEnd,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				Padding:            &widget.Insets{Right: design.Space.M},
			},
		)
		bar.AddChild(u.buildStampText)
	}
	return bar
}

func (u *shellUI) buildApplicationToolbar(shell *Shell) *widget.Container {
	design := u.design
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.Toolbar),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
				Padding:            &widget.Insets{Top: menuBarHeight},
			}),
			widget.WidgetOpts.MinSize(0, applicationToolbarHeight),
		),
	)

	actions := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Padding(&widget.Insets{
				Left:   design.Space.M,
				Top:    design.Space.S,
				Bottom: design.Space.S,
			}),
			widget.RowLayoutOpts.Spacing(design.Space.XS),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
		})),
	)

	addAction := func(id, label string, width int) {
		commandID := id
		button := design.button(
			label,
			design.Components.SubtleButton,
			design.Type.Strong,
			width,
			32,
			widget.TextPositionCenter,
			func() { shell.dispatchCommand(commandID) },
		)
		button.GetWidget().CustomData = commandID
		u.toolbarButtons[commandID] = button
		actions.AddChild(button)
	}
	addSeparator := func() {
		actions.AddChild(widget.NewContainer(
			widget.ContainerOpts.BackgroundImage(design.Components.Divider),
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{
					Position: widget.RowLayoutPositionCenter,
				}),
				widget.WidgetOpts.MinSize(1, 22),
			),
		))
	}

	addAction("file.open", shell.tr("Open"), 68)
	addSeparator()
	addAction("emu.start", shell.tr("Start"), 64)
	addAction("emu.pause", shell.tr("Pause"), 64)
	addAction("emu.stop", shell.tr("Stop"), 60)
	addAction("emu.reset", shell.tr("Reset"), 62)
	addSeparator()
	addAction("emu.configure", shell.tr("Settings"), 82)

	u.toolbarTitle = design.text(
		"",
		design.Type.Body,
		design.Palette.TextMuted,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionEnd,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Right: design.Space.L},
		},
	)
	bar.AddChild(actions, u.toolbarTitle)
	return bar
}

func (u *shellUI) buildStatusBar() *widget.Container {
	design := u.design
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.StatusBar),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionEnd,
				StretchHorizontal:  true,
			}),
			widget.WidgetOpts.MinSize(0, statusBarHeight),
		),
	)
	u.statusText = design.text(
		"",
		design.Type.Caption,
		design.Palette.TextMuted,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.L},
		},
	)
	u.statusMeta = design.text(
		"",
		design.Type.Caption,
		design.Palette.TextDisabled,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionEnd,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Right: design.Space.L},
		},
	)
	bar.AddChild(u.statusText, u.statusMeta)
	return bar
}

func (u *shellUI) sync(shell *Shell) {
	width, height := shell.viewportSize()
	if width != u.viewportWidth || height != u.viewportHeight {
		u.viewportWidth = width
		u.viewportHeight = height
		u.compact = width < 820 || height < 620
		u.panelSignature = ""
		u.closeMenu()
	}
	u.toolbarTitle.GetWidget().SetVisibility(visibility(width >= 760))
	if u.buildStampText != nil {
		u.buildStampText.GetWidget().SetVisibility(visibility(width >= 700))
	}
	u.statusMeta.GetWidget().SetVisibility(visibility(width >= 700))
	for id, button := range u.toolbarButtons {
		visible := true
		if width < 620 && (id == "emu.stop" || id == "emu.reset") {
			visible = false
		}
		if width < 480 && id == "emu.pause" {
			visible = false
		}
		button.GetWidget().SetVisibility(visibility(visible))
	}
	statusLimit := max(24, min(92, (width-32)/7))
	u.statusText.Label = shorten(shell.status, statusLimit)
	// Show the achieved speed (e.g. "1x (98%)") rather than only the requested
	// setting, so a title running below handset speed is visible in the
	// always-on status bar without opening settings, which pauses the guest.
	u.statusMeta.Label = fmt.Sprintf(
		"%s  •  %s  •  %s",
		strings.ToUpper(shell.tr(stateValueLabel(string(shell.backend.State())))),
		shell.speedSettingValue(),
		strings.ToUpper(shell.tr(settingValueLabel(shell.settings.Filter))),
	)
	if shell.input == nil {
		u.toolbarTitle.Label = shell.tr("No title loaded")
	} else {
		u.toolbarTitle.Label = shorten(shell.input.DisplayName, 52)
	}

	for id, button := range u.toolbarButtons {
		command, found := shell.findCommand(id)
		button.GetWidget().Disabled = !found || !command.IsEnabled(shell)
	}

	if shell.activeMenu != u.menuIndex {
		if shell.activeMenu < 0 {
			u.closeMenu()
		} else {
			u.openMenu(shell.activeMenu)
		}
	}
	for id, button := range u.commandButtons {
		if command, found := shell.findCommand(id); found {
			button.SetText(commandButtonLabel(command, shell))
			button.GetWidget().Disabled = !command.IsEnabled(shell)
		}
	}
	u.syncPanel(shell)
}

func (u *shellUI) toggleMenu(index int) {
	if u.menuIndex == index {
		u.owner.activeMenu = -1
		u.closeMenu()
		return
	}
	u.openMenu(index)
}

func (u *shellUI) openMenu(index int) {
	if index < 0 || index >= len(u.owner.menus) {
		return
	}
	u.closeMenu()
	u.owner.activeMenu = index
	u.menuIndex = index
	u.syncMenuButtonStyles(index)

	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.Dropdown),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(design.Space.S)),
			widget.RowLayoutOpts.Spacing(design.Space.XS),
		)),
	)
	u.commandButtons = make(map[string]*widget.Button)
	for _, command := range u.owner.menus[index].Commands {
		commandID := command.ID
		button := design.button(
			commandButtonLabel(command, u.owner),
			design.Components.CommandButton,
			design.Type.Body,
			dropdownWidth-design.Space.L,
			design.Components.CommandButton.MinHeight,
			widget.TextPositionStart,
			func() {
				u.owner.activeMenu = -1
				u.closeMenu()
				u.owner.dispatchCommand(commandID)
			},
		)
		button.GetWidget().Disabled = !command.IsEnabled(u.owner)
		button.GetWidget().CustomData = commandID
		u.commandButtons[commandID] = button
		contents.AddChild(button)
	}

	startX := menuUIStartX(u.owner.menus, index, design)
	height := design.Space.L +
		len(u.owner.menus[index].Commands)*(design.Components.CommandButton.MinHeight+design.Space.XS)
	var window *widget.Window
	window = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.CloseMode(widget.CLICK_OUT),
		widget.WindowOpts.Location(image.Rect(
			startX,
			menuBarHeight+design.Space.XS,
			startX+dropdownWidth,
			menuBarHeight+design.Space.XS+height,
		)),
		widget.WindowOpts.ClosedHandler(func(*widget.WindowClosedEventArgs) {
			if u.menuWindow == window {
				u.menuWindow = nil
				u.menuIndex = -1
				u.owner.activeMenu = -1
				u.commandButtons = make(map[string]*widget.Button)
				u.syncMenuButtonStyles(-1)
			}
		}),
	)
	u.menuWindow = window
	u.ui.AddWindow(window)
}

func (u *shellUI) closeMenu() {
	window := u.menuWindow
	u.menuWindow = nil
	u.menuIndex = -1
	u.commandButtons = make(map[string]*widget.Button)
	u.syncMenuButtonStyles(-1)
	if window != nil {
		window.Close()
	}
}

func (u *shellUI) syncMenuButtonStyles(active int) {
	base := u.design.Components.MenuButton.Image
	for index, button := range u.menuButtons {
		if index != active {
			button.SetImage(base)
			continue
		}
		button.SetImage(&widget.ButtonImage{
			Idle:         base.Pressed,
			Hover:        base.PressedHover,
			Pressed:      base.Pressed,
			PressedHover: base.PressedHover,
			Disabled:     base.Disabled,
		})
	}
}

func commandButtonLabel(command Command, shell *Shell) string {
	label := command.DisplayLabel(shell)
	if command.Shortcut == "" {
		return label
	}
	return label + "    ·    " + command.Shortcut
}

func menuUIStartX(menus []Menu, index int, design *ARAMDesignSystem) int {
	x := design.Space.L
	widths := menuWidths(menus)
	for current := 0; current < index; current++ {
		x += widths[current] + design.Space.XS
	}
	return x
}

func (u *shellUI) syncPanel(shell *Shell) {
	if shell.panel == nil {
		u.closePanel()
		return
	}
	if shell.panel.Kind == "settings" {
		u.syncSettingsPanel(shell)
		return
	}
	if shell.panel.Kind == "welcome" {
		u.syncWelcomePanel(shell)
		return
	}
	if shell.panel.Kind == "recent" {
		u.syncRecentPanel(shell)
		return
	}
	if (shell.panel.Kind == "tool" ||
		shell.panel.Kind == "issue-report") &&
		(len(shell.panel.Fields) > 0 || len(shell.panel.Actions) > 0) {
		u.syncInteractiveToolPanel(shell)
		return
	}
	wrapWidth := max(28, min(78, (u.viewportWidth-72)/7))
	lineLimit := max(8, min(29, (u.viewportHeight-150)/16))
	lines := wrapPanelLines(shell.trLines(shell.panelLines()), wrapWidth, lineLimit)
	signature := fmt.Sprintf(
		"%dx%d\x00%s\x00%s\x00%s",
		u.viewportWidth,
		u.viewportHeight,
		shell.tr(shell.panel.Title),
		strings.Join(lines, "\x00"),
		shell.tr(shell.panelFooter()),
	)
	if signature == u.panelSignature && u.panelWindow != nil {
		return
	}
	u.closePanel()
	u.panelSignature = signature
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)

	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.SurfaceRaised),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	body := widget.NewText(
		widget.TextOpts.Text(strings.Join(lines, "\n"), design.Type.Body, design.Palette.TextMuted),
		widget.TextOpts.MaxWidth(680),
		widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			Padding: &widget.Insets{
				Left:   design.Space.XL,
				Top:    design.Space.XL,
				Right:  design.Space.XL,
				Bottom: 72,
			},
		})),
	)
	contents.AddChild(body)
	footer := shell.tr(shell.panelFooter())
	if footer != "" {
		contents.AddChild(design.text(
			footer,
			design.Type.Caption,
			design.Palette.TextDisabled,
			widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionEnd,
				Padding: &widget.Insets{
					Left:   design.Space.XL,
					Bottom: design.Space.L,
				},
			},
		))
	}

	var panelWindow *widget.Window
	closeButton := design.button(
		shell.tr("Close"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		96,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			shell.panel = nil
			if panelWindow != nil {
				panelWindow.Close()
			}
			u.panelWindow = nil
			u.panelSignature = ""
			u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
		},
	)
	closeButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(closeButton)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(design.Palette.AccentSoft)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr(shell.panel.Title),
		design.Type.Heading,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.XL},
		},
	))
	panelWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 46),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			740,
			580,
		)),
	)
	u.panelWindow = panelWindow
	u.ui.AddWindow(panelWindow)
}

func (u *shellUI) syncWelcomePanel(shell *Shell) {
	progress := shell.updateProgress[updateComponentProduct]
	signature := fmt.Sprintf(
		"welcome|%dx%d|%s|%t|%s",
		u.viewportWidth,
		u.viewportHeight,
		shell.settings.UpdateChannel,
		shell.welcomeInstalling,
		progress.Message,
	)
	if signature == u.panelSignature && u.panelWindow != nil {
		return
	}

	u.closePanel()
	u.panelSignature = signature
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.SurfaceRaised),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	compactActions := u.viewportWidth < 560
	bodyBottom := 106
	actionDirection := widget.DirectionHorizontal
	actionWidth := 154
	if compactActions {
		bodyBottom = 202
		actionDirection = widget.DirectionVertical
		actionWidth = min(280, max(190, u.viewportWidth-88))
	}
	bodyText := shell.tr(
		"Choose the update channel for the integrated ARAM product.\n\n" +
			"Stable is recommended for normal play. Nightly follows the latest " +
			"successful main-branch build and may contain experimental changes.\n\n" +
			"aram-core is already compiled into aram-emu; no separate core " +
			"download is required. The optional aram-core tools archive only " +
			"contains developer CLI utilities.\n\n" +
			"You can change this later in Settings > Updates.",
	)
	if _, ok := shell.backend.(ProductUpdateInstaller); ok {
		bodyText = shell.tr(
			"Choose Stable or Nightly for the integrated ARAM product.\n\n" +
				"ARAM downloads the latest build for that channel, including its " +
				"compatible aram-core and aram-frontend revisions, then installs " +
				"and restarts automatically.\n\n" +
				"If no Stable release exists yet, ARAM continues with the bundled " +
				"build. Nightly follows the latest successful integration build.",
		)
	}
	if shell.welcomeInstalling {
		message := progress.Message
		if message == "" {
			message = shell.tr("Preparing the integrated ARAM update...")
		}
		bodyText = shell.tr("Setting up ARAM") + "\n\n" + message + "\n\n" +
			shell.tr("The verified integrated build contains compatible aram-core and "+
				"aram-frontend revisions. ARAM will restart automatically when "+
				"installation finishes.")
	}
	body := widget.NewText(
		widget.TextOpts.Text(
			bodyText,
			design.Type.Body,
			design.Palette.TextMuted,
		),
		widget.TextOpts.MaxWidth(float64(min(560, max(180, u.viewportWidth-88)))),
		widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			Padding: &widget.Insets{
				Left:   design.Space.XL,
				Top:    design.Space.XL,
				Right:  design.Space.XL,
				Bottom: bodyBottom,
			},
		})),
	)
	contents.AddChild(body)

	var welcomeWindow *widget.Window
	closeWindow := func() {
		if welcomeWindow != nil {
			welcomeWindow.Close()
		}
		u.panelWindow = nil
		u.panelSignature = ""
		u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
	}
	complete := func(channel updateChannel) {
		shell.completeWelcome(channel)
		if shell.panel == nil {
			closeWindow()
		}
	}
	u.welcomeStableButton = design.button(
		shell.tr("Use Stable (Recommended)"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		actionWidth,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() { complete(updateChannelStable) },
	)
	u.welcomeNightlyButton = design.button(
		shell.tr("Use Nightly"),
		design.Components.SubtleButton,
		design.Type.Strong,
		actionWidth,
		design.Components.SubtleButton.MinHeight,
		widget.TextPositionCenter,
		func() { complete(updateChannelNightly) },
	)
	u.welcomeLaterButton = design.button(
		shell.tr("Decide later"),
		design.Components.SubtleButton,
		design.Type.Strong,
		actionWidth,
		design.Components.SubtleButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			shell.dismissWelcome()
			closeWindow()
		},
	)
	u.welcomeStableButton.GetWidget().Disabled = shell.welcomeInstalling
	u.welcomeNightlyButton.GetWidget().Disabled = shell.welcomeInstalling
	u.welcomeLaterButton.GetWidget().Disabled = shell.welcomeInstalling
	actions := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(actionDirection),
			widget.RowLayoutOpts.Spacing(design.Space.S),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionCenter,
			VerticalPosition:   widget.AnchorLayoutPositionEnd,
			Padding:            &widget.Insets{Bottom: design.Space.L},
		})),
	)
	actions.AddChild(
		u.welcomeStableButton,
		u.welcomeNightlyButton,
		u.welcomeLaterButton,
	)
	contents.AddChild(actions)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(
			euiimage.NewNineSliceColor(design.Palette.AccentSoft),
		),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr("Welcome to ARAM"),
		design.Type.Heading,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.XL},
		},
	))
	welcomeWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 46),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			650,
			470,
		)),
	)
	u.panelWindow = welcomeWindow
	u.ui.AddWindow(welcomeWindow)
}

func (u *shellUI) syncRecentPanel(shell *Shell) {
	recent := append([]string(nil), shell.settings.RecentFiles...)
	signature := fmt.Sprintf(
		"recent|%dx%d|%s",
		u.viewportWidth,
		u.viewportHeight,
		strings.Join(recent, "\x00"),
	)
	if signature == u.panelSignature && u.panelWindow != nil {
		return
	}

	u.closePanel()
	u.panelSignature = signature
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.SurfaceRaised),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	contents.AddChild(design.text(
		shell.trf(
			"%d recent inputs — select one to inspect its full path.",
			len(recent),
		),
		design.Type.Body,
		design.Palette.TextMuted,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			Padding: &widget.Insets{
				Left: design.Space.XL,
				Top:  design.Space.L,
			},
		},
	))

	labelWidth := max(28, min(104, (u.viewportWidth-96)/7))
	detailWidth := max(28, min(104, (u.viewportWidth-84)/7))
	selectedPath := ""
	if len(recent) > 0 {
		selectedPath = recent[0]
	}
	pathText := widget.NewText(
		widget.TextOpts.Text(
			recentPathDetails(
				selectedPath,
				detailWidth,
				5,
				shell.language(),
			),
			design.Type.Caption,
			design.Palette.TextMuted,
		),
		widget.TextOpts.MaxWidth(float64(max(240, u.viewportWidth-96))),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionEnd,
				StretchHorizontal:  true,
				Padding: &widget.Insets{
					Left:   design.Space.XL,
					Right:  design.Space.XL,
					Bottom: 62,
				},
			}),
			widget.WidgetOpts.MinSize(0, 92),
		),
	)
	contents.AddChild(pathText)

	var recentWindow *widget.Window
	closeRecent := func() {
		shell.panel = nil
		if recentWindow != nil {
			recentWindow.Close()
		}
		u.panelWindow = nil
		u.panelSignature = ""
		u.recentList = nil
		u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
	}
	openButton := design.button(
		shell.tr("Open"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		96,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			path := selectedPath
			closeRecent()
			shell.openRecentPath(path)
		},
	)
	openButton.GetWidget().Disabled = selectedPath == ""
	openButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(openButton)

	cancelButton := design.button(
		shell.tr("Cancel"),
		design.Components.SubtleButton,
		design.Type.Strong,
		96,
		design.Components.SubtleButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			closeRecent()
			shell.setStatus(shell.tr("Open recent canceled"))
		},
	)
	cancelButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L + 104,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(cancelButton)

	entries := make([]any, len(recent))
	for index, path := range recent {
		entries[index] = path
	}
	trackIdle := euiimage.NewNineSliceColor(design.Palette.Border)
	trackHover := euiimage.NewNineSliceColor(design.Palette.BorderStrong)
	recentList := widget.NewList(
		widget.ListOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
					HorizontalPosition: widget.AnchorLayoutPositionStart,
					VerticalPosition:   widget.AnchorLayoutPositionStart,
					StretchHorizontal:  true,
					StretchVertical:    true,
					Padding: &widget.Insets{
						Left:   design.Space.XL,
						Top:    48,
						Right:  design.Space.XL,
						Bottom: 172,
					},
				}),
			),
		),
		widget.ListOpts.Entries(entries),
		widget.ListOpts.ScrollContainerImage(design.Components.Scroll),
		widget.ListOpts.SliderParams(&widget.SliderParams{
			TrackImage: &widget.SliderTrackImage{
				Idle:     trackIdle,
				Hover:    trackHover,
				Disabled: trackIdle,
			},
			HandleImage:   design.Components.TouchButton.Image,
			MinHandleSize: intPointer(28),
			TrackPadding:  &widget.Insets{Left: 3, Right: 3},
		}),
		widget.ListOpts.ControlWidgetSpacing(design.Space.XS),
		widget.ListOpts.HideHorizontalSlider(),
		widget.ListOpts.EntryFontFace(design.Type.Body),
		widget.ListOpts.EntryColor(&widget.ListEntryColor{
			Unselected:                 design.Palette.Text,
			Selected:                   design.Palette.Text,
			DisabledUnselected:         design.Palette.TextDisabled,
			DisabledSelected:           design.Palette.TextDisabled,
			SelectingBackground:        design.Palette.SurfaceHover,
			SelectedBackground:         design.Palette.AccentSoft,
			FocusedBackground:          design.Palette.SurfaceHover,
			SelectingFocusedBackground: design.Palette.AccentSoft,
			SelectedFocusedBackground:  design.Palette.AccentSoft,
			DisabledSelectedBackground: design.Palette.Surface,
		}),
		widget.ListOpts.EntryLabelFunc(func(entry any) string {
			path, _ := entry.(string)
			return recentEntryLabel(path, labelWidth)
		}),
		widget.ListOpts.EntryTextPadding(&widget.Insets{
			Left:   design.Space.S,
			Top:    11,
			Right:  design.Space.S,
			Bottom: 11,
		}),
		widget.ListOpts.EntryTextPosition(
			widget.TextPositionStart,
			widget.TextPositionCenter,
		),
		widget.ListOpts.SelectFocus(),
		widget.ListOpts.EntrySelectedHandler(func(args *widget.ListEntrySelectedEventArgs) {
			path, _ := args.Entry.(string)
			selectedPath = path
			pathText.Label = recentPathDetails(
				path,
				detailWidth,
				5,
				shell.language(),
			)
			openButton.GetWidget().Disabled = path == ""
		}),
	)
	u.recentList = recentList
	contents.AddChild(recentList)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(design.Palette.AccentSoft)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr("Open Recent"),
		design.Type.Heading,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.XL},
		},
	))
	recentWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 42),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Draggable(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			760,
			580,
		)),
	)
	u.panelWindow = recentWindow
	u.ui.AddWindow(recentWindow)
	if selectedPath != "" {
		recentList.SetSelectedEntry(selectedPath)
	}
}

func recentEntryLabel(path string, width int) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = path
	}
	parent := filepath.Dir(path)
	nameLimit := max(12, width*2/5)
	parentLimit := max(10, width-nameLimit-5)
	return shorten(name, nameLimit) + "  —  " + shorten(parent, parentLimit)
}

func recentPathDetails(path string, width, limit int, languages ...Language) string {
	language := LanguageEnglish
	if len(languages) > 0 {
		language = languages[0]
	}
	if strings.TrimSpace(path) == "" {
		return translate(language, "Select an input to view its full path.")
	}
	lines := wrapPanelLines(
		[]string{translatef(language, "Full path: %s", path)},
		max(12, width),
		1024,
	)
	if len(lines) > limit {
		lines = lines[:limit]
		last := strings.TrimSpace(lines[limit-1])
		lines[limit-1] = strings.TrimSuffix(last, "...") + "..."
	}
	return strings.Join(lines, "\n")
}

func intPointer(value int) *int {
	return &value
}

func newToolFieldDropdown(
	design *ARAMDesignSystem,
	panel *Panel,
	field ToolField,
	translateLabel func(string) string,
) *widget.ListComboButton {
	entries := make([]any, len(field.Options))
	initial := field.Options[0]
	current := panel.FieldValues[field.ID]
	for index, option := range field.Options {
		entries[index] = option
		if option.Value == current {
			initial = option
		}
	}
	if panel.FieldValues == nil {
		panel.FieldValues = make(map[string]string)
	}
	panel.FieldValues[field.ID] = initial.Value

	trackIdle := euiimage.NewNineSliceColor(design.Palette.Border)
	trackHover := euiimage.NewNineSliceColor(design.Palette.BorderStrong)
	return widget.NewListComboButton(
		widget.ListComboButtonOpts.Entries(entries),
		widget.ListComboButtonOpts.InitialEntry(initial),
		widget.ListComboButtonOpts.MaxContentHeight(132),
		widget.ListComboButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MinSize(0, 34),
		),
		widget.ListComboButtonOpts.ButtonParams(&widget.ButtonParams{
			Image: buttonImages(
				design.Components.ControlGroup,
				design.Components.Dropdown,
				design.Components.Dropdown,
				design.Components.Dropdown,
				design.Components.ControlGroup,
			),
			TextColor: design.Components.CommandButton.Text,
			TextFace:  design.Type.Body,
			TextPadding: &widget.Insets{
				Left: design.Space.S, Right: design.Space.S,
			},
			TextPosition: &widget.TextPositioning{
				HTextPosition: widget.TextPositionStart,
				VTextPosition: widget.TextPositionCenter,
			},
			MinSize: &image.Point{Y: 34},
		}),
		widget.ListComboButtonOpts.ListParams(&widget.ListParams{
			ScrollContainerImage: design.Components.Scroll,
			Slider: &widget.SliderParams{
				TrackImage: &widget.SliderTrackImage{
					Idle:     trackIdle,
					Hover:    trackHover,
					Disabled: trackIdle,
				},
				HandleImage:   design.Components.TouchButton.Image,
				MinHandleSize: intPointer(24),
				TrackPadding:  &widget.Insets{Left: 3, Right: 3},
			},
			EntryFace: design.Type.Body,
			EntryColor: &widget.ListEntryColor{
				Unselected:                 design.Palette.Text,
				Selected:                   design.Palette.Text,
				DisabledUnselected:         design.Palette.TextDisabled,
				DisabledSelected:           design.Palette.TextDisabled,
				SelectingBackground:        design.Palette.SurfaceHover,
				SelectedBackground:         design.Palette.AccentSoft,
				FocusedBackground:          design.Palette.SurfaceHover,
				SelectingFocusedBackground: design.Palette.AccentSoft,
				SelectedFocusedBackground:  design.Palette.AccentSoft,
				DisabledSelectedBackground: design.Palette.Surface,
			},
			EntryTextPadding: &widget.Insets{
				Left:   design.Space.S,
				Top:    design.Space.S,
				Right:  design.Space.S,
				Bottom: design.Space.S,
			},
			MinSize: &image.Point{X: 240},
		}),
		widget.ListComboButtonOpts.EntryLabelFunc(
			func(entry any) string {
				option, _ := entry.(ToolFieldOption)
				return translateLabel(option.Label)
			},
			func(entry any) string {
				option, _ := entry.(ToolFieldOption)
				return translateLabel(option.Label)
			},
		),
		widget.ListComboButtonOpts.EntrySelectedHandler(
			func(args *widget.ListComboButtonEntrySelectedEventArgs) {
				option, ok := args.Entry.(ToolFieldOption)
				if !ok {
					return
				}
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = option.Value
			},
		),
	)
}

func newToolFieldCheckbox(
	design *ARAMDesignSystem,
	shell *Shell,
	panel *Panel,
	field ToolField,
	translateLabel func(string) string,
) *widget.Checkbox {
	value := field.Value
	if current, ok := panel.FieldValues[field.ID]; ok {
		value = current
	}
	checked := strings.EqualFold(strings.TrimSpace(value), "true")
	if panel.FieldValues == nil {
		panel.FieldValues = make(map[string]string)
	}
	panel.FieldValues[field.ID] = fmt.Sprintf("%t", checked)
	initialState := widget.WidgetUnchecked
	if checked {
		initialState = widget.WidgetChecked
	}
	options := []widget.CheckboxOpt{}
	if panel.AllowGuestInput {
		// The same keys reach the guest while this panel is open, so a focused
		// control must not also answer to them.
		options = append(options, widget.CheckboxOpts.DisableDefaultKeys())
	}
	return widget.NewCheckbox(append(options,
		widget.CheckboxOpts.Image(design.Components.Checkbox),
		widget.CheckboxOpts.InitialState(initialState),
		widget.CheckboxOpts.Spacing(design.Space.S),
		widget.CheckboxOpts.Text(
			translateLabel(field.Label),
			design.Type.Body,
			&widget.LabelColor{
				Idle:     design.Palette.Text,
				Disabled: design.Palette.TextDisabled,
			},
		),
		widget.CheckboxOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MinSize(0, 34),
		),
		widget.CheckboxOpts.StateChangedHandler(
			func(args *widget.CheckboxChangedEventArgs) {
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = fmt.Sprintf(
					"%t",
					args.State == widget.WidgetChecked,
				)
				if field.Action != "" {
					shell.executeToolAction(field.Action, panel.FieldValues)
				}
			},
		),
	)...)
}

func (u *shellUI) syncInteractiveToolPanel(shell *Shell) {
	panel := shell.panel
	var signatureParts []string
	signatureParts = append(
		signatureParts,
		fmt.Sprintf("%dx%d", u.viewportWidth, u.viewportHeight),
		panel.Title,
		strings.Join(panel.Lines, "\x00"),
		fmt.Sprintf("busy=%t", panel.Busy),
	)
	for _, field := range panel.Fields {
		// A self-applying control reports its state through Value, so the
		// signature has to notice when the backend answers with a new one.
		state := ""
		if field.Action != "" {
			state = field.Value
		}
		signatureParts = append(
			signatureParts,
			fmt.Sprintf(
				"field:%s:%s:%s:%s:checkbox=%t:action=%s:state=%s",
				field.ID,
				field.Label,
				field.Placeholder,
				field.Detail,
				field.Checkbox,
				field.Action,
				state,
			),
		)
		for _, option := range field.Options {
			signatureParts = append(
				signatureParts,
				"option:"+field.ID+":"+option.Value+":"+option.Label,
			)
		}
	}
	for _, action := range panel.Actions {
		signatureParts = append(
			signatureParts,
			fmt.Sprintf("action:%s:%s:%t", action.ID, action.Label, action.Enabled),
		)
	}
	signature := strings.Join(signatureParts, "\x01")
	if signature == u.panelSignature && u.panelWindow != nil {
		return
	}

	u.closePanel()
	u.panelSignature = signature
	u.panelDropdowns = make(map[string]*widget.ListComboButton)
	u.panelCheckboxes = make(map[string]*widget.Checkbox)
	u.panelTextInputs = make(map[string]*imeTextInput)
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.SurfaceRaised),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	form := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(design.Space.M),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			StretchVertical:    true,
			Padding: &widget.Insets{
				Left:   design.Space.XL,
				Top:    design.Space.XL,
				Right:  design.Space.XL,
				Bottom: 82,
			},
		})),
	)
	if len(panel.Lines) > 0 {
		form.AddChild(widget.NewText(
			widget.TextOpts.Text(
				strings.Join(wrapPanelLines(shell.trLines(panel.Lines), 76, 14), "\n"),
				design.Type.Body,
				design.Palette.TextMuted,
			),
			widget.TextOpts.MaxWidth(float64(max(280, u.viewportWidth-120))),
			widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
				Stretch: true,
			})),
		))
	}
	for _, field := range panel.Fields {
		field := field
		fieldBlock := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionVertical),
				widget.RowLayoutOpts.Spacing(design.Space.XS),
			)),
			widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.RowLayoutData{
				Stretch: true,
			})),
		)
		if field.Checkbox {
			checkbox := newToolFieldCheckbox(
				design,
				shell,
				panel,
				field,
				shell.tr,
			)
			checkbox.GetWidget().Disabled = panel.Busy
			u.panelCheckboxes[field.ID] = checkbox
			fieldBlock.AddChild(checkbox)
			if field.Detail != "" {
				fieldBlock.AddChild(design.text(
					shell.tr(field.Detail),
					design.Type.Caption,
					design.Palette.TextMuted,
					widget.RowLayoutData{Stretch: true},
				))
			}
			form.AddChild(fieldBlock)
			continue
		}
		fieldBlock.AddChild(design.text(
			shell.tr(field.Label),
			design.Type.Strong,
			design.Palette.Text,
			widget.RowLayoutData{Stretch: true},
		))
		if len(field.Options) > 0 {
			dropdown := newToolFieldDropdown(design, panel, field, shell.tr)
			dropdown.GetWidget().Disabled = panel.Busy
			u.panelDropdowns[field.ID] = dropdown
			fieldBlock.AddChild(dropdown)
			form.AddChild(fieldBlock)
			continue
		}
		input := newIMETextInput(design, imeTextInputConfig{
			Placeholder: shell.tr(field.Placeholder),
			Text:        panel.FieldValues[field.ID],
			Disabled:    panel.Busy,
			MinHeight:   34,
			LayoutData:  widget.RowLayoutData{Stretch: true},
			Changed: func(value string) {
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = value
			},
		})
		u.panelTextInputs[field.ID] = input
		fieldBlock.AddChild(input)
		form.AddChild(fieldBlock)
	}
	contents.AddChild(form)

	actionRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(design.Space.S),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionEnd,
			Padding: &widget.Insets{
				Left:   design.Space.XL,
				Bottom: design.Space.M,
			},
		})),
	)
	for _, action := range panel.Actions {
		action := action
		button := design.button(
			shell.tr(action.Label),
			design.Components.SubtleButton,
			design.Type.Strong,
			112,
			design.Components.SubtleButton.MinHeight,
			widget.TextPositionCenter,
			func() {
				if panel.Kind == "issue-report" {
					shell.executeIssueReportAction(action.ID, panel.FieldValues)
				} else {
					shell.executeToolAction(action.ID, panel.FieldValues)
				}
				u.panelSignature = ""
			},
		)
		button.GetWidget().Disabled = !action.Enabled || panel.Busy
		actionRow.AddChild(button)
	}
	contents.AddChild(actionRow)

	var toolWindow *widget.Window
	closeButton := design.button(
		shell.tr("Close"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		96,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			shell.panel = nil
			if toolWindow != nil {
				toolWindow.Close()
			}
			u.panelWindow = nil
			u.panelSignature = ""
			u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
		},
	)
	closeButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(closeButton)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(design.Palette.AccentSoft)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr(panel.Title),
		design.Type.Heading,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.XL},
		},
	))
	toolWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 46),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Draggable(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			740,
			580,
		)),
	)
	u.panelWindow = toolWindow
	u.ui.AddWindow(toolWindow)
}

func (u *shellUI) syncSettingsPanel(shell *Shell) {
	switch u.settingsSection {
	case "General", "Appearance", "Graphics", "Audio", "Controls", "Bindings", "Updates":
	default:
		u.settingsSection = "General"
	}
	profile := shell.controllerProfile()
	// Slider-backed values (speed, state slot, volume, latency) are absent on
	// purpose: rebuilding the panel on every slider tick would destroy the
	// handle mid-drag. refreshSettingsSliders keeps their widgets in sync.
	signature := fmt.Sprintf(
		"settings|%s|%dx%d|%s|%s|%t|%t|%d|%s|%s|%t|%s|%s|%s|%s|%s|%s|%t|%s|%s|%s|%s",
		shell.settings.Language,
		u.viewportWidth,
		u.viewportHeight,
		u.settingsSection,
		shell.settings.ThemeMode,
		shell.settings.IntegerScaling,
		shell.settings.PreserveAspect,
		shell.settings.Rotation,
		shell.settings.ScreenLayout,
		shell.settings.Filter,
		shell.settings.Muted,
		shell.settings.AudioDeviceID,
		controllerProfileSignature(profile),
		shell.controllerProfileScopeLabel(),
		gamepadConnectionSignature(),
		shell.gamepadActivityLabel(),
		shell.backendName(),
		shell.settings.ShowVirtualKeypad,
		u.bindingDevice,
		bindingCaptureSignature(shell.bindingCapture),
		shell.settings.UpdateChannel,
		shell.updateProgressSignature(),
	)
	if signature == u.panelSignature && u.panelWindow != nil {
		u.refreshSettingsSliders()
		return
	}

	if u.settingsScroll != nil {
		u.settingsOffsets[u.settingsSection] = u.settingsScroll.ScrollTop
	}
	u.closePanel()
	u.panelSignature = signature
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.SurfaceRaised),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	navWidth := settingsNavWidth
	if u.viewportWidth < 600 {
		navWidth = 112
	}
	navigation := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(design.Palette.CanvasRaised)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(design.Space.M)),
			widget.RowLayoutOpts.Spacing(design.Space.XS),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchVertical:    true,
			}),
			widget.WidgetOpts.MinSize(navWidth, 0),
		),
	)
	navigation.AddChild(design.text(
		shell.tr("SETTINGS"),
		design.Type.Caption,
		design.Palette.AccentHover,
		widget.RowLayoutData{Stretch: true},
	))
	for _, section := range []string{"General", "Appearance", "Graphics", "Audio", "Controls", "Bindings", "Updates"} {
		sectionName := section
		button := design.button(
			shell.tr(sectionName),
			design.Components.SubtleButton,
			design.Type.Strong,
			navWidth-design.Space.XL,
			34,
			widget.TextPositionStart,
			func() {
				u.selectSettingsSection(u.owner, sectionName)
			},
		)
		button.GetWidget().LayoutData = widget.RowLayoutData{Stretch: true}
		if sectionName == u.settingsSection {
			base := design.Components.SubtleButton.Image
			button.SetImage(&widget.ButtonImage{
				Idle:         base.Pressed,
				Hover:        base.Pressed,
				Pressed:      base.Pressed,
				PressedHover: base.Pressed,
				Disabled:     base.Disabled,
			})
		}
		navigation.AddChild(button)
	}
	contents.AddChild(navigation)

	contentLeft := navWidth + design.Space.XL
	if u.compact {
		contentLeft = navWidth + design.Space.M
	}
	settingsContent := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			StretchVertical:    true,
			Padding: &widget.Insets{
				Left:   contentLeft,
				Top:    design.Space.L,
				Right:  design.Space.L,
				Bottom: 64,
			},
		})),
	)
	header := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(design.Space.XS),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
		})),
	)
	header.AddChild(
		design.text(
			shell.tr(u.settingsSection),
			design.Type.Display,
			design.Palette.Text,
			widget.RowLayoutData{Stretch: true},
		),
		design.text(
			shell.tr(settingsSectionDescription(u.settingsSection)),
			design.Type.Body,
			design.Palette.TextMuted,
			widget.RowLayoutData{Stretch: true},
		),
	)
	settingsContent.AddChild(header)
	rowsContent := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(design.Space.S),
		)),
	)
	for _, row := range u.settingsRows(shell) {
		rowsContent.AddChild(row)
	}
	var rowsScroll *widget.ScrollContainer
	rowsScroll = widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(rowsContent),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(design.Components.Scroll),
		widget.ScrollContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
				StretchVertical:    true,
				Padding:            &widget.Insets{Top: 62},
			}),
			widget.WidgetOpts.ScrolledHandler(func(args *widget.WidgetScrolledEventArgs) {
				scrollContainerByWheel(rowsScroll, args.Y)
			}),
		),
	)
	rowsScroll.ScrollTop = u.settingsOffsets[u.settingsSection]
	u.settingsScroll = rowsScroll
	settingsContent.AddChild(rowsScroll)
	contents.AddChild(settingsContent)

	contents.AddChild(design.text(
		shell.tr("Changes are saved immediately."),
		design.Type.Caption,
		design.Palette.TextDisabled,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionEnd,
			Padding: &widget.Insets{
				Left:   contentLeft,
				Bottom: design.Space.L,
			},
		},
	))

	var settingsWindow *widget.Window
	doneButton := design.button(
		shell.tr("Done"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		92,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			shell.panel = nil
			if settingsWindow != nil {
				settingsWindow.Close()
			}
			u.panelWindow = nil
			u.panelSignature = ""
			u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
		},
	)
	doneButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(doneButton)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(design.Palette.CanvasRaised)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr("Configure ARAM"),
		design.Type.Heading,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.L},
		},
	))
	settingsWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 42),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Draggable(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			700,
			560,
		)),
	)
	u.panelWindow = settingsWindow
	u.ui.AddWindow(settingsWindow)
}

// refreshSettingsSliders re-syncs slider and dropdown rows from the live
// settings, covering changes made outside the panel (menu commands,
// shortcuts) without rebuilding the panel widgets.
func (u *shellUI) refreshSettingsSliders() {
	for _, binding := range u.settingsSliders {
		value := clampInt(binding.model.value(), binding.model.min, binding.model.max)
		if binding.slider.Current != value {
			binding.slider.Current = value
		}
		if label := binding.model.format(value); binding.label.Label != label {
			binding.label.Label = label
		}
	}
	for _, binding := range u.settingsDropdowns {
		value := clampInt(binding.model.value(), 0, binding.model.count-1)
		if selected, ok := binding.dropdown.SelectedEntry().(int); !ok || selected != value {
			binding.dropdown.SetSelectedEntry(value)
		}
	}
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// scrollContainerByWheel moves sc by wheelY notches, one notch covering a
// fixed pixel step regardless of how tall the scrolled content is.
func scrollContainerByWheel(sc *widget.ScrollContainer, wheelY float64) {
	overflow := float64(sc.ContentRect().Dy() - sc.ViewRect().Dy())
	if overflow <= 0 {
		return
	}
	const wheelScrollStep = 48
	top := sc.ScrollTop - wheelY*wheelScrollStep/overflow
	if top < 0 {
		top = 0
	} else if top > 1 {
		top = 1
	}
	sc.ScrollTop = top
}

func settingsSectionDescription(section string) string {
	switch section {
	case "Appearance":
		return "Choose a neutral light or dark application theme."
	case "Graphics":
		return "Configure guest presentation and scaling."
	case "Audio":
		return "Configure frontend audio output."
	case "Controls":
		return "Configure normalized host input."
	case "Bindings":
		return "Select an action, then press the keyboard or gamepad button to assign."
	case "Updates":
		return "Download the latest public ARAM component archives from GitHub."
	default:
		return "Configure emulation and integration defaults."
	}
}

type settingsRowModel struct {
	label       string
	description string
	value       string
	action      func()
	disabled    bool
	slider      *settingsSliderModel
	dropdown    *settingsDropdownModel
}

// settingsSliderModel drives a slider-backed settings row. Slider positions
// run min..max in whole steps; value/format/apply translate between the
// position and the underlying setting.
type settingsSliderModel struct {
	min    int
	max    int
	value  func() int
	format func(int) string
	apply  func(int)
}

type settingsSliderBinding struct {
	slider *widget.Slider
	label  *widget.Text
	model  *settingsSliderModel
}

// settingsDropdownModel drives a dropdown-backed settings row with entries
// indexed 0..count-1.
type settingsDropdownModel struct {
	count int
	label func(int) string
	value func() int
	apply func(int)
}

type settingsDropdownBinding struct {
	dropdown *widget.ListComboButton
	model    *settingsDropdownModel
}

func (u *shellUI) settingsRows(shell *Shell) []*widget.Container {
	var rows []settingsRowModel
	profile := shell.controllerProfile()
	switch u.settingsSection {
	case "Appearance":
		rows = []settingsRowModel{
			{
				label:       "Application theme",
				description: "Switch between the neutral light and dark palettes.",
				value:       strings.Title(shell.settings.ThemeMode),
				action:      shell.cycleThemeMode,
			},
			{
				label:       "Control shape",
				description: "Buttons, menus, cards, and dialogs use square corners.",
				value:       "Square",
			},
		}
	case "Graphics":
		rows = []settingsRowModel{
			{
				label:       "Integer scaling",
				description: "Use whole-number scale factors when possible.",
				value:       onOff(shell.settings.IntegerScaling),
				action:      func() { shell.dispatchCommand("view.integer") },
			},
			{
				label:       "Preserve aspect ratio",
				description: "Prevent the guest image from stretching.",
				value:       onOff(shell.settings.PreserveAspect),
				action:      func() { shell.dispatchCommand("view.aspect") },
			},
			{
				label:       "Rotation",
				description: "Rotate the guest display clockwise.",
				value:       fmt.Sprintf("%d°", shell.settings.Rotation),
				action:      func() { shell.dispatchCommand("view.rotation") },
			},
			{
				label:       "Texture filter",
				description: "Choose nearest or linear sampling.",
				value:       strings.Title(shell.settings.Filter),
				action:      func() { shell.dispatchCommand("view.filter") },
			},
			{
				label:       "Screen layout",
				description: "Center or stretch the guest display.",
				value:       strings.Title(shell.settings.ScreenLayout),
				action:      func() { shell.dispatchCommand("view.layout") },
			},
		}
	case "Audio":
		rows = []settingsRowModel{
			{
				label:       "Mute output",
				description: "Silence frontend audio without stopping emulation.",
				value:       onOff(shell.settings.Muted),
				action:      shell.toggleMuted,
			},
			{
				label:       "Volume",
				description: "Output volume in five-percent steps.",
				slider: &settingsSliderModel{
					min:    0,
					max:    20,
					value:  func() int { return (shell.settings.Volume + 2) / 5 },
					format: func(v int) string { return fmt.Sprintf("%d%%", v*5) },
					apply:  func(v int) { shell.setVolume(v * 5) },
				},
			},
			{
				label:       "Requested latency",
				description: "Audio buffer target in ten-millisecond steps.",
				slider: &settingsSliderModel{
					min:    2,
					max:    25,
					value:  func() int { return (shell.settings.AudioLatencyMS + 5) / 10 },
					format: func(v int) string { return fmt.Sprintf("%d ms", v*10) },
					apply:  func(v int) { shell.setAudioLatency(v * 10) },
				},
			},
			{
				label:       "Output device",
				description: "Choose a backend-provided device or the system default.",
				value:       shorten(shell.audioDeviceLabel(), 20),
				action:      shell.cycleAudioDevice,
			},
		}
	case "Controls":
		rows = []settingsRowModel{
			{
				label:       "Profile scope",
				description: "Keep global controls or save overrides for the loaded title.",
				value:       shell.controllerProfileScopeLabel(),
				action:      shell.togglePerTitleControls,
			},
			{
				label:       "Keyboard profile",
				description: "Apply the arrow-key or WASD preset, replacing custom keyboard bindings.",
				value:       keyboardProfileLabel(profile.KeyboardProfile),
				action:      shell.cycleKeyboardProfile,
			},
			{
				label:       "Virtual keypad",
				description: "Show a clickable phone keypad to the right of the guest display.",
				value:       onOff(shell.settings.ShowVirtualKeypad),
				action:      shell.toggleVirtualKeypad,
			},
			{
				label:       "Gamepad input",
				description: "Accept input from standard-layout controllers.",
				value:       onOff(profile.GamepadEnabled),
				action:      shell.toggleGamepadEnabled,
			},
			{
				label:       "Confirm / back layout",
				description: "Choose the south or east face button for confirm.",
				value:       gamepadLayoutLabel(profile.GamepadLayout),
				action:      shell.cycleGamepadLayout,
			},
			{
				label:       "Analog directions",
				description: "Map the left stick to normalized directions.",
				value:       onOff(profile.GamepadAnalog),
				action:      shell.toggleGamepadAnalog,
			},
			{
				label:       "Stick dead zone",
				description: "Ignore small left-stick movement.",
				value:       fmt.Sprintf("%d%%", profile.GamepadDeadzone),
				action:      shell.cycleGamepadDeadzone,
			},
			{
				label:       "Connected gamepads",
				description: "Detected devices and standard-layout support.",
				value:       gamepadConnectionLabel(shell.language()),
			},
			{
				label:       "Controller database",
				description: "Reload ARAM/gamecontrollerdb.txt for unsupported devices.",
				value: func() string {
					if shell.gamepadMappingsLoaded {
						return "Custom"
					}
					return "Built-in"
				}(),
				action: shell.reloadGamepadMappings,
			},
			{
				label:       "Live input test",
				description: "Press controller buttons to verify the active mapping.",
				value:       shorten(shell.gamepadActivityLabel(), 22),
			},
			{
				label:       "Button bindings",
				description: "Capture keyboard keys or physical gamepad buttons.",
				value:       "Edit",
				action: func() {
					u.selectSettingsSection(shell, "Bindings")
				},
			},
		}
	case "Bindings":
		if u.bindingDevice != bindingDeviceKeyboard &&
			u.bindingDevice != bindingDeviceGamepad {
			u.bindingDevice = bindingDeviceKeyboard
		}
		rows = append(rows, settingsRowModel{
			label:       "Binding device",
			description: "Choose which physical input type to configure.",
			value:       strings.Title(string(u.bindingDevice)),
			action: func() {
				if shell.bindingCapture != nil {
					shell.cancelBindingCapture("Binding capture canceled")
				}
				if u.bindingDevice == bindingDeviceKeyboard {
					u.bindingDevice = bindingDeviceGamepad
				} else {
					u.bindingDevice = bindingDeviceKeyboard
				}
				u.panelSignature = ""
			},
		})
		controls := controllerControlOrder
		if u.bindingDevice == bindingDeviceKeyboard {
			controls = keyboardControlOrder
		}
		for _, control := range controls {
			controlID := control
			if u.bindingDevice == bindingDeviceKeyboard {
				value := shorten(shell.keyboardBindingLabel(controlID), 20)
				description := "Click, then press the keyboard key to assign. Esc cancels."
				if captureMatches(shell.bindingCapture, bindingDeviceKeyboard, controlID) {
					value = "Press a key..."
					description = "Listening for keyboard input. Esc cancels."
				}
				rows = append(rows, settingsRowModel{
					label:       controlDisplayName(controlID),
					description: description,
					value:       value,
					action:      func() { shell.beginKeyboardBindingCapture(controlID) },
				})
				continue
			}
			value := shorten(shell.gamepadBindingLabel(controlID), 20)
			description := "Click, then press the physical gamepad button to assign. Esc cancels."
			if captureMatches(shell.bindingCapture, bindingDeviceGamepad, controlID) {
				value = "Press a button..."
				description = "Listening for a standard-layout gamepad button. Esc cancels."
			}
			rows = append(rows, settingsRowModel{
				label:       controlDisplayName(controlID),
				description: description,
				value:       value,
				action:      func() { shell.beginGamepadBindingCapture(controlID) },
			})
		}
		rows = append(rows, settingsRowModel{
			label:       "Reset all bindings",
			description: "Restore keyboard and gamepad mappings for the active profile.",
			value:       "Reset all",
			action:      shell.resetControllerBindings,
		})
	case "Updates":
		rows = updateSettingsRowModels(shell)
	default:
		rows = []settingsRowModel{
			{
				label:       "Language",
				description: "Choose the language used by menus, settings, and frontend messages.",
				value:       languageLabel(shell.language(), shell.language()),
				action:      shell.cycleLanguage,
			},
			{
				label:       "Emulation speed",
				description: "Guest execution speed relative to the original handset.",
				slider: &settingsSliderModel{
					min:   0,
					max:   len(speedPresets) - 1,
					value: func() int { return speedPresetIndex(shell.settings.Speed) },
					format: func(v int) string {
						return fmt.Sprintf("%gx", speedPresets[clampInt(v, 0, len(speedPresets)-1)])
					},
					apply: func(v int) {
						shell.setSpeed(speedPresets[clampInt(v, 0, len(speedPresets)-1)])
					},
				},
			},
			{
				label:       "Save-state slot",
				description: "Slot used by load and save state commands.",
				dropdown: &settingsDropdownModel{
					count: 10,
					label: func(v int) string { return shell.trf("Slot %d", v) },
					value: func() int { return shell.settings.StateSlot },
					apply: shell.setStateSlot,
				},
			},
			{
				label:       "Backend",
				description: "Integration currently connected to the frontend.",
				value:       shorten(shell.backendName(), 24),
			},
		}
	}

	result := make([]*widget.Container, 0, len(rows))
	for _, row := range rows {
		result = append(result, u.buildSettingsRow(row))
	}
	return result
}

func updateSettingsRowModels(shell *Shell) []settingsRowModel {
	channel := normalizeUpdateChannel(shell.settings.UpdateChannel)
	downloadRoot, downloadRootErr := defaultUpdateDownloadRoot()
	downloadRootLabel := shorten(downloadRoot, 34)
	if downloadRootErr != nil {
		downloadRootLabel = "Unavailable"
	}
	return []settingsRowModel{
		{
			label:       "Current version",
			description: "Version of the running ARAM application.",
			value:       currentApplicationVersion(),
		},
		{
			label:       "Update channel",
			description: "Stable uses the latest official release; Nightly uses the latest main-branch build.",
			value:       updateChannelLabel(channel),
			action:      shell.cycleUpdateChannel,
		},
		{
			label: "ARAM product",
			description: shell.updateRowDescription(
				updateComponentProduct,
				"Integrated app, rebuilt from successful aram-core and aram-frontend Nightlies.",
			),
			value:    shell.updateActionLabel(updateComponentProduct),
			action:   func() { shell.downloadUpdate(updateComponentProduct) },
			disabled: !shell.updateActionAvailable(updateComponentProduct),
		},
		{
			label: "aram-core developer tools",
			description: shell.updateRowDescription(
				updateComponentCore,
				"Optional CLI debugger/inspectors; the emulator runtime is already built into ARAM product.",
			),
			value:    shell.updateActionLabel(updateComponentCore),
			action:   func() { shell.downloadUpdate(updateComponentCore) },
			disabled: !shell.updateActionAvailable(updateComponentCore),
		},
		{
			label: "aram-frontend",
			description: shell.updateRowDescription(
				updateComponentFrontend,
				"Optional standalone UI archive; it does not update the integrated ARAM app.",
			),
			value:    shell.updateActionLabel(updateComponentFrontend),
			action:   func() { shell.downloadUpdate(updateComponentFrontend) },
			disabled: !shell.updateActionAvailable(updateComponentFrontend),
		},
		{
			label:       "Download folder",
			description: "Archives are verified and saved without replacing the running application.",
			value:       downloadRootLabel,
		},
	}
}

func (u *shellUI) buildSettingsRow(model settingsRowModel) *widget.Container {
	design := u.design
	actionWidth := 126
	sliderWidth := 150
	if u.viewportWidth < 600 {
		actionWidth = 92
		sliderWidth = 110
	}
	const sliderValueWidth = 56
	if model.slider != nil {
		actionWidth = sliderWidth + design.Space.S + sliderValueWidth
	}
	row := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.ControlGroup),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MinSize(0, func() int {
				if u.compact {
					return 50
				}
				return 58
			}()),
		),
	)
	copyBlock := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(design.Space.XXS),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding: &widget.Insets{
				Left:  design.Space.M,
				Right: actionWidth + design.Space.L,
			},
		})),
	)
	copyBlock.AddChild(design.text(
		u.owner.tr(model.label),
		design.Type.Strong,
		design.Palette.Text,
		nil,
	))
	if !u.compact {
		copyBlock.AddChild(
			design.text(
				u.owner.tr(model.description),
				design.Type.Caption,
				design.Palette.TextMuted,
				nil,
			),
		)
	}
	row.AddChild(copyBlock)

	if model.dropdown != nil {
		dropdownModel := model.dropdown
		entries := make([]any, dropdownModel.count)
		for index := range entries {
			entries[index] = index
		}
		initial := clampInt(dropdownModel.value(), 0, dropdownModel.count-1)
		entryLabel := func(entry any) string {
			index, _ := entry.(int)
			return dropdownModel.label(index)
		}
		trackIdle := euiimage.NewNineSliceColor(design.Palette.Border)
		trackHover := euiimage.NewNineSliceColor(design.Palette.BorderStrong)
		dropdown := widget.NewListComboButton(
			widget.ListComboButtonOpts.Entries(entries),
			widget.ListComboButtonOpts.InitialEntry(initial),
			widget.ListComboButtonOpts.MaxContentHeight(148),
			widget.ListComboButtonOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(actionWidth, 32),
				widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
					HorizontalPosition: widget.AnchorLayoutPositionEnd,
					VerticalPosition:   widget.AnchorLayoutPositionCenter,
					Padding:            &widget.Insets{Right: design.Space.S},
				}),
			),
			widget.ListComboButtonOpts.ButtonParams(&widget.ButtonParams{
				Image: buttonImages(
					design.Components.ControlGroup,
					design.Components.Dropdown,
					design.Components.Dropdown,
					design.Components.Dropdown,
					design.Components.ControlGroup,
				),
				TextColor: design.Components.CommandButton.Text,
				TextFace:  design.Type.Strong,
				TextPadding: &widget.Insets{
					Left: design.Space.S, Right: design.Space.S,
				},
				TextPosition: &widget.TextPositioning{
					HTextPosition: widget.TextPositionCenter,
					VTextPosition: widget.TextPositionCenter,
				},
				MinSize: &image.Point{Y: 32},
			}),
			widget.ListComboButtonOpts.ListParams(&widget.ListParams{
				ScrollContainerImage: &widget.ScrollContainerImage{
					Idle:     design.Components.Dropdown,
					Disabled: design.Components.Dropdown,
					Mask:     design.Components.Scroll.Mask,
				},
				Slider: &widget.SliderParams{
					TrackImage: &widget.SliderTrackImage{
						Idle:     trackIdle,
						Hover:    trackHover,
						Disabled: trackIdle,
					},
					HandleImage:   design.Components.TouchButton.Image,
					MinHandleSize: intPointer(24),
					TrackPadding:  &widget.Insets{Left: 3, Right: 3},
				},
				EntryFace: design.Type.Body,
				EntryColor: &widget.ListEntryColor{
					Unselected:                 design.Palette.Text,
					Selected:                   design.Palette.Text,
					DisabledUnselected:         design.Palette.TextDisabled,
					DisabledSelected:           design.Palette.TextDisabled,
					SelectingBackground:        design.Palette.SurfaceHover,
					SelectedBackground:         design.Palette.AccentSoft,
					FocusedBackground:          design.Palette.SurfaceHover,
					SelectingFocusedBackground: design.Palette.AccentSoft,
					SelectedFocusedBackground:  design.Palette.AccentSoft,
					DisabledSelectedBackground: design.Palette.Surface,
				},
				EntryTextPadding: &widget.Insets{
					Left:   design.Space.S,
					Top:    design.Space.XS,
					Right:  design.Space.S,
					Bottom: design.Space.XS,
				},
				MinSize: &image.Point{X: actionWidth},
			}),
			widget.ListComboButtonOpts.EntryLabelFunc(entryLabel, entryLabel),
			widget.ListComboButtonOpts.EntrySelectedHandler(
				func(args *widget.ListComboButtonEntrySelectedEventArgs) {
					index, ok := args.Entry.(int)
					if !ok {
						return
					}
					dropdownModel.apply(index)
				},
			),
		)
		dropdown.GetWidget().Disabled = model.disabled
		row.AddChild(dropdown)
		u.settingsDropdowns = append(u.settingsDropdowns, settingsDropdownBinding{
			dropdown: dropdown,
			model:    dropdownModel,
		})
		return row
	}

	if model.slider != nil {
		sliderModel := model.slider
		current := clampInt(sliderModel.value(), sliderModel.min, sliderModel.max)
		valueLabel := design.text(
			sliderModel.format(current),
			design.Type.Strong,
			design.Palette.Text,
			widget.RowLayoutData{Position: widget.RowLayoutPositionCenter},
		)
		valueLabel.GetWidget().MinWidth = sliderValueWidth
		slider := widget.NewSlider(
			widget.SliderOpts.Direction(widget.DirectionHorizontal),
			widget.SliderOpts.MinMax(sliderModel.min, sliderModel.max),
			widget.SliderOpts.InitialCurrent(current),
			widget.SliderOpts.Images(design.Components.SliderTrack, design.Components.SliderHandle),
			widget.SliderOpts.FixedHandleSize(14),
			widget.SliderOpts.PageSizeFunc(func() int { return 1 }),
			widget.SliderOpts.ChangedHandler(func(args *widget.SliderChangedEventArgs) {
				sliderModel.apply(args.Current)
				applied := clampInt(sliderModel.value(), sliderModel.min, sliderModel.max)
				if applied != args.Slider.Current {
					args.Slider.Current = applied
				}
				valueLabel.Label = sliderModel.format(applied)
			}),
			widget.SliderOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(sliderWidth, 22),
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			),
		)
		slider.GetWidget().Disabled = model.disabled
		sliderGroup := widget.NewContainer(
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Spacing(design.Space.S),
			)),
			widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionEnd,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				Padding:            &widget.Insets{Right: design.Space.M},
			})),
		)
		sliderGroup.AddChild(slider, valueLabel)
		row.AddChild(sliderGroup)
		u.settingsSliders = append(u.settingsSliders, settingsSliderBinding{
			slider: slider,
			label:  valueLabel,
			model:  sliderModel,
		})
		return row
	}

	if model.action == nil {
		row.AddChild(design.text(
			u.owner.tr(model.value),
			design.Type.Strong,
			design.Palette.TextMuted,
			widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionEnd,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				Padding:            &widget.Insets{Right: design.Space.M},
			},
		))
		return row
	}

	action := model.action
	valueButton := design.button(
		u.owner.tr(model.value),
		design.Components.SubtleButton,
		design.Type.Strong,
		actionWidth,
		32,
		widget.TextPositionCenter,
		func() {
			action()
			u.panelSignature = ""
		},
	)
	valueButton.GetWidget().Disabled = model.disabled
	valueButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionCenter,
		Padding:            &widget.Insets{Right: design.Space.S},
	}
	row.AddChild(valueButton)
	return row
}

func onOff(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}

func visibility(visible bool) widget.Visibility {
	if visible {
		return widget.Visibility_Show
	}
	return widget.Visibility_Hide
}

func (u *shellUI) selectSettingsSection(shell *Shell, section string) {
	if u.settingsScroll != nil {
		u.settingsOffsets[u.settingsSection] = u.settingsScroll.ScrollTop
		u.settingsScroll = nil
	}
	u.settingsSection = section
	shell.settingsSection = section
	u.panelSignature = ""
}

func centeredWindowRect(viewWidth, viewHeight, preferredWidth, preferredHeight int) image.Rectangle {
	if viewWidth <= 0 || viewHeight <= 0 {
		viewWidth, viewHeight = logicalWidth, logicalHeight
	}
	margin := 18
	width := min(preferredWidth, max(1, viewWidth-margin*2))
	height := min(preferredHeight, max(1, viewHeight-margin*2))
	x := max(0, (viewWidth-width)/2)
	y := max(0, (viewHeight-height)/2)
	return image.Rect(x, y, x+width, y+height)
}

func (u *shellUI) closePanel() {
	window := u.panelWindow
	u.panelWindow = nil
	u.panelSignature = ""
	u.panelDropdowns = nil
	u.panelCheckboxes = nil
	u.settingsSliders = nil
	u.settingsDropdowns = nil
	// Release the text input session so the IME is detached again once the
	// form is gone.
	for _, input := range u.panelTextInputs {
		input.Focus(false)
	}
	u.panelTextInputs = nil
	u.settingsScroll = nil
	u.recentList = nil
	u.welcomeStableButton = nil
	u.welcomeNightlyButton = nil
	u.welcomeLaterButton = nil
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
	if window != nil {
		window.Close()
	}
}
