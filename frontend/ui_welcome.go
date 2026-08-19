package frontend

import (
	"fmt"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

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
	if shell.welcomeInstallsProduct() {
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
