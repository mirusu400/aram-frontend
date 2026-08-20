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
}

func defaultTouchLayoutOptions() touchLayoutOptions {
	return touchLayoutOptions{Scale: 1}
}

func touchControlButtonsFor(width, height int) []touchButton {
	return touchControlButtonsWithOptions(width, height, defaultTouchLayoutOptions())
}

func touchControlButtonsWithOptions(
	width, height int,
	options touchLayoutOptions,
) []touchButton {
	deckHeight := touchDeckHeight(width, height)
	deckTop := height - statusBarHeight - deckHeight
	margin := max(12, min(28, width/32))
	gap := max(6, min(12, width/96))
	// The D-pad and action clusters are both three columns wide. Cap the
	// button size by the horizontal room left after margins, the center
	// gap, and in-cluster gaps so the clusters can never overlap, and by
	// the vertical room for three rows.
	horizontalFit := (width - margin*2 - touchDeckCenterGap - gap*4) / 6
	verticalFit := (deckHeight - touchDeckPadding*2 - gap*2) / 3
	fit := max(40, min(88, min(horizontalFit, verticalFit)))
	scale := options.Scale
	if scale <= 0 {
		scale = 1
	}
	scaled := clampInt(
		int(float64(fit)*scale+0.5),
		touchControlMinSize,
		touchControlMaxSize,
	)
	// Buttons still in the default grid stay within the geometric fit so
	// the clusters cannot collide; repositioned buttons take the full
	// scaled size because the user controls where they sit.
	buttonSize := min(scaled, fit)
	gridHeight := buttonSize*3 + gap*2
	gridTop := deckTop + touchDeckPadding +
		max(0, (deckHeight-touchDeckPadding*2-gridHeight)/2)
	dpadX := margin + buttonSize
	dpadY := gridTop + buttonSize + gap
	actionX := width - margin - buttonSize*3 - gap*2
	actionY := dpadY

	buttons := []touchButton{
		{ID: "up", Control: "up", Label: "UP", Bounds: rectAt(dpadX, dpadY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "left", Control: "left", Label: "LEFT", Bounds: rectAt(dpadX-buttonSize-gap, dpadY, buttonSize, buttonSize)},
		{ID: "ok", Control: "ok", Label: "OK", Bounds: rectAt(dpadX, dpadY, buttonSize, buttonSize)},
		{ID: "right", Control: "right", Label: "RIGHT", Bounds: rectAt(dpadX+buttonSize+gap, dpadY, buttonSize, buttonSize)},
		{ID: "down", Control: "down", Label: "DOWN", Bounds: rectAt(dpadX, dpadY+buttonSize+gap, buttonSize, buttonSize)},
		{ID: "soft-left", Control: "soft-left", Label: "L", Bounds: rectAt(actionX, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "soft-right", Control: "soft-right", Label: "R", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY-buttonSize-gap, buttonSize, buttonSize)},
		{ID: "back", Control: "back", Label: "BACK", Bounds: rectAt(actionX, actionY, buttonSize, buttonSize)},
		{ID: "menu", Control: "menu", Label: "MENU", Bounds: rectAt(actionX+buttonSize+gap, actionY, buttonSize, buttonSize)},
		{ID: "ok-action", Control: "ok", Label: "OK", Bounds: rectAt(actionX+buttonSize*2+gap*2, actionY, buttonSize, buttonSize)},
	}
	for index := range buttons {
		placement, ok := options.Placements[buttons[index].ID]
		if !ok {
			continue
		}
		buttons[index].Bounds = placedTouchBounds(placement, scaled, width, height)
	}
	return buttons
}

// placedTouchBounds converts a normalized center placement into pixel
// bounds, clamped so the whole button stays on the interactive screen.
func placedTouchBounds(
	placement TouchPlacement,
	size, width, height int,
) image.Rectangle {
	half := size / 2
	centerX := clampInt(int(placement.X*float64(width)+0.5), half, max(half, width-size+half))
	centerY := clampInt(
		int(placement.Y*float64(height)+0.5),
		half,
		max(half, height-statusBarHeight-size+half),
	)
	return rectAt(centerX-half, centerY-half, size, size)
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
	if width <= 0 || height <= 0 {
		return 0
	}
	return min(280, max(190, height*32/100))
}

// touchChromeToggleBounds places the floating chrome toggle at the
// top-right. With the chrome hidden it is a small translucent hamburger
// floating over the guest viewport; with the chrome visible it is a HIDE
// button inside the otherwise empty right end of the toolbar, because narrow
// phone widths fill the whole menu bar with menu titles.
func touchChromeToggleBounds(width int, hidden bool) image.Rectangle {
	if hidden {
		return rectAt(width-12-44, 12, 44, 44)
	}
	return rectAt(
		width-8-76,
		menuBarHeight+(applicationToolbarHeight-32)/2,
		76,
		32,
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
	for _, button := range touchControlButtonsWithOptions(width, height, options) {
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
	active := make(map[string]bool)
	for _, control := range s.touchControls {
		active[control] = true
	}
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	buttons := touchControlButtonsWithOptions(width, height, s.touchLayoutOptions())
	for _, button := range buttons {
		s.drawTouchButton(screen, button, active[button.Control])
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
		s.drawHamburgerToggle(screen, touchChromeToggleBounds(width, true))
		return
	}
	s.drawTouchButton(screen, touchButton{
		Label:  "HIDE",
		Bounds: touchChromeToggleBounds(width, false),
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
	barWidth := bounds.Dx() - 24
	const barHeight = 3
	centerY := bounds.Min.Y + bounds.Dy()/2
	for _, offset := range []int{-8, 0, 8} {
		ebitenutil.DrawRect(
			screen,
			float64(bounds.Min.X+12),
			float64(centerY+offset-barHeight/2),
			float64(barWidth),
			barHeight,
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
		surface := s.design.Components.TouchButton.Image.Idle
		textColor := s.design.Palette.TextMuted
		if active {
			surface = s.design.Components.TouchButton.Image.Pressed
			textColor = s.design.Palette.Text
		}
		surface.Draw(screen, bounds.Dx(), bounds.Dy(), func(options *ebiten.DrawImageOptions) {
			options.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
		})
		drawCenteredText(
			screen,
			label,
			s.design.Type.Strong,
			textColor,
			bounds,
			centeredTextTop(s.design.Type.Strong, bounds, s.design.Type.CenterNudge),
		)
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
