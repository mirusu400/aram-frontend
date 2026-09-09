package frontend

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type touchButton struct {
	// ID identifies a button slot for custom placements; two slots may
	// share one Control (both OK buttons) but never one ID.
	ID      string
	Control string
	Label   string
	Bounds  image.Rectangle
	// Hidden marks a button the user has put away. Only the layout editor
	// draws these, in its tray.
	Hidden bool
}

const (
	touchDeckPadding    = 12
	touchDeckCenterGap  = 24
	touchControlMinSize = 36
	touchControlMaxSize = 120
)

// touchLayoutOptions carries the user-configurable parts of the on-screen
// control layout: a size multiplier and per-button custom positions.
type touchLayoutOptions struct {
	Scale      float64
	Placements map[string]TouchPlacement
	// Circular replaces the five directional-cross slots with one movable
	// round-pad slot. Each mode keeps its own placement records.
	Circular bool
	// Keypad adds the numeric cluster to the deck. Feature-phone titles ask
	// for digits, star, and hash as often as they ask for a direction, and a
	// touch layout has no keyboard to fall back on.
	Keypad bool
	// DeckRatio is the share of screen height the deck claims, as a percent.
	// Zero keeps the automatic height, which only knows whether the keypad is
	// on; a title that needs a bigger picture more than it needs big buttons
	// can trade one for the other.
	DeckRatio int
	// Hidden drops buttons a title never uses. They keep their placement so
	// bringing one back does not lose where it sat.
	Hidden map[string]bool
}

func defaultTouchLayoutOptions() touchLayoutOptions {
	return touchLayoutOptions{Scale: 1}
}

func touchControlButtonsFor(width, height int) []touchButton {
	return touchControlButtonsWithOptions(width, height, defaultTouchLayoutOptions())
}

// touchDeckMetrics holds the derived geometry of the on-screen deck: the
// button size and gap and the anchor of each cluster. The button builder and
// the circular pad both read it, so the pad always sits exactly where the
// directional cross it replaces would.
type touchDeckMetrics struct {
	buttonSize int
	gap        int
	// scaled is the full user-scaled button size a repositioned button takes,
	// before the geometric fit clamps the default grid.
	scaled  int
	gridTop int
	dpadX   int
	dpadY   int
	actionX int
	actionY int
}

func touchDeckMetricsFor(width, height int, options touchLayoutOptions) touchDeckMetrics {
	return touchDeckMetricsForScale(width, height, options, 1)
}

func touchDeckMetricsForScale(width, height int, options touchLayoutOptions, renderScale float64) touchDeckMetrics {
	px := func(value int) int { return scaledPixels(value, renderScale) }
	deckHeight := touchDeckHeightWithRenderScale(width, height, options, renderScale)
	deckTop := height - px(statusBarHeight) - deckHeight
	margin := max(px(12), min(px(28), width/32))
	gap := max(px(6), min(px(12), width/96))
	// The D-pad and action clusters are both three columns wide. Cap the
	// button size by the horizontal room left after margins, the center
	// gap, and in-cluster gaps so the clusters can never overlap, and by
	// the vertical room for the rows on show — three for the clusters, four
	// more when the numeric keypad sits below them.
	rows := touchDeckRowCount(options)
	horizontalFit := (width - margin*2 - px(touchDeckCenterGap) - gap*4) / 6
	verticalFit := (deckHeight - px(touchDeckPadding)*2 - gap*(rows-1)) / rows
	// The floor is the smallest button the deck will draw. Anything higher
	// would let the grid outgrow a short deck and push its last row under the
	// status bar, which is what a hand-set deck ratio can produce.
	fit := max(px(touchControlMinSize), min(px(88), min(horizontalFit, verticalFit)))
	scale := options.Scale
	if scale <= 0 {
		scale = 1
	}
	scaled := clampInt(
		int(float64(fit)*scale+0.5),
		px(touchControlMinSize),
		px(touchControlMaxSize),
	)
	// Buttons still in the default grid stay within the geometric fit so
	// the clusters cannot collide; repositioned buttons take the full
	// scaled size because the user controls where they sit.
	buttonSize := min(scaled, fit)
	gridHeight := buttonSize*rows + gap*(rows-1)
	gridTop := deckTop + px(touchDeckPadding) +
		max(0, (deckHeight-px(touchDeckPadding)*2-gridHeight)/2)
	return touchDeckMetrics{
		buttonSize: buttonSize,
		gap:        gap,
		scaled:     scaled,
		gridTop:    gridTop,
		dpadX:      margin + buttonSize,
		dpadY:      gridTop + buttonSize + gap,
		actionX:    width - margin - buttonSize*3 - gap*2,
		actionY:    gridTop + buttonSize + gap,
	}
}

