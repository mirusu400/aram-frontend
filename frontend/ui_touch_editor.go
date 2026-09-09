package frontend

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The touch-layout editor replaces the whole interface while it is open:
// every on-screen control is draggable and the only other targets are the
// three actions below.

const (
	touchEditorSave         = "editor-save"
	touchEditorReset        = "editor-reset"
	touchEditorCancel       = "editor-cancel"
	touchEditorGuestSmaller = "editor-guest-smaller"
	touchEditorGuestLarger  = "editor-guest-larger"
	touchEditorSizeSmaller  = "editor-size-smaller"
	touchEditorSizeLarger   = "editor-size-larger"
	touchEditorGridFiner    = "editor-grid-finer"
	touchEditorGridCoarser  = "editor-grid-coarser"
)

const (
	touchEditorActionHeight = 44
	touchEditorActionGap    = 16
	touchEditorStepperWidth = 56
	touchEditorStepperGap   = 8
	// touchEditorDeckStep and touchEditorSizeStep are one press of a stepper.
	touchEditorDeckStep = 5
	touchEditorSizeStep = 10
	// touchEditorGridStepDelta is one press of the grid stepper: the pixel
	// pitch coarsens or refines by this much, and stepping below the minimum
	// turns snapping off.
	touchEditorGridStepDelta = 8
	// touchEditorTrayChip is the size a put-away button shrinks to in the
	// tray, small enough that a full deck's worth still fits above the deck.
	touchEditorTrayChip = 44
	touchEditorTrayGap  = 6
)

// touchEditorStepperRow returns the y of one of the two stepper rows.
func touchEditorStepperRow(index int) int {
	return touchEditorStepperRowAtScale(index, 1)
}

func touchEditorStepperRowAtScale(index int, scale float64) int {
	return scaledPixels(
		touchEditorActionGap*2+touchEditorActionHeight+
			index*(touchEditorActionHeight+touchEditorStepperGap),
		scale,
	)
}

// touchLayoutEditorSteppers lays out the deck-ratio and button-size controls
// as a minus/plus pair per row, with the reading between them.
func touchLayoutEditorSteppers(width int) []touchButton {
	return touchLayoutEditorSteppersAtScale(width, 1)
}

func touchLayoutEditorSteppersAtScale(width int, scale float64) []touchButton {
	px := func(value int) int { return scaledPixels(value, scale) }
	rows := []struct{ minus, plus string }{
		{touchEditorGuestSmaller, touchEditorGuestLarger},
		{touchEditorSizeSmaller, touchEditorSizeLarger},
		{touchEditorGridFiner, touchEditorGridCoarser},
	}
	span := min(width-px(touchEditorActionGap)*2, px(420))
	left := max(px(touchEditorActionGap), (width-span)/2)
	buttons := make([]touchButton, 0, len(rows)*2)
	for index, row := range rows {
		y := px(touchEditorStepperRow(index))
		buttons = append(buttons,
			touchButton{
				ID:    row.minus,
				Label: "-",
				Bounds: rectAt(left, y,
					px(touchEditorStepperWidth), px(touchEditorActionHeight)),
			},
			touchButton{
				ID:    row.plus,
				Label: "+",
				Bounds: rectAt(left+span-px(touchEditorStepperWidth), y,
					px(touchEditorStepperWidth), px(touchEditorActionHeight)),
			},
		)
	}
	return buttons
}

// touchEditorTrayBounds is the strip that holds put-away buttons. Dropping a
// button in it hides the button; dragging one out brings it back.
func touchEditorTrayBounds(width, height int, options touchLayoutOptions) image.Rectangle {
	return touchEditorTrayBoundsAtScale(width, height, options, 1)
}

func touchEditorTrayBoundsAtScale(width, height int, options touchLayoutOptions, scale float64) image.Rectangle {
	px := func(value int) int { return scaledPixels(value, scale) }
	top := px(touchEditorStepperRow(3) + 24)
	deckTop := height - px(statusBarHeight) -
		touchDeckHeightWithRenderScale(width, height, options, scale)
	bottom := min(top+px(touchEditorTrayChip)*2+px(touchEditorTrayGap)*3, max(top+px(1), deckTop-px(8)))
	return rectAt(
		px(touchEditorActionGap),
		top,
		max(px(1), width-px(touchEditorActionGap)*2),
		max(px(1), bottom-top),
	)
}

