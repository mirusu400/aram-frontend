package frontend

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"

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

// drawGuestFrame keeps the guest-native texture immutable and builds each
// presentation preset from it. Original preserves the user's texture-filter
// choice. Crisp Fit and handset panels use sharp bilinear sampling so integer
// pixels stay flat while fractional window fits blend only at cell boundaries.
// Smooth Pixel first builds a cached edge-aware 2x image in guest coordinates.
func (s *Shell) drawGuestFrame(
	screen *ebiten.Image,
	destination image.Rectangle,
	sourceBounds image.Rectangle,
) {
	effect := s.settings.DisplayEffect
	if effect == displayEffectOff || !isDisplayEffectChoice(effect) {
		s.resetDisplayHistory()
		screen.DrawImage(s.frameImage, s.guestFrameDrawOptions(sourceBounds, destination))
		return
	}

	target := s.ensureDisplayEffectImage(destination.Dx(), destination.Dy())
	target.Clear()
	if effect == displayEffectSmoothPixel {
		if err := s.drawSmoothPixel(target); err != nil {
			if fallbackErr := s.drawSharpBilinear(target); fallbackErr != nil {
				local := image.Rect(0, 0, destination.Dx(), destination.Dy())
				target.DrawImage(s.frameImage, s.guestFrameDrawOptions(sourceBounds, local))
			}
		}
		s.resetDisplayHistory()
		drawDisplaySurface(screen, target, destination)
		return
	}
	if err := s.drawSharpBilinear(target); err != nil {
		// Fixed shader compilation is tested, but the frame must remain visible
		// if a platform rejects it at runtime.
		local := image.Rect(0, 0, destination.Dx(), destination.Dy())
		target.DrawImage(s.frameImage, s.guestFrameDrawOptions(sourceBounds, local))
	}

	if effect == displayEffectFeaturePhoneTFT ||
		effect == displayEffectFeaturePhoneSTN {
		target = s.updateDisplayPersistence(target)
		s.drawFeaturePhonePanel(screen, target, destination, sourceBounds, effect)
		return
	}
	if effect == displayEffectCRTTV {
		s.resetDisplayHistory()
		s.drawCRTTV(screen, target, destination, sourceBounds)
		return
	}

	s.resetDisplayHistory()
	drawDisplaySurface(screen, target, destination)
}

var displayQuadIndices = []uint16{0, 1, 2, 1, 3, 2}

func (s *Shell) drawSharpBilinear(
	target *ebiten.Image,
) error {
	return drawSharpBilinearImage(
		target,
		s.frameImage,
		s.frameImage.Bounds(),
		s.settings.Rotation,
	)
}

func drawSharpBilinearImage(
	target *ebiten.Image,
	source *ebiten.Image,
	sourceBounds image.Rectangle,
	rotation int,
) error {
	shader, err := loadSharpBilinearShader()
	if err != nil {
		return err
	}
	scaleX, scaleY := sharpBilinearScale(
		target.Bounds(),
		sourceBounds.Dx(),
		sourceBounds.Dy(),
		rotation,
	)
	options := &ebiten.DrawTrianglesShaderOptions{
		Uniforms: map[string]any{
			"Scale": []float32{scaleX, scaleY},
		},
	}
	options.Images[0] = source
	target.DrawTrianglesShader(
		displayQuadVertices(target.Bounds(), sourceBounds, rotation),
		displayQuadIndices,
		shader,
		options,
	)
	return nil
}

type displayScaleKey struct {
	Width      int
	Height     int
	Generation uint64
}

func (s *Shell) drawSmoothPixel(target *ebiten.Image) error {
	scaled, err := s.smoothPixel2xImage()
	if err != nil {
		return err
	}
	return drawSharpBilinearImage(
		target,
		scaled,
		scaled.Bounds(),
		s.settings.Rotation,
	)
}