func touchControlButtonsWithOptions(
	width, height int,
	options touchLayoutOptions,
) []touchButton {
	return touchControlButtonsWithRenderScale(width, height, options, 1)
}

func touchControlButtonsWithRenderScale(
	width, height int,
	options touchLayoutOptions,
	renderScale float64,
) []touchButton {
	metrics := touchDeckMetricsForScale(width, height, options, renderScale)
	buttonSize := metrics.buttonSize
	gap := metrics.gap
	scaled := metrics.scaled
	dpadX, dpadY := metrics.dpadX, metrics.dpadY
	actionX, actionY := metrics.actionX, metrics.actionY

	directions := []touchButton{
		{ID: "up", Control: "up", Label: "UP", Bounds: rectAt(dpadX, dpadY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "left", Control: "left", Label: "LEFT", Bounds: rectAt(dpadX-buttonSize-gap, dpadY, buttonSize, buttonSize)},
		{ID: "ok", Control: "ok", Label: "OK", Bounds: rectAt(dpadX, dpadY, buttonSize, buttonSize)},
		{ID: "right", Control: "right", Label: "RIGHT", Bounds: rectAt(dpadX+buttonSize+gap, dpadY, buttonSize, buttonSize)},
		{ID: "down", Control: "down", Label: "DOWN", Bounds: rectAt(dpadX, dpadY+buttonSize+gap, buttonSize, buttonSize)},
	}
	if options.Circular {
		center, radius := circularPadCircle(metrics)
		directions = []touchButton{{
			ID:      circularTouchPadID,
			Control: circularTouchPadID,
			Label:   "D-PAD",
			Bounds: rectAt(
				center.X-radius,
				center.Y-radius,
				radius*2,
				radius*2,
			),
		}}
	}
	buttons := append(directions, []touchButton{
		{ID: "soft-left", Control: "soft-left", Label: "L", Bounds: rectAt(actionX, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "soft-right", Control: "soft-right", Label: "CANCEL", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "back", Control: "back", Label: "C", Bounds: rectAt(actionX, actionY, buttonSize, buttonSize)},
		{ID: "menu", Control: "menu", Label: "MENU", Bounds: rectAt(actionX+buttonSize+gap, actionY, buttonSize, buttonSize)},
		{ID: "ok-action", Control: "ok", Label: "OK", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY, buttonSize, buttonSize)},
	}...)
	if options.Keypad {
		buttons = append(buttons, numericTouchButtons(
			width, metrics.gridTop+buttonSize*3+gap*3, buttonSize, gap)...)
	}
	buttons = visibleTouchButtons(buttons, options.Hidden)
	for index := range buttons {
		placement, ok := options.Placements[buttons[index].ID]
		if !ok {
			continue
		}
		placementSize := scaled
		if buttons[index].ID == circularTouchPadID {
			placementSize = scaled*3 + gap*2
		}
		buttons[index].Bounds = placedTouchBoundsAtScale(
			placement, placementSize, width, height, renderScale,
		)
	}
	return buttons
}

// placedTouchBounds converts a normalized center placement into pixel
// bounds, clamped so the whole button stays on the interactive screen.
func placedTouchBounds(
	placement TouchPlacement,
	size, width, height int,
) image.Rectangle {
	return placedTouchBoundsAtScale(placement, size, width, height, 1)
}

func placedTouchBoundsAtScale(
	placement TouchPlacement,
	size, width, height int,
	renderScale float64,
) image.Rectangle {
	half := size / 2
	centerX := clampInt(int(placement.X*float64(width)+0.5), half, max(half, width-size+half))
	centerY := clampInt(
		int(placement.Y*float64(height)+0.5),
		half,
		max(half, height-scaledPixels(statusBarHeight, renderScale)-size+half),
	)
	return rectAt(centerX-half, centerY-half, size, size)
}

// snapToGrid rounds a screen coordinate to the nearest multiple of step, which
// the layout editor uses to align a dragged button. A step of zero (the grid
// off) leaves the coordinate alone.
func snapToGrid(v, step int) int {
	if step <= 0 {
		return v
	}
	if v >= 0 {
		return ((v + step/2) / step) * step
	}
	return -((-v + step/2) / step) * step
}

// normalizedTouchPlacement is the inverse of placedTouchBounds for a button
// center dragged to x, y.
func normalizedTouchPlacement(x, y, width, height int) TouchPlacement {
	if width <= 0 || height <= 0 {
		return TouchPlacement{X: 0.5, Y: 0.5}
	}
	return TouchPlacement{
		X: min(1, max(0, float64(x)/float64(width))),
		Y: min(1, max(0, float64(y)/float64(height))),
	}
}

func touchDeckHeight(width, height int) int {
	return touchDeckHeightWithOptions(width, height, defaultTouchLayoutOptions())
}

// touchDeckHeightWithOptions reserves the deck. The numeric cluster needs four
// more rows than the direction and action clusters alone, so enabling it takes
// height from the guest viewport rather than crowding the existing buttons.
func touchDeckHeightWithOptions(
	width, height int,
	options touchLayoutOptions,
) int {
	return touchDeckHeightWithRenderScale(width, height, options, 1)
}

func touchDeckHeightWithRenderScale(
	width, height int,
	options touchLayoutOptions,
	renderScale float64,
) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	if ratio := options.DeckRatio; ratio > 0 {
		ratio = clampInt(ratio, touchDeckRatioMin, touchDeckRatioMax)
		// A ratio that cannot seat every row is raised to one that can: the
		// alternative is a row drawn under the status bar, unreachable.
		return min(
			height-scaledPixels(statusBarHeight, renderScale),
			max(minimumTouchDeckHeightAtScale(options, renderScale), height*ratio/100),
		)
	}
	if options.Keypad {
		return min(scaledPixels(560, renderScale), max(scaledPixels(320, renderScale), height*54/100))
	}
	return min(scaledPixels(280, renderScale), max(scaledPixels(190, renderScale), height*32/100))
}

