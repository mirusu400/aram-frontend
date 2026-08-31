package frontend

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

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
		contentBottom -= s.touchDeckHeight(bounds.Dx(), bounds.Dy())
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
		s.touchDeckHeight(bounds.Dx(), bounds.Dy())
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
	if s.design != nil && s.design.Components.LCDBezel != nil {
		// Sprite skins frame the guest screen with the pack's LCD bezel; its
		// fixed border is 8px, so the tile is drawn one border larger than
		// the viewport on every side.
		bezel := s.design.Components.LCDBezel
		bezel.Draw(
			screen,
			viewport.Dx()+2*retroSliceBorder,
			viewport.Dy()+2*retroSliceBorder,
			func(opts *ebiten.DrawImageOptions) {
				opts.GeoM.Translate(
					float64(viewport.Min.X-retroSliceBorder),
					float64(viewport.Min.Y-retroSliceBorder),
				)
			},
		)
	} else {
		ebitenutil.DrawRect(
			screen,
			float64(viewport.Min.X-2),
			float64(viewport.Min.Y-2),
			float64(viewport.Dx()+4),
			float64(viewport.Dy()+4),
			palette.BorderStrong,
		)
	}
	ebitenutil.DrawRect(
		screen,
		float64(viewport.Min.X),
		float64(viewport.Min.Y),
		float64(viewport.Dx()),
		float64(viewport.Dy()),
		palette.GuestSurface,
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
	s.drawGuestFrame(screen, destination, sourceBounds)
}

// drawGuestFrame keeps sampling and the optional panel simulation separate.
// The guest is first rotated and scaled exactly as it is without an effect;
// the feature-phone pass then models the physical LCD over those final pixels.
// This also keeps screenshots on the guest-native frame, before presentation.
func (s *Shell) drawGuestFrame(
	screen *ebiten.Image,
	destination image.Rectangle,
	sourceBounds image.Rectangle,
) {
	drawDestination := destination
	target := screen
	if s.settings.DisplayEffect == displayEffectFeaturePhone {
		target = s.ensureDisplayEffectImage(destination.Dx(), destination.Dy())
		target.Clear()
		drawDestination = image.Rect(0, 0, destination.Dx(), destination.Dy())
	}

	target.DrawImage(s.frameImage, s.guestFrameDrawOptions(sourceBounds, drawDestination))
	if target == screen {
		return
	}

	shader, err := loadFeaturePhoneDisplayShader()
	if err != nil {
		// A fixed shader compilation failure is covered by a focused test. The
		// runtime fallback still leaves the guest visible on unusual drivers.
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
		screen.DrawImage(target, options)
		return
	}
	pitchX, pitchY := displayPixelPitch(
		destination,
		sourceBounds.Dx(),
		sourceBounds.Dy(),
		s.settings.Rotation,
	)
	options := &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]any{
			"PixelPitch": []float32{pitchX, pitchY},
		},
	}
	options.Images[0] = target
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	screen.DrawRectShader(destination.Dx(), destination.Dy(), shader, options)
}

func (s *Shell) guestFrameDrawOptions(
	sourceBounds image.Rectangle,
	destination image.Rectangle,
) *ebiten.DrawImageOptions {
	sourceWidth, sourceHeight := sourceBounds.Dx(), sourceBounds.Dy()
	rotatedWidth, rotatedHeight := sourceWidth, sourceHeight
	if s.settings.Rotation == 90 || s.settings.Rotation == 270 {
		rotatedWidth, rotatedHeight = sourceHeight, sourceWidth
	}
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
	return options
}

func (s *Shell) ensureDisplayEffectImage(width, height int) *ebiten.Image {
	if s.displayEffectImage == nil ||
		s.displayEffectImage.Bounds().Dx() != width ||
		s.displayEffectImage.Bounds().Dy() != height {
		s.displayEffectImage = ebiten.NewImage(width, height)
	}
	return s.displayEffectImage
}

// displayPixelPitch reports how many final display pixels represent one guest
// pixel. The LCD grid follows the rotated guest axes rather than the window,
// so portrait and landscape titles retain square cells at integer scale.
func displayPixelPitch(
	destination image.Rectangle,
	sourceWidth, sourceHeight, rotation int,
) (float32, float32) {
	if rotation == 90 || rotation == 270 {
		sourceWidth, sourceHeight = sourceHeight, sourceWidth
	}
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return 1, 1
	}
	return float32(destination.Dx()) / float32(sourceWidth),
		float32(destination.Dy()) / float32(sourceHeight)
}

