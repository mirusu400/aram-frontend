package frontend

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed assets/NotoSansKR-ARAM.ttf
var notoSansKR []byte

// ARAMPalette is the semantic color layer for the frontend. UI code should
// depend on these roles instead of introducing component-local colors.
type ARAMPalette struct {
	Canvas        color.NRGBA
	CanvasRaised  color.NRGBA
	Surface       color.NRGBA
	SurfaceRaised color.NRGBA
	SurfaceHover  color.NRGBA
	Border        color.NRGBA
	BorderStrong  color.NRGBA
	Text          color.NRGBA
	TextMuted     color.NRGBA
	TextDisabled  color.NRGBA
	OnAccent      color.NRGBA
	Accent        color.NRGBA
	AccentHover   color.NRGBA
	AccentPressed color.NRGBA
	AccentSoft    color.NRGBA
	Success       color.NRGBA
	Warning       color.NRGBA
	Fault         color.NRGBA
	Overlay       color.NRGBA
}

// ARAMSpacing and ARAMRadius keep layout rhythm and shape consistent across
// desktop and mobile surfaces.
type ARAMSpacing struct {
	XXS int
	XS  int
	S   int
	M   int
	L   int
	XL  int
	XXL int
}

type ARAMRadius struct {
	Small  int
	Medium int
	Large  int
	Pill   int
}

type ARAMTypography struct {
	Caption *text.Face
	Body    *text.Face
	Strong  *text.Face
	Heading *text.Face
	Display *text.Face
}

type ARAMButtonStyle struct {
	Image     *widget.ButtonImage
	Text      *widget.ButtonTextColor
	Padding   widget.Insets
	MinHeight int
}

type ARAMComponents struct {
	MenuBar       *euiimage.NineSlice
	Toolbar       *euiimage.NineSlice
	StatusBar     *euiimage.NineSlice
	Surface       *euiimage.NineSlice
	SurfaceRaised *euiimage.NineSlice
	Dropdown      *euiimage.NineSlice
	Badge         *euiimage.NineSlice
	Divider       *euiimage.NineSlice
	ControlGroup  *euiimage.NineSlice
	Scrim         *euiimage.NineSlice
	Scroll        *widget.ScrollContainerImage
	SliderTrack   *widget.SliderTrackImage
	SliderHandle  *widget.ButtonImage
	Checkbox      *widget.CheckboxImage
	MenuButton    ARAMButtonStyle
	CommandButton ARAMButtonStyle
	SubtleButton  ARAMButtonStyle
	PrimaryButton ARAMButtonStyle
	TouchButton   ARAMButtonStyle
}

// ARAMDesignSystem is the single style entry point used by the EbitenUI shell.
// The tokens are intentionally public so platform-specific frontend code can
// reuse the same language without importing widget construction details.
type ARAMDesignSystem struct {
	Mode       string
	Palette    ARAMPalette
	Space      ARAMSpacing
	Radius     ARAMRadius
	Type       ARAMTypography
	Components ARAMComponents
	Theme      *widget.Theme
}

