package frontend

import (
	"fmt"
	"strings"

	"github.com/ebitenui/ebitenui/widget"
)

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

	// A field waiting for the platform editor outlives this rebuild, which a
	// soft keyboard triggers by resizing the window.
	previousInputs := u.panelTextInputs
	u.closePanel()
	u.panelSignature = signature
	u.panelDropdowns = make(map[string]*widget.ListComboButton)
	u.panelCheckboxes = make(map[string]*widget.Checkbox)
	u.panelTextInputs = make(map[string]*imeTextInput)
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.DialogBody),
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
			Label:       shell.tr(field.Label),
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
		input.adoptNativeEdit(previousInputs[field.ID])
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
		widget.ContainerOpts.BackgroundImage(design.Components.DialogTitle),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr(panel.Title),
		design.Type.Heading,
		design.Palette.OnTitle,
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
