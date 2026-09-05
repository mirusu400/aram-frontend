package frontend

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
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
	updateBadge          *widget.Button
	updateBadgeTip       *widget.Text
	statusBar            *widget.Container
	statusText           *widget.Text
	statusMeta           *widget.Text
	statusSignal         *widget.Graphic
	statusBattery        *widget.Graphic
	statusBatteryText    *widget.Text
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
	settingsTouchID      ebiten.TouchID
	settingsTouchActive  bool
	settingsTouchLastY   int
	recentScroll         *widget.ScrollContainer
	recentRowPaths       []string
	recentSelectedPath   string
	homeContainer        *widget.Container
	homeScroll           *widget.ScrollContainer
	homeRowPaths         []string
	homeRowContainers    map[string]*widget.Container
	homeOpenButton       *widget.Button
	homeFavButton        *widget.Button
	homeSelectedPath     string
	homeSignature        string
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

	// The Home surface sits in the guest viewport, behind the chrome bars and
	// the modal scrim, so File/Settings dialogs float over it. It is built
	// empty here and populated by syncHomeSurface. Its dark feature-phone skin
	// is its own palette, not the app design system.
	view.homeContainer = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(homeBackgroundImage()),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	view.homeContainer.GetWidget().SetVisibility(widget.Visibility_Hide)

	root.AddChild(view.homeContainer, topBar, toolbar, statusBar, view.scrim)
	view.ui = &ebitenui.UI{
		Container:           root,
		DisableDefaultFocus: false,
		PrimaryTheme:        design.Theme,
	}
	view.sync(shell)
	return view
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
	if u.updateBadge != nil {
		u.updateBadge.GetWidget().SetVisibility(visibility(shell.updateNoticeReady))
		if shell.updateNoticeReady && u.updateBadgeTip != nil {
			u.updateBadgeTip.Label = shell.updateNoticeTooltip()
		}
	}
	u.statusMeta.GetWidget().SetVisibility(visibility(width >= 700))
	u.syncStatusIndicators(shell, width)
	for id, button := range u.toolbarButtons {
		visible := true
		if width < 620 && (id == "emu.stop" || id == "emu.reset" ||
			id == "view.keypad" || id == "view.layout" || id == "view.aspect") {
			visible = false
		}
		if width < 480 && id == "emu.pause" {
			visible = false
		}
		button.GetWidget().SetVisibility(visibility(visible))
	}
	statusLimit := max(24, min(92, (width-32)/7))
	u.setStatusLabel(u.statusText, shorten(shell.status, statusLimit))
	// Show the achieved speed (e.g. "1x (98%)") rather than only the requested
	// setting, so a title running below handset speed is visible in the
	// always-on status bar without opening settings, which pauses the guest.
	u.setStatusLabel(u.statusMeta, fmt.Sprintf(
		"%s  •  %s  •  %s",
		strings.ToUpper(shell.tr(stateValueLabel(string(shell.backend.State())))),
		shell.speedSettingValue(),
		strings.ToUpper(shell.tr(shell.displayPresentationValueLabel())),
	))
	if shell.input == nil {
		u.toolbarTitle.Label = shell.tr("No title loaded")
	} else {
		u.toolbarTitle.Label = shorten(shell.input.DisplayName, 52)
	}

	for id, button := range u.toolbarButtons {
		command, found := shell.findCommand(id)
		button.GetWidget().Disabled = !found || !command.IsEnabled(shell)
	}

	// Toolbar toggles wear the sunken pressed face while active, the way the
	// settings navigation marks its section; momentary actions never do.
	base := u.design.Components.SubtleButton.Image
	pressed := &widget.ButtonImage{
		Idle:         base.Pressed,
		Hover:        base.Pressed,
		Pressed:      base.Pressed,
		PressedHover: base.Pressed,
		Disabled:     base.Disabled,
	}
	display := shell.displayProfile()
	for id, active := range map[string]bool{
		"view.keypad": shell.settings.ShowVirtualKeypad,
		"view.layout": display.ScreenLayout == "stretch",
		"view.aspect": display.PreserveAspect,
	} {
		if button, ok := u.toolbarButtons[id]; ok {
			if active {
				button.SetImage(pressed)
			} else {
				button.SetImage(base)
			}
		}
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
	u.syncHomeSurface(shell)
	u.syncPanel(shell)
	u.updateSettingsTouchScroll(shell)
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
		widget.ContainerOpts.BackgroundImage(design.Components.DialogBody),
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
		widget.ContainerOpts.BackgroundImage(design.Components.DialogTitle),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr(shell.panel.Title),
		design.Type.Heading,
		design.Palette.OnTitle,
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
	u.recentScroll = nil
	u.recentRowPaths = nil
	u.recentSelectedPath = ""
	u.welcomeStableButton = nil
	u.welcomeNightlyButton = nil
	u.welcomeLaterButton = nil
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
	if window != nil {
		window.Close()
	}
}

// setStatusLabel writes a status-bar label and asks the bar to lay itself out
// again when the text actually changed.
//
// EbitenUI caches a container's layout until something requests otherwise, and
// a plain assignment to Text.Label is not that. The trailing indicator cluster
// is anchored to the bar's right edge from its own preferred width, so a stale
// layout left the cluster sitting on top of the meta text as the reading grew,
// or pushed it past the edge of the bar entirely.
func (u *shellUI) setStatusLabel(target *widget.Text, label string) {
	if target == nil || target.Label == label {
		return
	}
	target.Label = label
	if u.statusBar != nil {
		u.statusBar.RequestRelayout()
	}
}

// syncStatusIndicators updates the handset indicator cluster at the right end
// of the status bar. Both readings are real or absent: the signal glyph is
// inked from what the guest machine is actually doing, and the charge meter
// only appears where the platform reports one, so a build with no way to read
// power shows nothing rather than a full battery it invented.
func (u *shellUI) syncStatusIndicators(shell *Shell, width int) {
	// The cluster is the first thing to go when the bar runs out of room; the
	// status text it sits beside carries the words that cannot be inferred.
	room := width >= 520
	palette := u.design.Palette
	if u.statusSignal != nil {
		var ink color.Color
		switch shell.machineActivity() {
		case activityRunning:
			ink = palette.Accent
		case activityPaused:
			ink = statusBarInk(palette, palette.TextMuted)
		case activityFaulted:
			ink = palette.Fault
		default:
			ink = statusBarInk(palette, palette.TextDisabled)
		}
		if icon := u.design.retroIndicatorIcon("signal", ink); icon != nil {
			u.statusSignal.Image = icon
		}
		u.statusSignal.GetWidget().SetVisibility(visibility(room))
	}
	battery := shell.hostBattery()
	show := room && battery.Present
	if u.statusBattery != nil {
		ink := statusBarInk(palette, palette.TextMuted)
		switch {
		case battery.Charging:
			ink = palette.Accent
		case battery.Percent <= batteryLow:
			ink = palette.Fault
		}
		if icon := u.design.retroIndicatorIcon("battery", ink); icon != nil {
			u.statusBattery.Image = icon
		}
		u.statusBattery.GetWidget().SetVisibility(visibility(show))
	}
	if show {
		u.setStatusLabel(u.statusBatteryText, fmt.Sprintf("%d%%", battery.Percent))
	} else {
		u.setStatusLabel(u.statusBatteryText, "")
	}
	u.statusBatteryText.GetWidget().SetVisibility(visibility(show))
}
