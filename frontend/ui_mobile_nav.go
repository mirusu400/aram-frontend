package frontend

import (
	"image"

	"github.com/ebitenui/ebitenui/widget"
)

const mobileDrawerMaxWidth = 360

var mobileQuickCommands = []string{
	"emu.pause",
	"emu.save_state",
	"emu.stop",
}

func mobileMenuRootIndex(menus []Menu) int {
	return len(menus)
}

func mobileDrawerRect(viewWidth, viewHeight int, scale float64) image.Rectangle {
	margin := scaledPixels(32, scale)
	minimum := scaledPixels(240, scale)
	width := max(minimum, viewWidth*86/100)
	width = min(width, scaledPixels(mobileDrawerMaxWidth, scale))
	width = min(width, max(1, viewWidth-margin))
	return image.Rect(0, 0, width, max(1, viewHeight))
}

// buildMobileAppBar replaces the desktop menu and action rows on touch
// devices. The title provides context while every command lives in the one
// reachable drawer opened by the hamburger button.
func (u *shellUI) buildMobileAppBar(shell *Shell) *widget.Container {
	design := u.design
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.Toolbar),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
			}),
			widget.WidgetOpts.MinSize(0, design.px(mobileAppBarHeight)),
		),
	)

	graphic := &widget.GraphicImage{
		Idle:     drawModernIconAtScale("menu", design.Palette.Text, design.Scale),
		Pressed:  drawModernIconAtScale("menu", design.Palette.Text, design.Scale),
		Disabled: drawModernIconAtScale("menu", design.Palette.TextDisabled, design.Scale),
	}
	u.mobileMenuButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(design.px(48), design.px(44)),
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				Padding:            &widget.Insets{Left: design.Space.S},
			}),
		),
		widget.ButtonOpts.Image(design.Components.SubtleButton.Image),
		widget.ButtonOpts.Graphic(graphic),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
			shell.buttonHaptic()
			if shell.activeMenu >= 0 {
				u.dismissMenu()
				return
			}
			u.openMobileMenuRoot()
		}),
	)
	u.mobileMenuButton.GetWidget().SetTheme(&widget.Theme{})
	u.mobileMenuButton.GetWidget().CustomData = "mobile:menu"

	u.toolbarTitle = design.text(
		"",
		design.Type.Strong,
		design.Palette.Text,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.px(68), Right: design.px(92)},
		},
	)
	bar.AddChild(u.mobileMenuButton, u.toolbarTitle)
	return bar
}

func (u *shellUI) openMobileMenuRoot() {
	u.closeMenu()
	u.owner.activeMenu = mobileMenuRootIndex(u.owner.menus)
	u.menuIndex = u.owner.activeMenu
	u.commandButtons = make(map[string]*widget.Button)

	body := u.mobileDrawerBody()
	body.AddChild(u.mobileDrawerSection(u.owner.tr("Quick actions")))
	for _, id := range mobileQuickCommands {
		command, found := u.owner.findCommand(id)
		if !found {
			continue
		}
		body.AddChild(u.mobileCommandButton(command))
	}
	body.AddChild(u.mobileDrawerSection(u.owner.tr("All commands")))
	for index, menu := range u.owner.menus {
		menuIndex := index
		body.AddChild(u.mobileDrawerButton(
			u.owner.tr(menu.Label)+"  >",
			"mobile:category:"+menu.Label,
			func() {
				u.owner.buttonHaptic()
				u.openMobileMenuCategory(menuIndex)
			},
		))
	}
	u.showMobileDrawer(u.owner.tr("Menu"), false, body)
}

func (u *shellUI) openMobileMenuCategory(index int) {
	if index < 0 || index >= len(u.owner.menus) {
		return
	}
	u.closeMenu()
	u.owner.activeMenu = index
	u.menuIndex = index
	u.commandButtons = make(map[string]*widget.Button)

	body := u.mobileDrawerBody()
	for _, command := range u.owner.menus[index].Commands {
		body.AddChild(u.mobileCommandButton(command))
	}
	u.showMobileDrawer(u.owner.tr(u.owner.menus[index].Label), true, body)
}

func (u *shellUI) mobileDrawerBody() *widget.Container {
	return widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(
		widget.RowLayoutOpts.Direction(widget.DirectionVertical),
		widget.RowLayoutOpts.Spacing(u.design.Space.XS),
	)))
}

