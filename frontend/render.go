package frontend

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func (s *Shell) drawWorkspace(screen *ebiten.Image) {
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	bounds := screen.Bounds()
	contentTop := bounds.Min.Y + menuHeight + applicationToolbarHeight + 12
	contentBottom := bounds.Max.Y - statusHeight - 12
	if platformUsesTouchLayout() {
		contentBottom -= touchDeckHeight(bounds.Dx(), bounds.Dy())
	}
	contentRight := bounds.Max.X - 12
	if s.virtualKeypadVisible() {
		contentRight -= virtualKeypadReservedWidthFor(bounds.Dx())
	}
	viewportPanel := image.Rect(bounds.Min.X+12, contentTop, contentRight, contentBottom)
	if viewportPanel.Dx() < 32 || viewportPanel.Dy() < 32 {
		return
	}
	viewport := image.Rect(
		viewportPanel.Min.X+6,
		viewportPanel.Min.Y+6,
		viewportPanel.Max.X-6,
		viewportPanel.Max.Y-6,
	)
	ebitenutil.DrawRect(
		screen,
		float64(viewportPanel.Min.X),
		float64(viewportPanel.Min.Y),
		float64(viewportPanel.Dx()),
		float64(viewportPanel.Dy()),
		palette.Border,
	)
	ebitenutil.DrawRect(
		screen,
		float64(viewportPanel.Min.X+1),
		float64(viewportPanel.Min.Y+1),
		float64(viewportPanel.Dx()-2),
		float64(viewportPanel.Dy()-2),
		palette.Surface,
	)
	s.drawGuestViewport(screen, viewport)
}

// drawImmersiveWorkspace fills everything above the touch control deck with
// the guest viewport. The chrome is hidden, so there are no bars, panels, or
// framing margins around the guest screen.
func (s *Shell) drawImmersiveWorkspace(screen *ebiten.Image) {
	bounds := screen.Bounds()
	deckTop := bounds.Max.Y - statusBarHeight -
		touchDeckHeight(bounds.Dx(), bounds.Dy())
	viewport := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, deckTop)
	if viewport.Dx() < 32 || viewport.Dy() < 32 {
		return
	}
	s.drawFilledGuestViewport(screen, viewport)
}

// drawFilledGuestViewport draws the guest screen at the largest
// aspect-preserving size. Chrome-less play surfaces exist to give the guest
// the whole screen, so the integer-scaling preference must not floor a
// fractional fit down to a fraction of that space; it still applies to the
// windowed workspace.
func (s *Shell) drawFilledGuestViewport(
	screen *ebiten.Image,
	viewport image.Rectangle,
) {
	s.fillGuestViewport = true
	defer func() { s.fillGuestViewport = false }()
	s.drawGuestViewport(screen, viewport)
}

func (s *Shell) drawGuestViewport(screen *ebiten.Image, viewport image.Rectangle) {
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	ebitenutil.DrawRect(
		screen,
		float64(viewport.Min.X-2),
		float64(viewport.Min.Y-2),
		float64(viewport.Dx()+4),
		float64(viewport.Dy()+4),
		palette.BorderStrong,
	)
	ebitenutil.DrawRect(
		screen,
		float64(viewport.Min.X),
		float64(viewport.Min.Y),
		float64(viewport.Dx()),
		float64(viewport.Dy()),
		palette.Canvas,
	)

	if s.frameImage == nil || s.frame.Image == nil {
		s.drawEmptyViewport(screen, viewport)
		return
	}

	sourceBounds := s.frame.Image.Bounds()
	sourceWidth, sourceHeight := sourceBounds.Dx(), sourceBounds.Dy()
	rotatedWidth, rotatedHeight := sourceWidth, sourceHeight
	if s.settings.Rotation == 90 || s.settings.Rotation == 270 {
		rotatedWidth, rotatedHeight = sourceHeight, sourceWidth
	}
	destination := s.frameDestination(viewport, rotatedWidth, rotatedHeight)
	scaleX := float64(destination.Dx()) / float64(rotatedWidth)
	scaleY := float64(destination.Dy()) / float64(rotatedHeight)

	options := &ebiten.DrawImageOptions{}
	options.GeoM.Translate(float64(-sourceBounds.Min.X), float64(-sourceBounds.Min.Y))
	switch s.settings.Rotation {
	case 90:
		options.GeoM.Rotate(math.Pi / 2)
		options.GeoM.Translate(float64(sourceHeight), 0)
	case 180:
		options.GeoM.Rotate(math.Pi)
		options.GeoM.Translate(float64(sourceWidth), float64(sourceHeight))
	case 270:
		options.GeoM.Rotate(3 * math.Pi / 2)
		options.GeoM.Translate(0, float64(sourceWidth))
	}
	options.GeoM.Scale(scaleX, scaleY)
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	if s.settings.Filter == "linear" {
		options.Filter = ebiten.FilterLinear
	} else {
		options.Filter = ebiten.FilterNearest
	}
	screen.DrawImage(s.frameImage, options)
}