// touchDeckRatioPercent reports the deck's share of the screen, whether it
// came from the setting or from the automatic height. The editor shows this
// number, so it has to describe the deck actually on screen.
func touchDeckRatioPercent(width, height int, options touchLayoutOptions) int {
	if height <= 0 {
		return touchDeckRatioMin
	}
	deck := touchDeckHeightWithOptions(width, height, options)
	return clampInt(deck*100/height, touchDeckRatioMin, touchDeckRatioMax)
}

// touchChromeToggleBounds places the floating chrome toggle at the
// top-right. With the chrome hidden it is a small translucent hamburger
// floating over the guest viewport; with the chrome visible it is a HIDE
// button inside the otherwise empty right end of the toolbar, because narrow
// phone widths fill the whole menu bar with menu titles.
func touchChromeToggleBounds(width int, hidden bool) image.Rectangle {
	return touchChromeToggleBoundsAtScale(width, hidden, 1)
}

func touchChromeToggleBoundsAtScale(width int, hidden bool, scale float64) image.Rectangle {
	px := func(value int) int { return scaledPixels(value, scale) }
	if hidden {
		return rectAt(width-px(12)-px(44), px(12), px(44), px(44))
	}
	return rectAt(
		width-px(8)-px(76),
		px(menuBarHeight)+px(applicationToolbarHeight-toolbarButtonHeight)/2,
		px(76),
		px(toolbarButtonHeight),
	)
}

func rectAt(x, y, width, height int) image.Rectangle {
	return image.Rect(x, y, x+width, y+height)
}

func touchControlAtSize(x, y, width, height int) (string, bool) {
	button, ok := touchButtonAtWithOptions(x, y, width, height, defaultTouchLayoutOptions())
	if !ok {
		return "", false
	}
	return button.Control, true
}

func touchButtonAtWithOptions(
	x, y, width, height int,
	options touchLayoutOptions,
) (touchButton, bool) {
	return touchButtonAtWithRenderScale(x, y, width, height, options, 1)
}