// touchEditorTrayButtons places the hidden buttons inside the tray, wrapping
// onto more rows as needed.
func touchEditorTrayButtons(
	width, height int,
	options touchLayoutOptions,
) []touchButton {
	return touchEditorTrayButtonsAtScale(width, height, options, 1)
}

func touchEditorTrayButtonsAtScale(
	width, height int,
	options touchLayoutOptions,
	scale float64,
) []touchButton {
	if len(options.Hidden) == 0 {
		return nil
	}
	px := func(value int) int { return scaledPixels(value, scale) }
	tray := touchEditorTrayBoundsAtScale(width, height, options, scale)
	columns := max(1, (tray.Dx()-px(touchEditorTrayGap))/(px(touchEditorTrayChip)+px(touchEditorTrayGap)))
	buttons := make([]touchButton, 0, len(options.Hidden))
	index := 0
	for _, slot := range touchButtonCatalogAtScale(width, height, options, scale) {
		if !options.Hidden[slot.ID] {
			continue
		}
		column := index % columns
		row := index / columns
		buttons = append(buttons, touchButton{
			ID:      slot.ID,
			Control: slot.Control,
			Label:   slot.Label,
			Hidden:  true,
			Bounds: rectAt(
				tray.Min.X+px(touchEditorTrayGap)+column*(px(touchEditorTrayChip)+px(touchEditorTrayGap)),
				tray.Min.Y+px(touchEditorTrayGap)+row*(px(touchEditorTrayChip)+px(touchEditorTrayGap)),
				px(touchEditorTrayChip),
				px(touchEditorTrayChip),
			),
		})
		index++
	}
	return buttons
}

func touchLayoutEditorActions(width int) []touchButton {
	return touchLayoutEditorActionsAtScale(width, 1)
}

func touchLayoutEditorActionsAtScale(width int, scale float64) []touchButton {
	px := func(value int) int { return scaledPixels(value, scale) }
	buttonWidth := max(px(80), min(px(150), (width-px(touchEditorActionGap)*4)/3))
	total := buttonWidth*3 + px(touchEditorActionGap)*2
	x := max(px(touchEditorActionGap), (width-total)/2)
	y := px(touchEditorActionGap)
	step := buttonWidth + px(touchEditorActionGap)
	return []touchButton{
		{ID: touchEditorSave, Label: "Save", Bounds: rectAt(x, y, buttonWidth, px(touchEditorActionHeight))},
		{ID: touchEditorReset, Label: "Reset", Bounds: rectAt(x+step, y, buttonWidth, px(touchEditorActionHeight))},
		{ID: touchEditorCancel, Label: "Cancel", Bounds: rectAt(x+step*2, y, buttonWidth, px(touchEditorActionHeight))},
	}
}

func touchLayoutEditorActionAt(x, y, width int) (string, bool) {
	return touchLayoutEditorActionAtScale(x, y, width, 1)
}

func touchLayoutEditorActionAtScale(x, y, width int, scale float64) (string, bool) {
	for _, button := range touchLayoutEditorActionsAtScale(width, scale) {
		if pointInRect(x, y, button.Bounds) {
			return button.ID, true
		}
	}
	for _, button := range touchLayoutEditorSteppersAtScale(width, scale) {
		if pointInRect(x, y, button.Bounds) {
			return button.ID, true
		}
	}
	return "", false
}

func (s *Shell) drawTouchLayoutEditor(screen *ebiten.Image) {
	s.drawWorkspace(screen)
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	ebitenutil.DrawRect(
		screen,
		0,
		0,
		float64(width),
		float64(height),
		color.NRGBA{A: 110},
	)

	if step := s.px(s.touchEditorGridStep()); step > 0 {
		s.drawTouchEditorGrid(screen, width, height, step)
	}

	dragging := make(map[string]bool, len(s.touchLayoutDrag))
	for _, buttonID := range s.touchLayoutDrag {
		dragging[buttonID] = true
	}
	options := s.touchLayoutOptions()
	s.drawTouchEditorTray(screen, width, height, options, dragging)
	for _, button := range touchControlButtonsWithRenderScale(width, height, options, s.renderScale) {
		if button.ID == circularTouchPadID {
			s.drawCircularPadBounds(screen, button.Bounds)
			continue
		}
		s.drawTouchButton(screen, button, dragging[button.ID])
	}
	for _, button := range touchLayoutEditorActionsAtScale(width, s.renderScale) {
		s.drawTouchButton(screen, button, false)
	}
	for _, button := range touchLayoutEditorSteppersAtScale(width, s.renderScale) {
		s.drawTouchButton(screen, button, false)
	}
	if s.design == nil {
		return
	}
	gridReading := s.tr("Grid snap: off")
	if step := s.touchEditorGridStep(); step > 0 {
		gridReading = s.trf("Grid snap: %d px", step)
	}
	readings := []string{
		s.trf("Guest screen %d%%", 100-s.touchEditorDeckRatio()),
		s.trf("Button size %d%%", s.touchEditorScale()),
		gridReading,
	}
	for index, reading := range readings {
		row := rectAt(0, s.px(touchEditorStepperRow(index)), width, s.px(touchEditorActionHeight))
		drawCenteredText(
			screen,
			reading,
			s.design.Type.Strong,
			s.design.Palette.Text,
			row,
			centeredTextTop(s.design.Type.Strong, row, s.design.Type.CenterNudge),
		)
	}
	hintBounds := rectAt(0, s.px(touchEditorStepperRow(3)), width, s.px(20))
	drawCenteredText(
		screen,
		s.tr("Drag buttons to move them, or into the tray to put them away"),
		s.design.Type.Caption,
		s.design.Palette.Text,
		hintBounds,
		hintBounds.Min.Y,
	)
}