func (s *Shell) frameDestination(viewport image.Rectangle, width, height int) image.Rectangle {
	if width <= 0 || height <= 0 {
		return viewport
	}
	scaleX := float64(viewport.Dx()) / float64(width)
	scaleY := float64(viewport.Dy()) / float64(height)

	if s.settings.ScreenLayout == "stretch" && !s.settings.PreserveAspect {
		return viewport
	}

	scale := math.Min(scaleX, scaleY)
	if s.settings.IntegerScaling && !s.fillGuestViewport && scale >= 1 {
		scale = math.Max(1, math.Floor(scale))
	}
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))
	if !s.settings.PreserveAspect && s.settings.ScreenLayout == "stretch" {
		targetWidth = viewport.Dx()
		targetHeight = viewport.Dy()
	}
	x := viewport.Min.X + (viewport.Dx()-targetWidth)/2
	y := viewport.Min.Y + (viewport.Dy()-targetHeight)/2
	return image.Rect(x, y, x+targetWidth, y+targetHeight)
}

func (s *Shell) drawEmptyViewport(screen *ebiten.Image, viewport image.Rectangle) {
	palette := defaultARAMPalette()
	if s.design != nil {
		palette = s.design.Palette
	}
	title := s.tr("No guest frame")
	details := []string{s.tr("Open a title from File, or drag a file here.")}
	if s.loading {
		title = s.tr("Preparing input")
		details = []string{
			strings.ToUpper(s.tr(stateValueLabel(string(s.state)))),
		}
	} else if s.problem != nil {
		title = s.tr("Unable to start this input")
		details = []string{
			s.trf(
				"State: %s",
				s.tr(stateValueLabel(string(s.problem.State))),
			),
			s.trf("Input: %s", shorten(s.problem.Input, 40)),
			s.trf(
				"Format: %s",
				emptyFallback(s.problem.Format, s.tr("unknown")),
			),
			s.trf(
				"Profile: %s",
				emptyFallback(s.problem.Profile, s.tr("unselected")),
			),
			s.trf(
				"Backend: %s",
				emptyFallback(s.problem.Backend, s.tr("unknown")),
			),
			shorten(s.problem.Reason, 64),
		}
		ebitenutil.DrawRect(
			screen,
			float64(viewport.Min.X),
			float64(viewport.Min.Y),
			float64(viewport.Dx()),
			4,
			palette.Fault,
		)
	}
	if s.design == nil {
		lines := append([]string{title, ""}, details...)
		ebitenutil.DebugPrintAt(
			screen,
			strings.Join(lines, "\n"),
			viewport.Min.X+28,
			viewport.Min.Y+viewport.Dy()/2-len(lines)*8,
		)
		return
	}

	blockHeight := 28 + len(details)*20
	y := viewport.Min.Y + (viewport.Dy()-blockHeight)/2
	drawCenteredText(screen, title, s.design.Type.Display, palette.Text, viewport, y)
	y += 36
	for _, line := range details {
		drawCenteredText(screen, line, s.design.Type.Body, palette.TextMuted, viewport, y)
		y += 20
	}
}

func drawCenteredText(
	screen *ebiten.Image,
	label string,
	face *text.Face,
	textColor color.Color,
	bounds image.Rectangle,
	y int,
) {
	width, _ := text.Measure(label, *face, 0)
	options := &text.DrawOptions{}
	options.GeoM.Translate(float64(bounds.Min.X)+(float64(bounds.Dx())-width)/2, float64(y))
	options.ColorScale.ScaleWithColor(textColor)
	text.Draw(screen, label, *face, options)
}
