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

	addAction := func(id, label, icon string, width int) {
		commandID := id
		var button *widget.Button
		if graphic := design.retroIcon(icon); graphic != nil {
			// Sprite skins draw the era-style icon toolbar instead of the
			// text actions; the labels stay available through the menus.
			button = widget.NewButton(
				widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(36, 32)),
				widget.ButtonOpts.Image(design.Components.SubtleButton.Image),
				widget.ButtonOpts.Graphic(graphic),
				widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
					shell.dispatchCommand(commandID)
				}),
			)
			// Blank per-widget theme: with the app theme's DefaultTextColor
			// visible, EbitenUI inserts an empty text widget next to the
			// graphic and the row spacing pushes the icon off center.
			button.GetWidget().SetTheme(&widget.Theme{})
		} else {
			button = design.button(
				label,
				design.Components.SubtleButton,
				design.Type.Strong,
				width,
				32,
				widget.TextPositionCenter,
				func() { shell.dispatchCommand(commandID) },
			)
		}
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

	addAction("file.open", shell.tr("Open"), "open", 68)
	addSeparator()
	addAction("emu.start", shell.tr("Start"), "play", 64)
	addAction("emu.pause", shell.tr("Pause"), "pause", 64)
	addAction("emu.stop", shell.tr("Stop"), "stop", 60)
	addAction("emu.reset", shell.tr("Reset"), "reset", 62)
	addSeparator()
	addAction("emu.configure", shell.tr("Settings"), "settings", 82)

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
	commands := u.owner.menus[index].Commands
	itemHeight := design.Components.CommandButton.MinHeight + design.Space.XS
	u.commandButtons = make(map[string]*widget.Button)
	newCommandButton := func(command Command, width int) *widget.Button {
		commandID := command.ID
		button := design.button(
			commandButtonLabel(command, u.owner),
			design.Components.CommandButton,
			design.Type.Body,
			width,
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
		return button
	}

	var contents *widget.Container
	var location image.Rectangle
	if platformUsesTouchLayout() {
		// Touch layouts show the menu as a centered modal sheet instead of
		// an anchored dropdown that can run past a phone screen. Commands
		// flow into extra columns when one column would not fit the height.
		viewWidth, viewHeight := u.owner.viewportSize()
		layout := touchMenuLayoutFor(
			viewWidth,
			viewHeight,
			itemHeight,
			design.Space.L,
			len(commands),
		)
		location = layout.window
		contents = widget.NewContainer(
			widget.ContainerOpts.BackgroundImage(design.Components.Dropdown),
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
				widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(design.Space.S)),
				widget.RowLayoutOpts.Spacing(design.Space.XS),
			)),
		)
		buttonWidth := (layout.window.Dx() -
			design.Space.L - (layout.columns-1)*design.Space.XS) / layout.columns
		var column *widget.Container
		for position, command := range commands {
			if position%layout.perColumn == 0 {
				column = widget.NewContainer(
					widget.ContainerOpts.Layout(widget.NewRowLayout(
						widget.RowLayoutOpts.Direction(widget.DirectionVertical),
						widget.RowLayoutOpts.Spacing(design.Space.XS),
					)),
				)
				contents.AddChild(column)
			}
			column.AddChild(newCommandButton(command, buttonWidth))
		}
	} else {
		contents = widget.NewContainer(
			widget.ContainerOpts.BackgroundImage(design.Components.Dropdown),
			widget.ContainerOpts.Layout(widget.NewRowLayout(
				widget.RowLayoutOpts.Direction(widget.DirectionVertical),
				widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(design.Space.S)),
				widget.RowLayoutOpts.Spacing(design.Space.XS),
			)),
		)
		for _, command := range commands {
			contents.AddChild(newCommandButton(command, dropdownWidth-design.Space.L))
		}
		startX := menuUIStartX(u.owner.menus, index, design)
		height := design.Space.L + len(commands)*itemHeight
		location = image.Rect(
			startX,
			menuBarHeight+design.Space.XS,
			startX+dropdownWidth,
			menuBarHeight+design.Space.XS+height,
		)
	}

	var window *widget.Window
	window = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.CloseMode(widget.CLICK_OUT),
		widget.WindowOpts.Location(location),
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

// touchMenuLayout describes the centered modal command sheet used by touch
// layouts in place of the desktop dropdown.
type touchMenuLayout struct {
	columns     int
	perColumn   int
	columnWidth int
	window      image.Rectangle
}

func touchMenuLayoutFor(
	viewWidth, viewHeight, itemHeight, framePadding, count int,
) touchMenuLayout {
	if viewWidth <= 0 || viewHeight <= 0 {
		viewWidth, viewHeight = logicalWidth, logicalHeight
	}
	count = max(1, count)
	itemHeight = max(1, itemHeight)
	const margin = 18
	const minColumnWidth = 160
	available := max(itemHeight, viewHeight-menuBarHeight-statusBarHeight-margin*2)
	maxRows := max(1, (available-framePadding)/itemHeight)
	columns := (count + maxRows - 1) / maxRows
	maxColumns := max(1, (viewWidth-margin*2)/minColumnWidth)
	columns = max(1, min(columns, maxColumns))
	perColumn := (count + columns - 1) / columns
	columnWidth := min(dropdownWidth, (viewWidth-margin*2)/columns)
	width := columnWidth * columns
	height := min(available, framePadding+perColumn*itemHeight)
	x := max(0, (viewWidth-width)/2)
	y := menuBarHeight + margin + max(0, (available-height)/2)
	return touchMenuLayout{
		columns:     columns,
		perColumn:   perColumn,
		columnWidth: columnWidth,
		window:      image.Rect(x, y, x+width, y+height),
	}
}