func touchButtonAtWithRenderScale(
	x, y, width, height int,
	options touchLayoutOptions,
	renderScale float64,
) (touchButton, bool) {
	for _, button := range touchControlButtonsWithRenderScale(width, height, options, renderScale) {
		if pointInRect(x, y, button.Bounds) {
			return button, true
		}
	}
	return touchButton{}, false
}

func pointInRect(x, y int, bounds image.Rectangle) bool {
	return image.Pt(x, y).In(bounds)
}

func (s *Shell) drawTouchControls(screen *ebiten.Image) {
	if !platformUsesTouchLayout() {
		return
	}
	if s.onScreenControlsHidden() {
		// A second panel or a connected controller owns the controls; the game
		// panel shows nothing over the guest screen.
		return
	}
	active := make(map[string]bool)
	for _, control := range s.touchControls {
		active[control] = true
	}
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	options := s.touchLayoutOptions()
	buttons := touchControlButtonsWithRenderScale(width, height, options, s.renderScale)
	for _, button := range buttons {
		if button.ID == circularTouchPadID {
			continue
		}
		s.drawTouchButton(screen, button, active[button.Control])
	}
	if options.Circular {
		s.drawCircularPad(screen, width, height, options)
	}
}

// drawTouchChromeToggle draws the floating control that hides the shell
// chrome for a full-size guest screen and brings it back afterwards. Over
// the game it is a faint hamburger glyph so it does not compete with play;
// inside the chrome it is a regular HIDE button.
func (s *Shell) drawTouchChromeToggle(screen *ebiten.Image) {
	if !s.touchChromeToggleAvailable() {
		return
	}
	width := screen.Bounds().Dx()
	if s.touchChromeHiddenActive() {
		s.drawHamburgerToggle(screen, touchChromeToggleBoundsAtScale(width, true, s.renderScale))
		return
	}
	s.drawTouchButton(screen, touchButton{
		Label:  "HIDE",
		Bounds: touchChromeToggleBoundsAtScale(width, false, s.renderScale),
	}, false)
}

// drawHamburgerToggle draws a translucent square with three menu bars.
func (s *Shell) drawHamburgerToggle(
	screen *ebiten.Image,
	bounds image.Rectangle,
) {
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	ebitenutil.DrawRect(
		screen,
		float64(bounds.Min.X),
		float64(bounds.Min.Y),
		float64(bounds.Dx()),
		float64(bounds.Dy()),
		fadeColor(palette.SurfaceRaised, 0.25),
	)
	barColor := fadeColor(palette.Text, 0.55)
	barWidth := bounds.Dx() - s.px(24)
	barHeight := s.px(3)
	centerY := bounds.Min.Y + bounds.Dy()/2
	for _, offset := range []int{-s.px(8), 0, s.px(8)} {
		ebitenutil.DrawRect(
			screen,
			float64(bounds.Min.X+s.px(12)),
			float64(centerY+offset-barHeight/2),
			float64(barWidth),
			float64(barHeight),
			barColor,
		)
	}
}

// fadeColor scales a color's premultiplied channels toward transparency.
func fadeColor(base color.Color, opacity float64) color.Color {
	red, green, blue, alpha := base.RGBA()
	return color.RGBA64{
		R: uint16(float64(red) * opacity),
		G: uint16(float64(green) * opacity),
		B: uint16(float64(blue) * opacity),
		A: uint16(float64(alpha) * opacity),
	}
}

func (s *Shell) drawTouchButton(screen *ebiten.Image, button touchButton, active bool) {
	bounds := button.Bounds
	label := s.tr(button.Label)
	if s.design != nil {
		key := s.design.Components.TouchButton
		surface := key.Image.Idle
		// The key's own ink roles, not the muted body text role: a legend has
		// to stay readable against the key face it sits on, and a pressed face
		// filled with the accent needs a different ink from an idle one.
		textColor := key.Text.Idle
		if active {
			surface = key.Image.Pressed
			textColor = key.Text.Pressed
		}
		surface.Draw(screen, bounds.Dx(), bounds.Dy(), func(options *ebiten.DrawImageOptions) {
			options.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
		})
		if direction := directionGlyphFor(button.Control); direction != "" {
			drawDirectionGlyph(screen, bounds, direction, textColor)
			return
		}
		face := s.design.Type.Strong
		top := centeredTextTop(face, bounds, s.design.Type.CenterNudge)
		if shadow, ok := retroKeyLegendShadow(s.design.Family, s.design.Palette); ok {
			drawCenteredText(
				screen, label, face, shadow,
				bounds.Add(image.Pt(s.px(1), s.px(1))), top+s.px(1),
			)
		}
		drawCenteredText(screen, label, face, textColor, bounds, top)
		return
	}
	palette := defaultARAMPalette()
	ebitenutil.DrawRect(
		screen,
		float64(bounds.Min.X),
		float64(bounds.Min.Y),
		float64(bounds.Dx()),
		float64(bounds.Dy()),
		palette.SurfaceRaised,
	)
	textX := bounds.Min.X + (bounds.Dx()-len([]rune(label))*6)/2
	textY := bounds.Min.Y + (bounds.Dy()-8)/2
	ebitenutil.DebugPrintAt(screen, label, textX, textY)
}