// drawTouchEditorGrid overlays the snap lattice a dragged button lands on, so
// the pixel pitch the stepper reports is something the user can actually see.
// It stops at the status bar because that band never holds a button.
func (s *Shell) drawTouchEditorGrid(screen *ebiten.Image, width, height, step int) {
	line := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x22}
	if s.design != nil {
		tint := s.design.Palette.BorderStrong
		line = color.NRGBA{R: tint.R, G: tint.G, B: tint.B, A: 0x30}
	}
	limit := height - s.px(statusBarHeight)
	for x := step; x < width; x += step {
		ebitenutil.DrawRect(screen, float64(x), 0, float64(s.px(1)), float64(limit), line)
	}
	for y := step; y < limit; y += step {
		ebitenutil.DrawRect(screen, 0, float64(y), float64(width), float64(s.px(1)), line)
	}
}

// drawTouchEditorTray marks the strip that puts buttons away and shows what is
// already in it. An empty tray still has to be visible, or dropping a button
// into it would be a guess.
func (s *Shell) drawTouchEditorTray(
	screen *ebiten.Image,
	width, height int,
	options touchLayoutOptions,
	dragging map[string]bool,
) {
	tray := touchEditorTrayBoundsAtScale(width, height, options, s.renderScale)
	if tray.Dy() < s.px(touchEditorTrayChip)/2 {
		return
	}
	border := color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xa0}
	fill := color.NRGBA{R: 0x10, G: 0x12, B: 0x18, A: 0xff}
	if s.design != nil {
		border = s.design.Palette.BorderStrong
		fill = s.design.Palette.CanvasRaised
	}
	ebitenutil.DrawRect(screen,
		float64(tray.Min.X), float64(tray.Min.Y),
		float64(tray.Dx()), float64(tray.Dy()), fill)
	drawRectOutlineAtScale(screen, tray, border, s.renderScale)
	if s.design != nil && len(options.Hidden) == 0 {
		label := rectAt(tray.Min.X, tray.Min.Y, tray.Dx(), tray.Dy())
		drawCenteredText(
			screen,
			s.tr("Put buttons here to hide them"),
			s.design.Type.Caption,
			s.design.Palette.TextMuted,
			label,
			centeredTextTop(s.design.Type.Caption, label, s.design.Type.CenterNudge),
		)
	}
	for _, button := range touchEditorTrayButtonsAtScale(width, height, options, s.renderScale) {
		s.drawTouchButton(screen, button, dragging[button.ID])
	}
}

// drawRectOutline draws a one-pixel frame, which the tray uses to read as a
// drop target rather than a panel.
func drawRectOutline(screen *ebiten.Image, bounds image.Rectangle, stroke color.Color) {
	drawRectOutlineAtScale(screen, bounds, stroke, 1)
}

func drawRectOutlineAtScale(screen *ebiten.Image, bounds image.Rectangle, stroke color.Color, scale float64) {
	x, y := float64(bounds.Min.X), float64(bounds.Min.Y)
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	line := float64(scaledPixels(1, scale))
	ebitenutil.DrawRect(screen, x, y, w, line, stroke)
	ebitenutil.DrawRect(screen, x, y+h-line, w, line, stroke)
	ebitenutil.DrawRect(screen, x, y, line, h, stroke)
	ebitenutil.DrawRect(screen, x+w-line, y, line, h, stroke)
}