// smoothPixel2xImage caches the native 2x result until the backend publishes
// another guest sequence. Keeping the edge classifier in guest coordinates
// makes its decisions independent of window size and screen rotation.
func (s *Shell) smoothPixel2xImage() (*ebiten.Image, error) {
	width, height := s.frameImage.Bounds().Dx(), s.frameImage.Bounds().Dy()
	key := displayScaleKey{
		Width:      width,
		Height:     height,
		Generation: s.frame.Generation,
	}
	if s.displayScaleValid && s.displayScaleKey == key &&
		s.displayScaleSequence == s.frame.Sequence {
		return s.displayScaleImage, nil
	}

	shader, err := loadSmoothPixelShader()
	if err != nil {
		return nil, err
	}
	scaled := ensureDisplaySurface(&s.displayScaleImage, width*2, height*2)
	scaled.Clear()
	options := &ebiten.DrawTrianglesShaderOptions{
		Uniforms: map[string]any{
			"SourceSize": []float32{float32(width), float32(height)},
		},
	}
	options.Images[0] = s.frameImage
	scaled.DrawTrianglesShader(
		displayQuadVertices(scaled.Bounds(), s.frameImage.Bounds(), 0),
		displayQuadIndices,
		shader,
		options,
	)
	s.displayScaleKey = key
	s.displayScaleSequence = s.frame.Sequence
	s.displayScaleValid = true
	return scaled, nil
}

func displayQuadVertices(
	destination image.Rectangle,
	source image.Rectangle,
	rotation int,
) []ebiten.Vertex {
	left, top := float32(source.Min.X), float32(source.Min.Y)
	right, bottom := float32(source.Max.X), float32(source.Max.Y)
	sourceCorners := [4][2]float32{
		{left, top}, {right, top}, {left, bottom}, {right, bottom},
	}
	switch rotation {
	case 90:
		sourceCorners = [4][2]float32{
			{left, bottom}, {left, top}, {right, bottom}, {right, top},
		}
	case 180:
		sourceCorners = [4][2]float32{
			{right, bottom}, {left, bottom}, {right, top}, {left, top},
		}
	case 270:
		sourceCorners = [4][2]float32{
			{right, top}, {right, bottom}, {left, top}, {left, bottom},
		}
	}
	destinationCorners := [4][2]float32{
		{float32(destination.Min.X), float32(destination.Min.Y)},
		{float32(destination.Max.X), float32(destination.Min.Y)},
		{float32(destination.Min.X), float32(destination.Max.Y)},
		{float32(destination.Max.X), float32(destination.Max.Y)},
	}
	vertices := make([]ebiten.Vertex, 4)
	for index := range vertices {
		vertices[index] = ebiten.Vertex{
			DstX:   destinationCorners[index][0],
			DstY:   destinationCorners[index][1],
			SrcX:   sourceCorners[index][0],
			SrcY:   sourceCorners[index][1],
			ColorR: 1,
			ColorG: 1,
			ColorB: 1,
			ColorA: 1,
		}
	}
	return vertices
}

// sharpBilinearScale is expressed in source axes. A quarter-turn swaps which
// destination edge scales source X and Y.
func sharpBilinearScale(
	destination image.Rectangle,
	sourceWidth, sourceHeight, rotation int,
) (float32, float32) {
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return 1, 1
	}
	if rotation == 90 || rotation == 270 {
		return float32(destination.Dy()) / float32(sourceWidth),
			float32(destination.Dx()) / float32(sourceHeight)
	}
	return float32(destination.Dx()) / float32(sourceWidth),
		float32(destination.Dy()) / float32(sourceHeight)
}

type displayHistoryKey struct {
	Width      int
	Height     int
	Rotation   int
	Effect     string
	Filter     string
	Generation uint64
}