func newARAMDesignSystem(mode string) *ARAMDesignSystem {
	palette := aramPalette(mode)
	spacing := ARAMSpacing{XXS: 2, XS: 4, S: 8, M: 12, L: 16, XL: 24, XXL: 32}
	radius := ARAMRadius{}

	regularSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic("load embedded ARAM regular font: " + err.Error())
	}
	boldSource, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		panic("load embedded ARAM bold font: " + err.Error())
	}
	koreanSource, err := text.NewGoTextFaceSource(bytes.NewReader(notoSansKR))
	if err != nil {
		panic("load embedded ARAM Korean font: " + err.Error())
	}
	typography := ARAMTypography{
		Caption: goTextFace(regularSource, koreanSource, 11, 400),
		Body:    goTextFace(regularSource, koreanSource, 13, 400),
		Strong:  goTextFace(boldSource, koreanSource, 13, 700),
		Heading: goTextFace(boldSource, koreanSource, 16, 700),
		Display: goTextFace(boldSource, koreanSource, 22, 700),
	}

	transparent := euiimage.NewNineSliceColor(color.NRGBA{})
	menuIdle := transparent
	menuHover := roundedNineSlice(palette.SurfaceHover, color.NRGBA{}, radius.Small, 0)
	menuPressed := roundedNineSlice(palette.AccentSoft, palette.Accent, radius.Small, 1)
	commandIdle := transparent
	commandHover := roundedNineSlice(palette.SurfaceHover, color.NRGBA{}, radius.Small, 0)
	commandPressed := roundedNineSlice(palette.AccentSoft, palette.Accent, radius.Small, 1)
	commandDisabled := euiimage.NewNineSliceColor(color.NRGBA{})
	subtleIdle := transparent
	subtleHover := roundedNineSlice(palette.SurfaceHover, color.NRGBA{}, radius.Small, 0)
	subtlePressed := roundedNineSlice(palette.AccentSoft, color.NRGBA{}, radius.Small, 0)
	primaryIdle := roundedNineSlice(palette.Accent, palette.Accent, radius.Small, 1)
	primaryHover := roundedNineSlice(palette.AccentHover, palette.AccentHover, radius.Small, 1)
	primaryPressed := roundedNineSlice(palette.AccentPressed, palette.AccentPressed, radius.Small, 1)
	touchIdle := roundedNineSlice(palette.SurfaceRaised, palette.Border, radius.Medium, 1)
	touchHover := roundedNineSlice(palette.SurfaceHover, palette.BorderStrong, radius.Medium, 1)
	touchPressed := roundedNineSlice(palette.Accent, palette.Accent, radius.Medium, 1)

	components := ARAMComponents{
		MenuBar:       euiimage.NewNineSliceColor(palette.CanvasRaised),
		Toolbar:       euiimage.NewNineSliceColor(palette.Surface),
		StatusBar:     euiimage.NewNineSliceColor(palette.CanvasRaised),
		Surface:       roundedNineSlice(palette.Surface, palette.Border, radius.Large, 1),
		SurfaceRaised: roundedNineSlice(palette.SurfaceRaised, palette.BorderStrong, radius.Large, 1),
		Dropdown:      roundedNineSlice(palette.SurfaceRaised, palette.BorderStrong, radius.Medium, 1),
		Badge:         roundedNineSlice(palette.AccentSoft, palette.Accent, radius.Pill, 1),
		Divider:       euiimage.NewNineSliceColor(palette.Border),
		ControlGroup:  roundedNineSlice(palette.CanvasRaised, palette.Border, radius.Medium, 1),
		Scrim:         euiimage.NewNineSliceColor(palette.Overlay),
		Scroll: &widget.ScrollContainerImage{
			Idle: euiimage.NewNineSliceColor(color.NRGBA{}),
			Mask: euiimage.NewNineSliceColor(color.White),
		},
		SliderTrack: &widget.SliderTrackImage{
			Idle:     roundedNineSlice(palette.CanvasRaised, palette.Border, radius.Pill, 1),
			Hover:    roundedNineSlice(palette.CanvasRaised, palette.BorderStrong, radius.Pill, 1),
			Disabled: roundedNineSlice(palette.CanvasRaised, palette.Border, radius.Pill, 1),
		},
		SliderHandle: buttonImages(
			roundedNineSlice(palette.SurfaceRaised, palette.BorderStrong, radius.Small, 1),
			roundedNineSlice(palette.SurfaceHover, palette.Accent, radius.Small, 1),
			roundedNineSlice(palette.AccentSoft, palette.Accent, radius.Small, 1),
			roundedNineSlice(palette.AccentSoft, palette.Accent, radius.Small, 1),
			roundedNineSlice(palette.SurfaceRaised, palette.Border, radius.Small, 1),
		),
		Checkbox: checkboxImages(palette),
		MenuButton: ARAMButtonStyle{
			Image:     buttonImages(menuIdle, menuHover, menuPressed, menuPressed, commandDisabled),
			Text:      buttonTextColors(palette.TextMuted, palette.Text, palette.Text, palette.TextDisabled),
			Padding:   widget.Insets{Left: 8, Right: 8},
			MinHeight: 28,
		},
		CommandButton: ARAMButtonStyle{
			Image:     buttonImages(commandIdle, commandHover, commandPressed, commandPressed, commandDisabled),
			Text:      buttonTextColors(palette.Text, palette.Text, palette.Text, palette.TextDisabled),
			Padding:   widget.Insets{Left: 12, Right: 12},
			MinHeight: 34,
		},
		SubtleButton: ARAMButtonStyle{
			Image:     buttonImages(subtleIdle, subtleHover, subtlePressed, subtlePressed, commandDisabled),
			Text:      buttonTextColors(palette.TextMuted, palette.Text, palette.Text, palette.TextDisabled),
			Padding:   widget.Insets{Left: 8, Right: 8},
			MinHeight: 30,
		},
		PrimaryButton: ARAMButtonStyle{
			Image:     buttonImages(primaryIdle, primaryHover, primaryPressed, primaryPressed, commandDisabled),
			Text:      buttonTextColors(palette.OnAccent, palette.OnAccent, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 18, Right: 18},
			MinHeight: 36,
		},
		TouchButton: ARAMButtonStyle{
			Image:     buttonImages(touchIdle, touchHover, touchPressed, touchPressed, commandDisabled),
			Text:      buttonTextColors(palette.TextMuted, palette.Text, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 10, Right: 10},
			MinHeight: 44,
		},
	}

	return &ARAMDesignSystem{
		Mode:       mode,
		Palette:    palette,
		Space:      spacing,
		Radius:     radius,
		Type:       typography,
		Components: components,
		Theme: &widget.Theme{
			DefaultFace:      typography.Body,
			DefaultTextColor: palette.Text,
		},
	}
}

