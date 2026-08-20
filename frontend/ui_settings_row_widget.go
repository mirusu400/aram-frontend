package frontend

import (
	"image"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

// buildSettingsRow renders one row. actionWidth is measured once per section
// so every row reserves the same space for its control; the label and
// description wrap inside what is left instead of stretching the row wider
// than the panel, which used to push the control out of view entirely.
func (u *shellUI) buildSettingsRow(
	model settingsRowModel,
	actionWidth int,
) *widget.Container {
	design := u.design
	sliderWidth := 150
	if u.viewportWidth < 600 {
		sliderWidth = 110
	}
	const sliderValueWidth = 56
	if model.slider != nil {
		actionWidth = sliderWidth + design.Space.S + sliderValueWidth
	}
	copyWidth := u.settingsCopyWidth(design, actionWidth)
	actionLabelWidth := actionWidth - 2*design.Space.S
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
	copyBlock.AddChild(design.wrappedText(
		u.owner.tr(model.label),
		design.Type.Strong,
		design.Palette.Text,
		copyWidth,
		nil,
	))
	if !u.compact {
		copyBlock.AddChild(
			design.wrappedText(
				u.owner.tr(model.description),
				design.Type.Caption,
				design.Palette.TextMuted,
				copyWidth,
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
		// The closed button has only the column to work with; the open list
		// gets the full entry width.
		buttonLabel := func(entry any) string {
			return fitTextToWidth(entryLabel(entry), design.Type.Strong, actionLabelWidth)
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
					Padding:            &widget.Insets{Right: design.Space.XS},
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
					HTextPosition: widget.TextPositionEnd,
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
					HandleImage:   design.Components.SliderHandle,
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
			widget.ListComboButtonOpts.EntryLabelFunc(buttonLabel, entryLabel),
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
		valueLabel := widget.NewText(
			widget.TextOpts.Text(sliderModel.format(current), design.Type.Strong, design.Palette.Text),
			widget.TextOpts.Position(widget.TextPositionEnd, widget.TextPositionCenter),
			widget.TextOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
				widget.WidgetOpts.MinSize(sliderValueWidth, 0),
			),
		)
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
			fitTextToWidth(u.owner.tr(model.value), design.Type.Strong, actionLabelWidth),
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
		fitTextToWidth(u.owner.tr(model.value), design.Type.Strong, actionLabelWidth),
		design.Components.SubtleButton,
		design.Type.Strong,
		actionWidth,
		32,
		widget.TextPositionEnd,
		func() {
			action()
			u.panelSignature = ""
		},
	)
	valueButton.GetWidget().Disabled = model.disabled
	// Right inset of XS plus the button's own text padding lines the label up
	// with the Space.M edge used by the static value rows.
	valueButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionCenter,
		Padding:            &widget.Insets{Right: design.Space.XS},
	}
	row.AddChild(valueButton)
	return row
}
