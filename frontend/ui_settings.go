package frontend

import (
	"fmt"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

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

func (u *shellUI) selectSettingsSection(shell *Shell, section string) {
	if u.settingsScroll != nil {
		u.settingsOffsets[u.settingsSection] = u.settingsScroll.ScrollTop
		u.settingsScroll = nil
	}
	u.settingsSection = section
	shell.settingsSection = section
	u.panelSignature = ""
}