var (
	featurePhoneDisplayShaderOnce sync.Once
	featurePhoneDisplayShader     *ebiten.Shader
	featurePhoneDisplayShaderErr  error
)

func loadFeaturePhoneDisplayShader() (*ebiten.Shader, error) {
	featurePhoneDisplayShaderOnce.Do(func() {
		featurePhoneDisplayShader, featurePhoneDisplayShaderErr =
			ebiten.NewShader([]byte(featurePhoneDisplayShaderSource))
	})
	return featurePhoneDisplayShader, featurePhoneDisplayShaderErr
}

// The panel is intentionally TFT/LCD, not CRT: RGB565 colour depth, a subtle
// RGB subpixel mask, source-pixel cell seams, slow-response bleed, lifted
// blacks, and uneven edge illumination. The strengths stay low enough that
// Hangul and small in-game text remain readable.
const featurePhoneDisplayShaderSource = `//kage:unit pixels

package main

var PixelPitch vec2

func LCDSample(pos vec2) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	halfPixel := vec2(0.5)
	return imageSrc0At(clamp(pos, origin+halfPixel, origin+size-halfPixel))
}

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	current := LCDSample(src0Pos)
	responseTrail := LCDSample(src0Pos - vec2(0.7, 0.2))
	rgb := current.rgb*0.955 + responseTrail.rgb*0.045

	// Mid-2000s colour handsets commonly exposed a 16-bit RGB565 panel.
	rgb.r = floor(rgb.r*31.0+0.5) / 31.0
	rgb.g = floor(rgb.g*63.0+0.5) / 63.0
	rgb.b = floor(rgb.b*31.0+0.5) / 31.0

	// A small black lift and green bias read as a lit LCD instead of OLED.
	rgb = rgb*0.94 + vec3(0.012, 0.016, 0.009)

	origin := imageSrc0Origin()
	size := imageSrc0Size()
	local := src0Pos - origin
	cellShade := 1.0
	if PixelPitch.x >= 2.75 && mod(local.x, PixelPitch.x) > PixelPitch.x-0.6 {
		cellShade *= 0.93
	}
	if PixelPitch.y >= 2.75 && mod(local.y, PixelPitch.y) > PixelPitch.y-0.6 {
		cellShade *= 0.92
	}

	// The mask is deliberately faint: visible up close without rainbowing text.
	mask := vec3(0.985)
	stripe := mod(floor(local.x), 3.0)
	if stripe < 1.0 {
		mask.r = 1.015
	} else if stripe < 2.0 {
		mask.g = 1.015
	} else {
		mask.b = 1.015
	}
	rgb *= mask * cellShade

	uv := local / size
	centered := uv*2.0 - 1.0
	vignette := 1.0 - 0.045*dot(centered, centered)
	glare := max(0.0, 1.0-uv.x*0.65-uv.y*1.45) * 0.018
	rgb = rgb*vignette + vec3(glare, glare, glare*0.82)
	return vec4(clamp(rgb, vec3(0.0), vec3(1.0)), current.a)
}
`

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
	drawCenteredText(screen, title, s.design.Type.Display, palette.GuestInk, viewport, y)
	y += 36
	detailInk := mixNRGBA(palette.GuestInk, palette.GuestSurface, 0.35)
	for _, line := range details {
		drawCenteredText(screen, line, s.design.Type.Body, detailInk, viewport, y)
		y += 20
	}
}

// centeredTextTop returns the y drawCenteredText needs so a single line sits
// vertically centered in bounds. It measures the face's own line box instead
// of assuming a text height, so swapping the type ramp — the pixel faces the
// sprite skins use are far taller than the modern ones — cannot silently push
// every label off center.
func centeredTextTop(
	face *text.Face,
	bounds image.Rectangle,
	nudge int,
) int {
	_, lineHeight := text.Measure(" ", *face, 0)
	top := float64(bounds.Min.Y) + (float64(bounds.Dy())-lineHeight)/2
	return int(top) - nudge
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
