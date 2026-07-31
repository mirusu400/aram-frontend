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
	statusText           *widget.Text
	statusMeta           *widget.Text
	panelWindow          *widget.Window
	panelSignature       string
	settingsSection      string
	bindingDevice        bindingDevice
	scrim                *widget.Container
	viewportWidth        int
	viewportHeight       int
	compact              bool
	settingsScroll       *widget.ScrollContainer
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
	u.statusMeta.Label = fmt.Sprintf(
		"%s  •  %gx  •  %s",
		strings.ToUpper(shell.tr(stateValueLabel(string(shell.backend.State())))),
		shell.settings.Speed,
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
		signatureParts = append(signatureParts, "field:"+field.ID+":"+field.Label+":"+field.Placeholder)
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
		fieldBlock.AddChild(design.text(
			shell.tr(field.Label),
			design.Type.Strong,
			design.Palette.Text,
			widget.RowLayoutData{Stretch: true},
		))
		input := widget.NewTextInput(
			widget.TextInputOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
				widget.WidgetOpts.MinSize(0, 34),
			),
			widget.TextInputOpts.Image(&widget.TextInputImage{
				Idle:     design.Components.ControlGroup,
				Disabled: design.Components.ControlGroup,
			}),
			widget.TextInputOpts.Color(&widget.TextInputColor{
				Idle:          design.Palette.Text,
				Disabled:      design.Palette.TextDisabled,
				Caret:         design.Palette.Accent,
				DisabledCaret: design.Palette.TextDisabled,
			}),
			widget.TextInputOpts.Padding(&widget.Insets{
				Left: design.Space.S, Right: design.Space.S,
			}),
			widget.TextInputOpts.Face(design.Type.Body),
			widget.TextInputOpts.Placeholder(shell.tr(field.Placeholder)),
			widget.TextInputOpts.ChangedHandler(func(args *widget.TextInputChangedEventArgs) {
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = args.InputText
			}),
		)
		input.SetText(panel.FieldValues[field.ID])
		input.GetWidget().Disabled = panel.Busy
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
	signature := fmt.Sprintf(
		"settings|%s|%dx%d|%s|%s|%t|%t|%d|%s|%s|%d|%g|%t|%d|%d|%s|%s|%s|%s|%s|%s|%t|%s|%s|%s|%s",
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
		shell.settings.StateSlot,
		shell.settings.Speed,
		shell.settings.Muted,
		shell.settings.Volume,
		shell.settings.AudioLatencyMS,
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
	rowsScroll := widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(rowsContent),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(design.Components.Scroll),
		widget.ScrollContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			StretchVertical:    true,
			Padding:            &widget.Insets{Top: 62},
		})),
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
				description: "Click to advance in five-percent steps.",
				value:       fmt.Sprintf("%d%%", shell.settings.Volume),
				action:      shell.cycleVolume,
			},
			{
				label:       "Requested latency",
				description: "Click to advance in ten-millisecond steps.",
				value:       fmt.Sprintf("%d ms", shell.settings.AudioLatencyMS),
				action:      shell.cycleAudioLatency,
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
		channel := normalizeUpdateChannel(shell.settings.UpdateChannel)
		downloadRoot, downloadRootErr := defaultUpdateDownloadRoot()
		downloadRootLabel := shorten(downloadRoot, 34)
		if downloadRootErr != nil {
			downloadRootLabel = "Unavailable"
		}
		rows = []settingsRowModel{
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
					"Integrated aram-emu build with aram-core and aram-frontend.",
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
					"Standalone frontend archive without the integrated emulator backend.",
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
				description: "Default guest execution speed.",
				value:       fmt.Sprintf("%gx", shell.settings.Speed),
				action:      shell.cycleSpeed,
			},
			{
				label:       "Save-state slot",
				description: "Slot used by load and save state commands.",
				value:       shell.trf("Slot %d", shell.settings.StateSlot),
				action:      shell.cycleStateSlot,
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

func (u *shellUI) buildSettingsRow(model settingsRowModel) *widget.Container {
	design := u.design
	actionWidth := 126
	if u.viewportWidth < 600 {
		actionWidth = 92
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
