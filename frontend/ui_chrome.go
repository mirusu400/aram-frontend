package frontend

import (
	"image"

	"github.com/ebitenui/ebitenui/widget"
)

const (
	menuBarHeight            = 36
	applicationToolbarHeight = 48
	statusBarHeight          = 28
	settingsNavWidth         = 168
)

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
