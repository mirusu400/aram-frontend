package frontend

import (
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// The settings panel is laid out for the modern 13px type ramp: a 700×560
// dialog, a 168px nav rail, and a fixed-width action column on the right of
// every row. A sprite skin swaps in a 20px pixel ramp, and at that size the
// descriptions outgrew the row, widened the scroll content past the panel, and
// carried the right-anchored buttons off screen where they could not be
// clicked. These helpers derive the same geometry from the active ramp instead
// of hardcoding the modern one.
const (
	settingsBaseWindowWidth  = 700
	settingsBaseWindowHeight = 560
	// settingsBaseLineHeight is the modern body ramp's line height, the size
	// the constants above were chosen for.
	settingsBaseLineHeight = 16.0
	// settingsActionMinWidth keeps narrow labels from collapsing the action
	// column; settingsActionMaxShare stops a long one from crowding out the
	// label and description.
	settingsActionMinWidth  = 92
	settingsActionMaxShare  = 0.45
	settingsCompactNavWidth = 112
)

// typeRampScale reports how much larger the active body ramp is than the one
// the settings geometry was sized for.
func typeRampScale(design *ARAMDesignSystem) float64 {
	if design == nil || design.Type.Body == nil {
		return 1
	}
	_, lineHeight := text.Measure(" ", *design.Type.Body, 0)
	if lineHeight <= 0 {
		return 1
	}
	scale := lineHeight / settingsBaseLineHeight
	if scale < 1 {
		return 1
	}
	return scale
}

// settingsWindowSize grows the dialog with the type ramp. centeredWindowRect
// clamps the result to the viewport, so a large ramp on a small window simply
// fills it.
func settingsWindowSize(design *ARAMDesignSystem) (int, int) {
	scale := typeRampScale(design)
	return int(float64(settingsBaseWindowWidth) * scale),
		int(float64(settingsBaseWindowHeight) * scale)
}

func (u *shellUI) settingsNavRailWidth() int {
	if u.viewportWidth < 600 {
		return settingsCompactNavWidth
	}
	return settingsNavWidth
}

func (u *shellUI) settingsContentLeft(design *ARAMDesignSystem) int {
	if u.compact {
		return u.settingsNavRailWidth() + design.Space.M
	}
	return u.settingsNavRailWidth() + design.Space.XL
}

// settingsRowsWidth is the width a settings row is stretched to, which is what
// the row's own children have to share.
func (u *shellUI) settingsRowsWidth(design *ARAMDesignSystem) int {
	width, _ := settingsWindowSize(design)
	if u.viewportWidth > 0 {
		width = min(width, max(1, u.viewportWidth-2*centeredWindowMargin))
	}
	return max(1, width-u.settingsContentLeft(design)-design.Space.L)
}

// settingsActionWidth sizes the right-hand action column so the widest label
// in the section fits inside it.
func (u *shellUI) settingsActionWidth(shell *Shell, models []settingsRowModel) int {
	design := u.design
	widest := 0.0
	measure := func(label string) {
		if label == "" {
			return
		}
		w, _ := text.Measure(label, *design.Type.Strong, 0)
		if w > widest {
			widest = w
		}
	}
	for _, model := range models {
		measure(shell.tr(model.value))
		if model.dropdown != nil {
			for index := 0; index < model.dropdown.count; index++ {
				measure(model.dropdown.label(index))
			}
		}
	}
	// Both the value buttons and the dropdown buttons carry horizontal text
	// padding, and the dropdown needs room for its own frame.
	width := int(widest) + 2*design.Space.M
	rows := u.settingsRowsWidth(design)
	return clampInt(width, settingsActionMinWidth, int(float64(rows)*settingsActionMaxShare))
}

// settingsCopyWidth is what is left for a row's label and description once the
// action column and the row's own padding are taken out. Text wider than this
// wraps instead of stretching the row.
func (u *shellUI) settingsCopyWidth(design *ARAMDesignSystem, actionWidth int) int {
	rows := u.settingsRowsWidth(design)
	return max(80, rows-design.Space.M-actionWidth-design.Space.L)
}

// fitTextToWidth trims a label with an ellipsis until it fits. The action
// column is clamped to a share of the row, so a long value — a download path,
// a device name — would otherwise widen its button back over the label and
// description it was measured to sit beside.
func fitTextToWidth(label string, face *text.Face, maxWidth int) string {
	if label == "" || maxWidth <= 0 {
		return label
	}
	if width, _ := text.Measure(label, *face, 0); int(width) <= maxWidth {
		return label
	}
	runes := []rune(label)
	for length := len(runes) - 1; length > 0; length-- {
		candidate := string(runes[:length]) + "…"
		if width, _ := text.Measure(candidate, *face, 0); int(width) <= maxWidth {
			return candidate
		}
	}
	return "…"
}