// numericTouchKeys is the handset keypad in reading order: three columns of
// digits and the star/zero/hash row that closes them.
var numericTouchKeys = [4][3]struct{ id, label string }{
	{{"num1", "1"}, {"num2", "2"}, {"num3", "3"}},
	{{"num4", "4"}, {"num5", "5"}, {"num6", "6"}},
	{{"num7", "7"}, {"num8", "8"}, {"num9", "9"}},
	{{"star", "*"}, {"num0", "0"}, {"hash", "#"}},
}

// numericTouchButtons lays the keypad out as a centered 3x4 grid starting at
// top. Every key is an ordinary deck button, so the layout editor can move it
// and a saved placement overrides this default like any other.
func numericTouchButtons(width, top, size, gap int) []touchButton {
	gridWidth := size*3 + gap*2
	left := max(0, (width-gridWidth)/2)
	buttons := make([]touchButton, 0, 12)
	for row, keys := range numericTouchKeys {
		for column, key := range keys {
			buttons = append(buttons, touchButton{
				ID:      key.id,
				Control: key.id,
				Label:   key.label,
				Bounds: rectAt(
					left+column*(size+gap),
					top+row*(size+gap),
					size,
					size,
				),
			})
		}
	}
	return buttons
}

// visibleTouchButtons drops the buttons the user has put away. The deck the
// player touches never carries them; only the layout editor still shows them,
// in its tray.
func visibleTouchButtons(
	buttons []touchButton,
	hidden map[string]bool,
) []touchButton {
	if len(hidden) == 0 {
		return buttons
	}
	kept := buttons[:0]
	for _, button := range buttons {
		if hidden[button.ID] {
			continue
		}
		kept = append(kept, button)
	}
	return kept
}

// touchButtonCatalog is every slot the deck can hold for these options,
// hidden ones included, in deck order. The editor needs the full set and
// settings normalization needs the vocabulary.
func touchButtonCatalog(width, height int, options touchLayoutOptions) []touchButton {
	return touchButtonCatalogAtScale(width, height, options, 1)
}

func touchButtonCatalogAtScale(width, height int, options touchLayoutOptions, scale float64) []touchButton {
	full := options
	full.Hidden = nil
	return touchControlButtonsWithRenderScale(width, height, full, scale)
}

// isTouchButtonID reports whether an id names a deck slot. Sizes do not matter
// here — the slots are the same whatever the screen.
func isTouchButtonID(id string) bool {
	if id == circularTouchPadID {
		return true
	}
	options := defaultTouchLayoutOptions()
	options.Keypad = true
	for _, button := range touchButtonCatalog(1080, 1920, options) {
		if button.ID == id {
			return true
		}
	}
	return false
}

// touchDeckRowCount is how many button rows the deck has to seat.
func touchDeckRowCount(options touchLayoutOptions) int {
	if options.Keypad {
		return 7
	}
	return 3
}

// minimumTouchDeckHeight is the shortest deck that still fits every row at the
// smallest button the deck draws, using the tightest gap any width produces.
func minimumTouchDeckHeight(options touchLayoutOptions) int {
	return minimumTouchDeckHeightAtScale(options, 1)
}

func minimumTouchDeckHeightAtScale(options touchLayoutOptions, scale float64) int {
	tightestGap := scaledPixels(6, scale)
	rows := touchDeckRowCount(options)
	return scaledPixels(touchControlMinSize, scale)*rows + tightestGap*(rows-1) + scaledPixels(touchDeckPadding, scale)*2
}
