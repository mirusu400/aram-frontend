package frontend

import (
	"fmt"
	"image"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

func intPointer(value int) *int {
	return &value
}

func newToolFieldDropdown(
	design *ARAMDesignSystem,
	panel *Panel,
	field ToolField,
	translateLabel func(string) string,
) *widget.ListComboButton {
	entries := make([]any, len(field.Options))
	initial := field.Options[0]
	current := panel.FieldValues[field.ID]
	for index, option := range field.Options {
		entries[index] = option
		if option.Value == current {
			initial = option
		}
	}
	if panel.FieldValues == nil {
		panel.FieldValues = make(map[string]string)
	}
	panel.FieldValues[field.ID] = initial.Value

	trackIdle := euiimage.NewNineSliceColor(design.Palette.Border)
	trackHover := euiimage.NewNineSliceColor(design.Palette.BorderStrong)
	return widget.NewListComboButton(
		widget.ListComboButtonOpts.Entries(entries),
		widget.ListComboButtonOpts.InitialEntry(initial),
		widget.ListComboButtonOpts.MaxContentHeight(132),
		widget.ListComboButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MinSize(0, 34),
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
			TextFace:  design.Type.Body,
			TextPadding: &widget.Insets{
				Left: design.Space.S, Right: design.Space.S,
			},
			TextPosition: &widget.TextPositioning{
				HTextPosition: widget.TextPositionStart,
				VTextPosition: widget.TextPositionCenter,
			},
			MinSize: &image.Point{Y: 34},
		}),
		widget.ListComboButtonOpts.ListParams(&widget.ListParams{
			ScrollContainerImage: design.Components.Scroll,
			Slider: &widget.SliderParams{
				TrackImage: &widget.SliderTrackImage{
					Idle:     trackIdle,
					Hover:    trackHover,
					Disabled: trackIdle,
				},
				HandleImage:   design.Components.TouchButton.Image,
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
				Top:    design.Space.S,
				Right:  design.Space.S,
				Bottom: design.Space.S,
			},
			MinSize: &image.Point{X: 240},
		}),
		widget.ListComboButtonOpts.EntryLabelFunc(
			func(entry any) string {
				option, _ := entry.(ToolFieldOption)
				return translateLabel(option.Label)
			},
			func(entry any) string {
				option, _ := entry.(ToolFieldOption)
				return translateLabel(option.Label)
			},
		),
		widget.ListComboButtonOpts.EntrySelectedHandler(
			func(args *widget.ListComboButtonEntrySelectedEventArgs) {
				option, ok := args.Entry.(ToolFieldOption)
				if !ok {
					return
				}
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = option.Value
			},
		),
	)
}

func newToolFieldCheckbox(
	design *ARAMDesignSystem,
	shell *Shell,
	panel *Panel,
	field ToolField,
	translateLabel func(string) string,
) *widget.Checkbox {
	value := field.Value
	if current, ok := panel.FieldValues[field.ID]; ok {
		value = current
	}
	checked := strings.EqualFold(strings.TrimSpace(value), "true")
	if panel.FieldValues == nil {
		panel.FieldValues = make(map[string]string)
	}
	panel.FieldValues[field.ID] = fmt.Sprintf("%t", checked)
	initialState := widget.WidgetUnchecked
	if checked {
		initialState = widget.WidgetChecked
	}
	options := []widget.CheckboxOpt{}
	if panel.AllowGuestInput {
		// The same keys reach the guest while this panel is open, so a focused
		// control must not also answer to them.
		options = append(options, widget.CheckboxOpts.DisableDefaultKeys())
	}
	return widget.NewCheckbox(append(options,
		widget.CheckboxOpts.Image(design.Components.Checkbox),
		widget.CheckboxOpts.InitialState(initialState),
		widget.CheckboxOpts.Spacing(design.Space.S),
		widget.CheckboxOpts.Text(
			translateLabel(field.Label),
			design.Type.Body,
			&widget.LabelColor{
				Idle:     design.Palette.Text,
				Disabled: design.Palette.TextDisabled,
			},
		),
		widget.CheckboxOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MinSize(0, 34),
		),
		widget.CheckboxOpts.StateChangedHandler(
			func(args *widget.CheckboxChangedEventArgs) {
				if panel.FieldValues == nil {
					panel.FieldValues = make(map[string]string)
				}
				panel.FieldValues[field.ID] = fmt.Sprintf(
					"%t",
					args.State == widget.WidgetChecked,
				)
				if field.Action != "" {
					shell.executeToolAction(field.Action, panel.FieldValues)
				}
			},
		),
	)...)
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// scrollContainerByWheel moves sc by wheelY notches, one notch covering a
// fixed pixel step regardless of how tall the scrolled content is.
func scrollContainerByWheel(sc *widget.ScrollContainer, wheelY float64) {
	overflow := float64(sc.ContentRect().Dy() - sc.ViewRect().Dy())
	if overflow <= 0 {
		return
	}
	const wheelScrollStep = 48
	top := sc.ScrollTop - wheelY*wheelScrollStep/overflow
	if top < 0 {
		top = 0
	} else if top > 1 {
		top = 1
	}
	sc.ScrollTop = top
}

// centeredWindowMargin is the gap a modal keeps from the viewport edge.
const centeredWindowMargin = 18

func centeredWindowRect(viewWidth, viewHeight, preferredWidth, preferredHeight int) image.Rectangle {
	if viewWidth <= 0 || viewHeight <= 0 {
		viewWidth, viewHeight = logicalWidth, logicalHeight
	}
	margin := centeredWindowMargin
	width := min(preferredWidth, max(1, viewWidth-margin*2))
	height := min(preferredHeight, max(1, viewHeight-margin*2))
	x := max(0, (viewWidth-width)/2)
	y := max(0, (viewHeight-height)/2)
	return image.Rect(x, y, x+width, y+height)
}