func (s *Shell) updateDisplayPersistence(
	current *ebiten.Image,
) *ebiten.Image {
	width, height := current.Bounds().Dx(), current.Bounds().Dy()
	history := ensureDisplaySurface(&s.displayHistoryImage, width, height)
	response := ensureDisplaySurface(&s.displayResponseImage, width, height)
	key := displayHistoryKey{
		Width:      width,
		Height:     height,
		Rotation:   s.settings.Rotation,
		Effect:     s.settings.DisplayEffect,
		Filter:     s.settings.Filter,
		Generation: s.frame.Generation,
	}
	now := s.now()
	if !s.displayHistoryValid || s.displayHistoryKey != key ||
		s.frame.Sequence < s.displayHistorySequence {
		history.Clear()
		history.DrawImage(current, nil)
		s.displayHistoryKey = key
		s.displayHistorySequence = s.frame.Sequence
		s.displayHistoryAt = now
		s.displayHistoryValid = true
		return history
	}
	elapsed := now.Sub(s.displayHistoryAt)
	if s.frame.Sequence == s.displayHistorySequence && elapsed <= 0 {
		return history
	}

	shader, err := loadTemporalBlendShader()
	response.Clear()
	if err != nil {
		response.DrawImage(current, nil)
	} else {
		weight := displayPersistenceWeight(
			s.settings.DisplayEffect,
			elapsed,
		)
		options := &ebiten.DrawRectShaderOptions{
			Uniforms: map[string]any{"HistoryWeight": weight},
		}
		options.Images[0] = current
		options.Images[1] = history
		response.DrawRectShader(width, height, shader, options)
	}
	s.displayHistoryImage, s.displayResponseImage = response, history
	s.displayHistorySequence = s.frame.Sequence
	s.displayHistoryAt = now
	return s.displayHistoryImage
}

func displayPersistenceWeight(effect string, elapsed time.Duration) float32 {
	halfLife := time.Duration(0)
	maximum := 0.95
	switch effect {
	case displayEffectFeaturePhoneTFT:
		halfLife = 22 * time.Millisecond
	case displayEffectFeaturePhoneSTN:
		halfLife = 75 * time.Millisecond
		maximum = 0.975
	default:
		return 0
	}
	if elapsed <= 0 {
		elapsed = time.Second / 60
	}
	weight := math.Pow(0.5, float64(elapsed)/float64(halfLife))
	return float32(min(maximum, max(0.0, weight)))
}

