package frontend

import (
	"fmt"
	"strconv"

	"github.com/ebitenui/ebitenui/widget"
)

func (s *Shell) selectMemoryResult(result MemoryResult) {
	fields := cloneStringMap(s.panel.FieldValues)
	fields["address"] = fmt.Sprintf("0x%08x", result.Address)
	s.executeToolAction("select", fields)
}

func (s *Shell) writeSelectedMemory() {
	if s.panel == nil || s.panel.Memory == nil || s.panel.Memory.Selected == nil {
		return
	}
	selected := s.panel.Memory.Selected
	fields := cloneStringMap(s.panel.FieldValues)
	fields["address"] = "0x" + strconv.FormatUint(uint64(selected.Address), 16)
	fields["expected"] = selected.Expected
	s.executeToolAction("write", fields)
}

func (u *shellUI) addMemoryResults(shell *Shell, panel *Panel, form *widget.Container, previousInputs map[string]*imeTextInput) {
	design, model := u.design, panel.Memory
	addText := func(text string) {
		form.AddChild(design.text(text, design.Type.Body, design.Palette.Text, widget.RowLayoutData{Stretch: true}))
	}
	if model.Status != "" {
		addText(shell.trMemoryText(model.Status))
	}
	if !model.Active {
		addText(shell.tr("Choose a numeric type and region, then run a first scan."))
		return
	}
	addText(fmt.Sprintf("%s: %d %s", model.Type, model.Total, shell.tr("results")))
	if model.Selected != nil {
		selected := model.Selected
		addText(fmt.Sprintf("0x%08X  %s  %s = %s", selected.Address, selected.Region, selected.Type, selected.Value))
		if !selected.Writable {
			addText(shell.tr("Read-only region"))
		}
		input := newIMETextInput(design, imeTextInputConfig{
			Label: shell.tr("New value"), Placeholder: shell.tr("Enter a new numeric value"),
			Text: panel.FieldValues["new_value"], Disabled: panel.Busy || !selected.Writable,
			MinHeight: design.px(34), LayoutData: widget.RowLayoutData{Stretch: true},
			Changed: func(value string) { panel.FieldValues["new_value"] = value },
		})
		input.adoptNativeEdit(previousInputs["new_value"])
		u.panelTextInputs["new_value"] = input
		form.AddChild(input)
		button := design.button(shell.tr("Apply value"), design.Components.PrimaryButton, design.Type.Strong, 0, design.px(34), widget.TextPositionCenter, shell.writeSelectedMemory)
		button.GetWidget().Disabled = panel.Busy || !selected.Writable
		form.AddChild(button)
	}
	navigation := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(
		widget.RowLayoutOpts.Direction(widget.DirectionHorizontal), widget.RowLayoutOpts.Spacing(design.Space.S),
	)))
	for _, action := range []ToolAction{
		{ID: "previous", Label: "Previous", Enabled: model.Offset > 0},
		{ID: "refresh", Label: "Refresh", Enabled: true},
		{ID: "next", Label: "Next", Enabled: model.Offset+len(model.Results) < model.Total},
	} {
		action := action
		button := design.button(shell.tr(action.Label), design.Components.SubtleButton, design.Type.Strong, 0, design.px(34), widget.TextPositionCenter, func() {
			shell.executeToolAction(action.ID, panel.FieldValues)
		})
		button.GetWidget().Disabled = panel.Busy || !action.Enabled
		navigation.AddChild(button)
	}
	form.AddChild(navigation)
	if model.Total == 0 {
		addText(shell.tr("No matching addresses. Start a new search to try again."))
	} else {
		addText(fmt.Sprintf("%d–%d / %d", model.Offset+1, model.Offset+len(model.Results), model.Total))
	}
	for _, result := range model.Results {
		result := result
		label := fmt.Sprintf("0x%08X  %s  %s\n%s", result.Address, result.Type, result.Value, result.Region)
		style := design.Components.SubtleButton
		if model.Selected != nil && result.Address == model.Selected.Address {
			style = design.Components.PrimaryButton
		}
		button := design.button(label, style, design.Type.Body, 0, design.px(50), widget.TextPositionStart, func() { shell.selectMemoryResult(result) })
		button.GetWidget().LayoutData = widget.RowLayoutData{Stretch: true}
		button.GetWidget().Disabled = panel.Busy
		form.AddChild(button)
		u.memoryResultButtons = append(u.memoryResultButtons, button)
	}
}
