package frontend

import (
	"strings"

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
	// label and description. A compact panel has so little width to divide
	// that the column has to give up more of it, accepting a trimmed control
	// label to keep the label beside it readable.
	settingsActionMinWidth        = 92
	settingsActionMaxShare        = 0.45
	settingsCompactActionMinWidth = 72
	settingsCompactActionMaxShare = 0.34
	settingsCompactNavWidth       = 112
	// settingsMinCopyWidth is the least a label and description can be
	// given before a row is better off stacking its control underneath.
	settingsMinCopyWidth = 160
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
	minWidth, share := settingsActionMinWidth, settingsActionMaxShare
	if u.compact {
		minWidth, share = settingsCompactActionMinWidth, settingsCompactActionMaxShare
	}
	return clampInt(width, minWidth, int(float64(rows)*share))
}

// settingsRowStacks reports whether a row puts its control under the copy
// instead of beside it. The decision is made from what is actually left for
// the copy, not from the compact flag: a phone leaves too little either way,
// while a merely short desktop window still has room to sit them side by
// side, and stacking there would be a surprise.
func (u *shellUI) settingsRowStacks(design *ARAMDesignSystem, actionWidth int) bool {
	rows := u.settingsRowsWidth(design)
	return rows-design.Space.M-actionWidth-design.Space.L < settingsMinCopyWidth
}

// settingsCopyWidth is the width a row's label and description have to work
// with. Text wider than this wraps instead of stretching the row.
func (u *shellUI) settingsCopyWidth(design *ARAMDesignSystem, actionWidth int) int {
	rows := u.settingsRowsWidth(design)
	if u.settingsRowStacks(design, actionWidth) {
		return max(80, rows-2*design.Space.M)
	}
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

// fitWordsToWidth trims the words a wrap cannot break. Wrapping only splits on
// spaces, so a single long token — a component name, a path — keeps reporting
// its full width no matter the wrap budget, which is enough to widen the row
// and carry its control out of reach.
func fitWordsToWidth(label string, face *text.Face, maxWidth int) string {
	if label == "" || maxWidth <= 0 {
		return label
	}
	if width, _ := text.Measure(label, *face, 0); int(width) <= maxWidth {
		return label
	}
	words := strings.Fields(label)
	trimmed := false
	for index, word := range words {
		width, _ := text.Measure(word, *face, 0)
		if int(width) <= maxWidth {
			continue
		}
		words[index] = fitTextToWidth(word, face, maxWidth)
		trimmed = true
	}
	if !trimmed {
		return label
	}
	return strings.Join(words, " ")
}