func defaultARAMPalette() ARAMPalette {
	return aramPalette("light")
}

func aramPalette(mode string) ARAMPalette {
	if mode == "dark" {
		return ARAMPalette{
			Canvas:        color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xff},
			CanvasRaised:  color.NRGBA{R: 0x19, G: 0x19, B: 0x19, A: 0xff},
			Surface:       color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff},
			SurfaceRaised: color.NRGBA{R: 0x26, G: 0x26, B: 0x26, A: 0xff},
			SurfaceHover:  color.NRGBA{R: 0x32, G: 0x32, B: 0x32, A: 0xff},
			Border:        color.NRGBA{R: 0x3d, G: 0x3d, B: 0x3d, A: 0xff},
			BorderStrong:  color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff},
			Text:          color.NRGBA{R: 0xf2, G: 0xf2, B: 0xf2, A: 0xff},
			TextMuted:     color.NRGBA{R: 0xb5, G: 0xb5, B: 0xb5, A: 0xff},
			TextDisabled:  color.NRGBA{R: 0x69, G: 0x69, B: 0x69, A: 0xff},
			OnAccent:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			Accent:        color.NRGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff},
			AccentHover:   color.NRGBA{R: 0x18, G: 0x8c, B: 0xe8, A: 0xff},
			AccentPressed: color.NRGBA{R: 0x00, G: 0x5a, B: 0x9e, A: 0xff},
			AccentSoft:    color.NRGBA{R: 0x39, G: 0x39, B: 0x39, A: 0xff},
			Success:       color.NRGBA{R: 0x35, G: 0xa8, B: 0x53, A: 0xff},
			Warning:       color.NRGBA{R: 0xd9, G: 0x9b, B: 0x26, A: 0xff},
			Fault:         color.NRGBA{R: 0xe0, G: 0x52, B: 0x5c, A: 0xff},
			Overlay:       color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xb0},
		}
	}
	return ARAMPalette{
		Canvas:        color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff},
		CanvasRaised:  color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Surface:       color.NRGBA{R: 0xf7, G: 0xf7, B: 0xf7, A: 0xff},
		SurfaceRaised: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		SurfaceHover:  color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff},
		Border:        color.NRGBA{R: 0xd0, G: 0xd0, B: 0xd0, A: 0xff},
		BorderStrong:  color.NRGBA{R: 0xa9, G: 0xa9, B: 0xa9, A: 0xff},
		Text:          color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff},
		TextMuted:     color.NRGBA{R: 0x5f, G: 0x5f, B: 0x5f, A: 0xff},
		TextDisabled:  color.NRGBA{R: 0xa2, G: 0xa2, B: 0xa2, A: 0xff},
		OnAccent:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Accent:        color.NRGBA{R: 0x00, G: 0x67, B: 0xc0, A: 0xff},
		AccentHover:   color.NRGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff},
		AccentPressed: color.NRGBA{R: 0x00, G: 0x4f, B: 0x94, A: 0xff},
		AccentSoft:    color.NRGBA{R: 0xe5, G: 0xf1, B: 0xfb, A: 0xff},
		Success:       color.NRGBA{R: 0x21, G: 0x78, B: 0x3b, A: 0xff},
		Warning:       color.NRGBA{R: 0x9a, G: 0x67, B: 0x00, A: 0xff},
		Fault:         color.NRGBA{R: 0xc4, G: 0x2b, B: 0x38, A: 0xff},
		Overlay:       color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66},
	}
}

func goTextFace(
	source *text.GoTextFaceSource,
	koreanSource *text.GoTextFaceSource,
	size float64,
	weight float32,
) *text.Face {
	primary := &text.GoTextFace{Source: source, Size: size}
	korean := &text.GoTextFace{Source: koreanSource, Size: size}
	korean.SetVariation(text.MustParseTag("wght"), weight)
	multi, err := text.NewMultiFace(primary, korean)
	if err != nil {
		panic("create ARAM font fallback: " + err.Error())
	}
	var face text.Face = multi
	return &face
}

func buttonImages(
	idle *euiimage.NineSlice,
	hover *euiimage.NineSlice,
	pressed *euiimage.NineSlice,
	pressedHover *euiimage.NineSlice,
	disabled *euiimage.NineSlice,
) *widget.ButtonImage {
	return &widget.ButtonImage{
		Idle:         idle,
		Hover:        hover,
		Pressed:      pressed,
		PressedHover: pressedHover,
		Disabled:     disabled,
	}
}

func buttonTextColors(idle, hover, pressed, disabled color.Color) *widget.ButtonTextColor {
	return &widget.ButtonTextColor{
		Idle:     idle,
		Hover:    hover,
		Pressed:  pressed,
		Disabled: disabled,
	}
}