func (u *shellUI) mobileDrawerSection(label string) *widget.Text {
	return u.design.text(
		label,
		u.design.Type.Caption,
		u.design.Palette.TextMuted,
		widget.RowLayoutData{Stretch: true},
	)
}

func (u *shellUI) mobileDrawerButton(label, id string, action func()) *widget.Button {
	viewWidth, viewHeight := u.owner.viewportSize()
	drawerWidth := mobileDrawerRect(viewWidth, viewHeight, u.design.Scale).Dx()
	button := u.design.button(
		label,
		u.design.Components.CommandButton,
		u.design.Type.Body,
		max(1, drawerWidth-u.design.Space.XL),
		u.design.Components.CommandButton.MinHeight,
		widget.TextPositionStart,
		action,
	)
	button.GetWidget().CustomData = id
	return button
}

func (u *shellUI) mobileCommandButton(command Command) *widget.Button {
	commandID := command.ID
	button := u.mobileDrawerButton(
		commandButtonLabel(command, u.owner),
		commandID,
		func() {
			u.owner.activeMenu = -1
			u.closeMenu()
			u.owner.finishTouchMenu()
			u.owner.dispatchCommand(commandID)
		},
	)
	button.GetWidget().Disabled = !command.IsEnabled(u.owner)
	u.commandButtons[commandID] = button
	return button
}

func (u *shellUI) showMobileDrawer(title string, back bool, body *widget.Container) {
	design := u.design
	viewWidth, viewHeight := u.owner.viewportSize()
	location := mobileDrawerRect(viewWidth, viewHeight, design.Scale)

	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.Dropdown),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	header := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.DialogTitle),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
			}),
			widget.WidgetOpts.MinSize(0, design.px(mobileAppBarHeight)),
		),
	)
	titlePadding := design.Space.L
	if back {
		titlePadding = design.px(84)
		backButton := design.button(
			u.owner.tr("Back"),
			design.Components.SubtleButton,
			design.Type.Strong,
			design.px(68),
			design.px(36),
			widget.TextPositionCenter,
			func() {
				u.owner.buttonHaptic()
				u.openMobileMenuRoot()
			},
		)
		backButton.GetWidget().CustomData = "mobile:back"
		backButton.GetWidget().LayoutData = widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.S},
		}
		header.AddChild(backButton)
	}
	header.AddChild(design.text(
		title,
		design.Type.Heading,
		design.Palette.OnTitle,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: titlePadding, Right: design.px(76)},
		},
	))
	closeButton := design.button(
		u.owner.tr("Close"),
		design.Components.SubtleButton,
		design.Type.Strong,
		design.px(64),
		design.px(36),
		widget.TextPositionCenter,
		func() {
			u.owner.buttonHaptic()
			u.dismissMenu()
		},
	)
	closeButton.GetWidget().CustomData = "mobile:close"
	closeButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionCenter,
		Padding:            &widget.Insets{Right: design.Space.S},
	}
	header.AddChild(closeButton)

	var scroll *widget.ScrollContainer
	scroll = widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(body),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(design.Components.Scroll),
		widget.ScrollContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
				StretchVertical:    true,
				Padding: &widget.Insets{
					Left:   design.Space.M,
					Top:    design.px(mobileAppBarHeight) + design.Space.M,
					Right:  design.Space.M,
					Bottom: design.Space.M,
				},
			}),
			widget.WidgetOpts.ScrolledHandler(func(args *widget.WidgetScrolledEventArgs) {
				scrollContainerByWheel(scroll, args.Y)
			}),
		),
	)
	contents.AddChild(scroll, header)

	var window *widget.Window
	window = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.CloseMode(widget.CLICK_OUT),
		widget.WindowOpts.Location(location),
		widget.WindowOpts.ClosedHandler(func(*widget.WindowClosedEventArgs) {
			if u.menuWindow != window {
				return
			}
			u.menuWindow = nil
			u.menuIndex = -1
			u.owner.activeMenu = -1
			u.commandButtons = make(map[string]*widget.Button)
			u.owner.finishTouchMenu()
		}),
	)
	u.menuWindow = window
	u.ui.AddWindow(window)
}

func (u *shellUI) dismissMenu() {
	u.owner.activeMenu = -1
	u.closeMenu()
	u.owner.finishTouchMenu()
}