func (s *Shell) drawFeaturePhonePanel(
	screen *ebiten.Image,
	source *ebiten.Image,
	destination image.Rectangle,
	sourceBounds image.Rectangle,
	effect string,
) {
	shader, err := loadFeaturePhoneDisplayShader()
	if effect == displayEffectFeaturePhoneSTN {
		shader, err = loadFeaturePhoneSTNShader()
	}
	if err != nil {
		drawDisplaySurface(screen, source, destination)
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
	options.Images[0] = source
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	screen.DrawRectShader(destination.Dx(), destination.Dy(), shader, options)
}

func (s *Shell) drawCRTTV(
	screen *ebiten.Image,
	source *ebiten.Image,
	destination image.Rectangle,
	sourceBounds image.Rectangle,
) {
	shader, err := loadCRTTVShader()
	if err != nil {
		drawDisplaySurface(screen, source, destination)
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
	options.Images[0] = source
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	screen.DrawRectShader(destination.Dx(), destination.Dy(), shader, options)
}

func drawDisplaySurface(
	screen *ebiten.Image,
	source *ebiten.Image,
	destination image.Rectangle,
) {
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Translate(float64(destination.Min.X), float64(destination.Min.Y))
	screen.DrawImage(source, options)
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
	return ensureDisplaySurface(&s.displayEffectImage, width, height)
}

func ensureDisplaySurface(surface **ebiten.Image, width, height int) *ebiten.Image {
	if *surface == nil ||
		(*surface).Bounds().Dx() != width ||
		(*surface).Bounds().Dy() != height {
		*surface = ebiten.NewImage(width, height)
	}
	return *surface
}

func (s *Shell) resetDisplayHistory() {
	s.displayHistoryKey = displayHistoryKey{}
	s.displayHistorySequence = 0
	s.displayHistoryAt = time.Time{}
	s.displayHistoryValid = false
}

func (s *Shell) releaseDisplaySurfaces() {
	s.displayEffectImage = nil
	s.displayScaleImage = nil
	s.displayScaleKey = displayScaleKey{}
	s.displayScaleSequence = 0
	s.displayScaleValid = false
	s.displayHistoryImage = nil
	s.displayResponseImage = nil
	s.resetDisplayHistory()
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
	sharpBilinearShaderOnce       sync.Once
	sharpBilinearShader           *ebiten.Shader
	sharpBilinearShaderErr        error
	temporalBlendShaderOnce       sync.Once
	temporalBlendShader           *ebiten.Shader
	temporalBlendShaderErr        error
	smoothPixelShaderOnce         sync.Once
	smoothPixelShader             *ebiten.Shader
	smoothPixelShaderErr          error
	featurePhoneDisplayShaderOnce sync.Once
	featurePhoneDisplayShader     *ebiten.Shader
	featurePhoneDisplayShaderErr  error
	featurePhoneSTNShaderOnce     sync.Once
	featurePhoneSTNShader         *ebiten.Shader
	featurePhoneSTNShaderErr      error
	crtTVShaderOnce               sync.Once
	crtTVShader                   *ebiten.Shader
	crtTVShaderErr                error
)

func loadSharpBilinearShader() (*ebiten.Shader, error) {
	sharpBilinearShaderOnce.Do(func() {
		sharpBilinearShader, sharpBilinearShaderErr =
			ebiten.NewShader([]byte(sharpBilinearShaderSource))
	})
	return sharpBilinearShader, sharpBilinearShaderErr
}

func loadTemporalBlendShader() (*ebiten.Shader, error) {
	temporalBlendShaderOnce.Do(func() {
		temporalBlendShader, temporalBlendShaderErr =
			ebiten.NewShader([]byte(temporalBlendShaderSource))
	})
	return temporalBlendShader, temporalBlendShaderErr
}

func loadSmoothPixelShader() (*ebiten.Shader, error) {
	smoothPixelShaderOnce.Do(func() {
		smoothPixelShader, smoothPixelShaderErr =
			ebiten.NewShader([]byte(smoothPixelShaderSource))
	})
	return smoothPixelShader, smoothPixelShaderErr
}

func loadFeaturePhoneDisplayShader() (*ebiten.Shader, error) {
	featurePhoneDisplayShaderOnce.Do(func() {
		featurePhoneDisplayShader, featurePhoneDisplayShaderErr =
			ebiten.NewShader([]byte(featurePhoneDisplayShaderSource))
	})
	return featurePhoneDisplayShader, featurePhoneDisplayShaderErr
}

func loadFeaturePhoneSTNShader() (*ebiten.Shader, error) {
	featurePhoneSTNShaderOnce.Do(func() {
		featurePhoneSTNShader, featurePhoneSTNShaderErr =
			ebiten.NewShader([]byte(featurePhoneSTNShaderSource))
	})
	return featurePhoneSTNShader, featurePhoneSTNShaderErr
}

func loadCRTTVShader() (*ebiten.Shader, error) {
	crtTVShaderOnce.Do(func() {
		crtTVShader, crtTVShaderErr = ebiten.NewShader([]byte(crtTVShaderSource))
	})
	return crtTVShader, crtTVShaderErr
}

// Sharp bilinear holds the center of each source pixel flat and confines
// interpolation to one output pixel around a source-cell boundary. Fractional
// fits therefore stay crisp without the uneven pixel widths of nearest-neighbor
// sampling. Downscaling naturally falls back to ordinary bilinear sampling.
const sharpBilinearShaderSource = `//kage:unit pixels

package main

var Scale vec2

func SafeSource(pos vec2) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	halfPixel := vec2(0.5)
	return imageSrc0At(clamp(pos, origin+halfPixel, origin+size-halfPixel))
}

func BilinearSource(pos vec2) vec4 {
	origin := imageSrc0Origin()
	texel := pos - origin - vec2(0.5)
	base := floor(texel)
	f := fract(texel)
	p := origin + base + vec2(0.5)
	top := mix(SafeSource(p), SafeSource(p+vec2(1.0, 0.0)), f.x)
	bottom := mix(SafeSource(p+vec2(0.0, 1.0)), SafeSource(p+vec2(1.0, 1.0)), f.x)
	return mix(top, bottom, f.y)
}

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	origin := imageSrc0Origin()
	local := src0Pos - origin
	scale := max(Scale, vec2(1.0))
	region := vec2(0.5) - vec2(0.5)/scale
	distanceFromCenter := fract(local) - vec2(0.5)
	transition := (distanceFromCenter-clamp(distanceFromCenter, -region, region))*scale + vec2(0.5)
	return BilinearSource(origin + floor(local) + transition)
}
`

const temporalBlendShaderSource = `//kage:unit pixels

package main

var HistoryWeight float

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	current := imageSrc0At(src0Pos)
	history := imageSrc1At(src0Pos)
	return mix(current, history, HistoryWeight)
}
`

// Smooth Pixel is a clean-room, xBRZ-style 2x scaler. It uses a perceptual
// colour distance and evaluates each source-pixel corner independently: only
// a continuous diagonal crossing that corner is blended. Axis-aligned edges,
// isolated pixels, and one-pixel strokes remain exact, which matters for the
// tiny bitmap Hangul commonly drawn by WIPI titles.
const smoothPixelShaderSource = `//kage:unit pixels

package main

var SourceSize vec2

func SmoothSource(cell vec2) vec4 {
	origin := imageSrc0Origin()
	bounded := clamp(cell, vec2(0.0), SourceSize-vec2(1.0))
	return imageSrc0At(origin+bounded+vec2(0.5))
}

func PerceptualDistance(a vec4, b vec4) float {
	delta := a.rgb - b.rgb
	luma := dot(delta, vec3(0.2627, 0.6780, 0.0593))
	blueChroma := (delta.b-luma)*0.5
	redChroma := (delta.r-luma)*0.5
	alpha := a.a - b.a
	return luma*luma*1.7 + blueChroma*blueChroma*0.35 +
		redChroma*redChroma*0.55 + alpha*alpha
}

func SmoothSimilar(a vec4, b vec4) bool {
	return PerceptualDistance(a, b) < 0.0022
}

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	outputPixel := floor(dstPos.xy)
	cell := floor(outputPixel/2.0)
	quadrant := mod(outputPixel, vec2(2.0))

	center := SmoothSource(cell)
	north := SmoothSource(cell+vec2(0.0, -1.0))
	south := SmoothSource(cell+vec2(0.0, 1.0))
	west := SmoothSource(cell+vec2(-1.0, 0.0))
	east := SmoothSource(cell+vec2(1.0, 0.0))
	northWest := SmoothSource(cell+vec2(-1.0, -1.0))
	northEast := SmoothSource(cell+vec2(1.0, -1.0))
	southWest := SmoothSource(cell+vec2(-1.0, 1.0))
	southEast := SmoothSource(cell+vec2(1.0, 1.0))

	// A lone contrasting source pixel is usually punctuation or part of a
	// small glyph. Rounding all four corners would make it disappear.
	isolated := SmoothSimilar(north, south) && SmoothSimilar(north, west) &&
		SmoothSimilar(north, east) && !SmoothSimilar(center, north)
	if isolated {
		return center
	}

	horizontal := west
	vertical := north
	oppositeHorizontal := east
	oppositeVertical := south
	diagonal := northWest
	if quadrant.x >= 1.0 {
		horizontal = east
		oppositeHorizontal = west
		diagonal = northEast
	}
	if quadrant.y >= 1.0 {
		vertical = south
		oppositeVertical = north
		if quadrant.x < 1.0 {
			diagonal = southWest
		} else {
			diagonal = southEast
		}
	}

	agreement := PerceptualDistance(horizontal, vertical)
	centerHorizontal := PerceptualDistance(center, horizontal)
	centerVertical := PerceptualDistance(center, vertical)
	contrast := min(centerHorizontal, centerVertical)
	continuity := agreement + PerceptualDistance(diagonal, horizontal)*0.25 +
		PerceptualDistance(diagonal, vertical)*0.25
	reverse := centerHorizontal + centerVertical +
		PerceptualDistance(oppositeHorizontal, oppositeVertical)*0.25
	centerSupported := SmoothSimilar(center, oppositeHorizontal) ||
		SmoothSimilar(center, oppositeVertical)

	if contrast > 0.0012 && continuity*2.0 < reverse && centerSupported {
		candidate := (horizontal + vertical + diagonal) / 3.0
		strength := 0.52
		if agreement < 0.00045 && SmoothSimilar(diagonal, horizontal) &&
			SmoothSimilar(diagonal, vertical) {
			strength = 0.68
		}
		return mix(center, candidate, strength)
	}
	return center
}
`

// The panel is intentionally TFT/LCD, not CRT: RGB565 colour depth, a subtle
// RGB subpixel mask, source-pixel cell seams, temporal response, lifted
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
	rgb := current.rgb

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

// Passive-matrix STN panels are substantially slower and less neutral than
// TFT. Temporal lag is applied in a separate history pass; this shader adds
// the panel's reduced colour separation, row crosstalk, coarse colour steps,
// stronger cell seams, green-yellow polarizer and uneven backlight.
const featurePhoneSTNShaderSource = `//kage:unit pixels

package main

var PixelPitch vec2

func STNSample(pos vec2) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	halfPixel := vec2(0.5)
	return imageSrc0At(clamp(pos, origin+halfPixel, origin+size-halfPixel))
}

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	current := STNSample(src0Pos)
	horizontalTrail := STNSample(src0Pos-vec2(0.85, 0.2))
	rowTrail := STNSample(src0Pos-vec2(0.0, 0.65))
	rgb := current.rgb*0.86 + horizontalTrail.rgb*0.09 + rowTrail.rgb*0.05

	luma := dot(rgb, vec3(0.299, 0.587, 0.114))
	rgb = vec3(luma) + (rgb-vec3(luma))*0.48
	rgb = (rgb-vec3(0.5))*0.80 + vec3(0.5)
	rgb.r = floor(rgb.r*15.0+0.5) / 15.0
	rgb.g = floor(rgb.g*15.0+0.5) / 15.0
	rgb.b = floor(rgb.b*15.0+0.5) / 15.0
	rgb = rgb*vec3(0.94, 1.015, 0.77) + vec3(0.018, 0.024, 0.008)

	origin := imageSrc0Origin()
	size := imageSrc0Size()
	local := src0Pos - origin
	cellShade := 1.0
	if PixelPitch.x >= 2.4 && mod(local.x, PixelPitch.x) > PixelPitch.x-0.75 {
		cellShade *= 0.88
	}
	if PixelPitch.y >= 2.4 && mod(local.y, PixelPitch.y) > PixelPitch.y-0.8 {
		cellShade *= 0.86
	}
	rowBand := mod(floor(local.y/4.0), 2.0)
	cellShade *= 0.985 + rowBand*0.015

	uv := local / size
	centered := uv*2.0 - 1.0
	backlight := 0.97 - 0.09*dot(centered, centered)
	viewingAngle := 1.0 - abs(uv.x-0.42)*0.055
	glare := max(0.0, 1.0-uv.x*0.7-uv.y*1.3) * 0.026
	rgb = rgb*cellShade*backlight*viewingAngle + vec3(glare, glare, glare*0.58)
	return vec4(clamp(rgb, vec3(0.0), vec3(1.0)), current.a)
}
`

// CRT TV runs after the guest has been fitted to the final viewport. Composite
// video carried much less chroma bandwidth than luma, so the shader preserves
// centre-pixel luma while low-pass filtering I/Q horizontally with an
// asymmetric five-tap kernel. Scanline spacing still follows guest rows; the
// physical 3x2 RGB triad mask follows final display pixels.
const crtTVShaderSource = `//kage:unit pixels

package main

var PixelPitch vec2

func CRTSample(pos vec2) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	halfPixel := vec2(0.5)
	return imageSrc0At(clamp(pos, origin+halfPixel, origin+size-halfPixel))
}

func RGBToYIQ(rgb vec3) vec3 {
	return vec3(
		dot(rgb, vec3(0.299, 0.587, 0.114)),
		dot(rgb, vec3(0.596, -0.274, -0.322)),
		dot(rgb, vec3(0.211, -0.523, 0.312)),
	)
}

func YIQToRGB(yiq vec3) vec3 {
	return vec3(
		yiq.x + yiq.y*0.956 + yiq.z*0.621,
		yiq.x - yiq.y*0.272 - yiq.z*0.647,
		yiq.x - yiq.y*1.106 + yiq.z*1.703,
	)
}

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	leftFar := RGBToYIQ(CRTSample(src0Pos-vec2(2.4, 0.0)).rgb)
	left := RGBToYIQ(CRTSample(src0Pos-vec2(1.2, 0.0)).rgb)
	centerSample := CRTSample(src0Pos)
	center := RGBToYIQ(centerSample.rgb)
	right := RGBToYIQ(CRTSample(src0Pos+vec2(1.2, 0.0)).rgb)
	rightFar := RGBToYIQ(CRTSample(src0Pos+vec2(2.4, 0.0)).rgb)

	// Luma receives only a light unsharp pass. I and Q use deliberately wider,
	// right-trailing kernels to reproduce composite-video colour bleed.
	luma := center.x*1.08 - left.x*0.04 - right.x*0.04
	inPhase := leftFar.y*0.18 + left.y*0.25 + center.y*0.28 +
		right.y*0.19 + rightFar.y*0.10
	quadrature := leftFar.z*0.22 + left.z*0.25 + center.z*0.24 +
		right.z*0.18 + rightFar.z*0.11
	rgb := YIQToRGB(vec3(clamp(luma, 0.0, 1.0), inPhase, quadrature))

	origin := imageSrc0Origin()
	size := imageSrc0Size()
	local := src0Pos - origin
	rowPitch := max(PixelPitch.y, 1.0)
	rowPhase := mod(local.y, rowPitch) / rowPitch
	rowEdge := abs(rowPhase-0.5) * 2.0
	scanline := 1.0
	if PixelPitch.y >= 1.35 {
		scanline = 1.0 - max(0.0, (rowEdge-0.34)/0.66)*0.32
	} else {
		scanline = 0.91 + mod(floor(local.y), 2.0)*0.09
	}

	// Offset alternate mask rows so the phosphors read as shadow-mask dots,
	// rather than a flat aperture-grille stripe.
	mask := vec3(0.875)
	maskPhase := mod(floor(local.x)+mod(floor(local.y/2.0), 2.0), 3.0)
	if maskPhase < 1.0 {
		mask.r = 1.12
	} else if maskPhase < 2.0 {
		mask.g = 1.12
	} else {
		mask.b = 1.12
	}
	mask *= 0.965 + mod(floor(local.y), 2.0)*0.035

	uv := local / size
	centered := uv*2.0 - 1.0
	vignette := 1.0 - dot(centered, centered)*0.075
	rgb = rgb*mask*scanline*vignette*1.075 + vec3(0.004)
	return vec4(clamp(rgb, vec3(0.0), vec3(1.0)), centerSample.a)
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