func checkboxImages(palette ARAMPalette) *widget.CheckboxImage {
	unchecked := checkboxImage(
		palette.SurfaceRaised,
		palette.BorderStrong,
		nil,
	)
	uncheckedHover := checkboxImage(
		palette.SurfaceHover,
		palette.Accent,
		nil,
	)
	checked := checkboxImage(
		palette.Accent,
		palette.Accent,
		palette.OnAccent,
	)
	checkedHover := checkboxImage(
		palette.AccentHover,
		palette.AccentHover,
		palette.OnAccent,
	)
	uncheckedDisabled := checkboxImage(
		palette.Surface,
		palette.Border,
		nil,
	)
	checkedDisabled := checkboxImage(
		palette.Border,
		palette.Border,
		palette.TextDisabled,
	)
	return &widget.CheckboxImage{
		Unchecked:         unchecked,
		UncheckedHovered:  uncheckedHover,
		UncheckedDisabled: uncheckedDisabled,
		Checked:           checked,
		CheckedHovered:    checkedHover,
		CheckedDisabled:   checkedDisabled,
	}
}

func checkboxImage(
	fill color.Color,
	border color.Color,
	checkmark color.Color,
) *euiimage.NineSlice {
	const size = 24
	result := ebiten.NewImage(size, size)
	euiimage.NewBorderedNineSliceColor(fill, border, 2).
		Draw(result, size, size, nil)
	if checkmark != nil {
		vector.StrokeLine(result, 5, 12, 10, 17, 2.5, checkmark, true)
		vector.StrokeLine(result, 10, 17, 20, 7, 2.5, checkmark, true)
	}
	return euiimage.NewFixedNineSlice(result)
}

// roundedNineSlice rasterizes a small scalable rounded surface once. Runtime
// widgets then use EbitenUI's nine-slice renderer without per-frame geometry.
func roundedNineSlice(fill, border color.Color, radius, borderWidth int) *euiimage.NineSlice {
	if radius <= 0 {
		if borderWidth <= 0 {
			return euiimage.NewNineSliceColor(fill)
		}
		return euiimage.NewBorderedNineSliceColor(fill, border, borderWidth)
	}
	radius = max(1, radius)
	size := radius*2 + 3
	source := image.NewNRGBA(image.Rect(0, 0, size, size))
	fillNRGBA := color.NRGBAModel.Convert(fill).(color.NRGBA)
	borderNRGBA := color.NRGBAModel.Convert(border).(color.NRGBA)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !insideRoundedRect(x, y, 0, 0, size, size, radius) {
				continue
			}
			pixel := borderNRGBA
			if borderWidth == 0 || insideRoundedRect(
				x,
				y,
				borderWidth,
				borderWidth,
				size-borderWidth*2,
				size-borderWidth*2,
				max(0, radius-borderWidth),
			) {
				pixel = fillNRGBA
			}
			source.SetNRGBA(x, y, pixel)
		}
	}

	corner := radius + 1
	return euiimage.NewNineSlice(
		ebiten.NewImageFromImage(source),
		[3]int{corner, 1, corner},
		[3]int{corner, 1, corner},
	)
}

func insideRoundedRect(x, y, left, top, width, height, radius int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	right := left + width - 1
	bottom := top + height - 1
	if x < left || x > right || y < top || y > bottom {
		return false
	}
	if radius <= 0 {
		return true
	}

	centerX := min(max(x, left+radius), right-radius)
	centerY := min(max(y, top+radius), bottom-radius)
	dx := x - centerX
	dy := y - centerY
	return dx*dx+dy*dy <= radius*radius
}

func (d *ARAMDesignSystem) button(
	label string,
	style ARAMButtonStyle,
	face *text.Face,
	width int,
	height int,
	position widget.TextPosition,
	clicked func(),
) *widget.Button {
	height = max(height, style.MinHeight)
	padding := style.Padding
	return widget.NewButton(
		widget.ButtonOpts.WidgetOpts(widget.WidgetOpts.MinSize(width, height)),
		widget.ButtonOpts.Image(style.Image),
		widget.ButtonOpts.Text(label, face, style.Text),
		widget.ButtonOpts.TextPosition(position, widget.TextPositionCenter),
		widget.ButtonOpts.TextPadding(&padding),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
			if clicked != nil {
				clicked()
			}
		}),
	)
}

func (d *ARAMDesignSystem) text(
	label string,
	face *text.Face,
	textColor color.Color,
	layoutData interface{},
) *widget.Text {
	options := []widget.TextOpt{
		widget.TextOpts.Text(label, face, textColor),
	}
	if layoutData != nil {
		options = append(options, widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(layoutData)))
	}
	return widget.NewText(options...)
}
