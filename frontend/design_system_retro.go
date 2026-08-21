package frontend

import (
	"image/color"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

// applyRetroSkin swaps the palette, component images, and type ramp of a
// freshly built modern design system for the sprite-backed retro skin.
// Spacing and radius tokens are shared: the sprites carry their own shape,
// so radius stays a layout-only concern.
func applyRetroSkin(ds *ARAMDesignSystem, family string) {
	theme := retroThemeID(family, ds.Mode)
	ds.Family = family
	ds.Palette = retroPalette(theme, ds.Palette)
	ds.Components = retroComponents(theme, family, ds.Palette)
	ds.Type = retroTypography()
	ds.Theme = &widget.Theme{
		DefaultFace:      ds.Type.Body,
		DefaultTextColor: ds.Palette.Text,
		// Applies to every text widget that does not set its own padding,
		// including the ones EbitenUI builds inside buttons and lists.
		TextTheme: &widget.TextParams{
			Face:    ds.Type.Body,
			Color:   ds.Palette.Text,
			Padding: retroTextPadding(),
		},
	}
}

// retroTheme reports the sprite theme id backing this design system; ok is
// false for the modern vector-drawn family.
func (d *ARAMDesignSystem) retroTheme() (string, bool) {
	if !isRetroFamily(d.Family) {
		return "", false
	}
	return retroThemeID(d.Family, d.Mode), true
}

// retroIcon returns the pack's 16×16 pixel icon for sprite skins and nil for
// the modern family, letting UI code fall back to text-only controls.
func (d *ARAMDesignSystem) retroIcon(name string) *widget.GraphicImage {
	theme, ok := d.retroTheme()
	if !ok {
		return nil
	}
	return retroIconGraphic(theme, name)
}

// retroComponents maps the pack's slice vocabulary (panel, titlebar, selection,
// …) onto the component roles the shell consumes. The checkbox stays
// vector-drawn from the retro palette because the pack ships no checkbox tile.
func retroComponents(theme, family string, palette ARAMPalette) ARAMComponents {
	transparent := euiimage.NewNineSliceColor(color.NRGBA{})
	button := retroButtonImage(theme, "button")
	hover := retroNineSlice(theme, "button_hover")
	selection := retroNineSlice(theme, "selection")
	panel := retroNineSlice(theme, "panel")
	sunken := retroNineSlice(theme, "panel_sunken")
	statusBar := retroNineSlice(theme, "statusbar")
	progressTrack := retroNineSlice(theme, "progress_track")
	// Primary actions wear the soft-key face (확인/취소 in the era's shells);
	// the pack ships it with idle and pressed states only.
	softKeyPressed := retroNineSlice(theme, "softkey_pressed")
	softKey := &widget.ButtonImage{
		Idle:         retroNineSlice(theme, "softkey_idle"),
		Hover:        retroNineSlice(theme, "softkey_idle"),
		Pressed:      softKeyPressed,
		PressedHover: softKeyPressed,
		Disabled:     retroNineSlice(theme, "button_disabled"),
	}

	return ARAMComponents{
		MenuBar:       statusBar,
		Toolbar:       panel,
		StatusBar:     statusBar,
		Surface:       panel,
		SurfaceRaised: panel,
		DialogTitle:   retroNineSlice(theme, "titlebar"),
		DialogBody:    panel,
		NavRail:       sunken,
		Dropdown:      panel,
		Badge:         selection,
		Divider:       euiimage.NewNineSliceColor(palette.Border),
		ControlGroup:  sunken,
		LCDBezel:      retroNineSlice(theme, "lcd_bezel"),
		Scrim:         euiimage.NewNineSliceColor(palette.Overlay),
		Scroll: &widget.ScrollContainerImage{
			Idle: retroNineSlice(theme, "scroll_track"),
			Mask: euiimage.NewNineSliceColor(color.White),
		},
		SliderTrack: &widget.SliderTrackImage{
			Idle:     progressTrack,
			Hover:    progressTrack,
			Disabled: progressTrack,
		},
		SliderHandle: button,
		Checkbox:     checkboxImages(palette),
		MenuButton: ARAMButtonStyle{
			// Hover already draws the era-defining accent gradient bar.
			Image: buttonImages(transparent, selection, selection, selection, transparent),
			Text: buttonTextColors(
				palette.Text, palette.OnAccent, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 8, Right: 8},
			MinHeight: menuRowHeight,
		},
		CommandButton: ARAMButtonStyle{
			Image: button,
			Text: buttonTextColors(
				palette.Text, palette.Text, palette.Text, palette.TextDisabled),
			Padding:   widget.Insets{Left: 12, Right: 12},
			MinHeight: 34,
		},
		SubtleButton: ARAMButtonStyle{
			Image: buttonImages(transparent, hover, selection, selection, transparent),
			Text: buttonTextColors(
				palette.TextMuted, palette.Text, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 8, Right: 8},
			MinHeight: 30,
		},
		PrimaryButton: ARAMButtonStyle{
			Image: softKey,
			Text: buttonTextColors(
				palette.OnWarm, palette.OnWarm, palette.OnWarm, palette.TextDisabled),
			Padding:   widget.Insets{Left: 18, Right: 18},
			MinHeight: 36,
		},
		TouchButton: ARAMButtonStyle{
			// A held key lights up with the accent fill, like the pressed
			// digit on the pack's keypad mockups. Keys are the largest
			// controls the shell draws — several times the 17px tile — so
			// they take the doubled slice, which keeps the gloss band and the
			// drop shadow at a thickness the eye still reads as a moulded key.
			Image: &widget.ButtonImage{
				Idle:         retroScaledNineSlice(theme, "button_idle", retroKeyScale),
				Hover:        retroScaledNineSlice(theme, "button_hover", retroKeyScale),
				Pressed:      retroScaledNineSlice(theme, "button_primary_pressed", retroKeyScale),
				PressedHover: retroScaledNineSlice(theme, "button_primary_pressed", retroKeyScale),
				Disabled:     retroScaledNineSlice(theme, "button_disabled", retroKeyScale),
			},
			// Full ink, not the muted role: a key legend is the label the
			// player reads mid-game, and the pack's own keypad sheet draws it
			// in Text. Only the modes whose pressed face fills solid with the
			// accent (flat, neon) invert the legend on press; the gloss, jelly,
			// and glass faces keep enough body under the fill to stay legible.
			Text: buttonTextColors(
				palette.Text, palette.Text,
				retroKeyPressedInk(family, palette), palette.TextDisabled),
			Padding:   widget.Insets{Left: 10, Right: 10},
			MinHeight: 44,
		},
	}
}

// retroKeyScale is the integer pixel scale the deck and keypad keys draw their
// nine-slices at. Doubling is the largest scale a 44px minimum key can carry:
// the doubled tile needs 8*2 + 1*2 + 8*2 = 34px before its center stretches.
const retroKeyScale = 2

// retroStyleMode reports the pack's style family for a skin, which decides how
// a pressed key's legend has to be inked.
func retroStyleMode(family string) string {
	switch family {
	case "chrome-blue":
		return "gloss"
	case "candy-orange":
		return "jelly"
	case "mono-lcd":
		return "flat"
	case "glass-touch":
		return "glass"
	case "neon-edge":
		return "neon"
	}
	return ""
}

// retroKeyPressedInk picks the legend color for a held key.
func retroKeyPressedInk(family string, palette ARAMPalette) color.Color {
	switch retroStyleMode(family) {
	case "flat", "neon":
		return palette.OnAccent
	}
	return palette.Text
}

// retroKeyLegendShadow returns the 1px shadow a key legend needs behind it,
// and false where none is wanted. The glass skin's gel highlight covers the
// top half of every key, and where the legend is itself pale it dissolves into
// that highlight without a dark offset copy behind it. A dark legend on the
// same gel already separates, so it gets none: an extra offset copy under
// pixel glyphs only thickens them.
func retroKeyLegendShadow(family string, palette ARAMPalette) (color.Color, bool) {
	if retroStyleMode(family) != "glass" {
		return nil, false
	}
	if relativeLuminance(palette.Text) <= relativeLuminance(palette.Canvas) {
		return nil, false
	}
	// The pack draws this in its hard outline color, which the semantic
	// palette does not carry as a role of its own; on the dark skin that color
	// is the canvas, the darkest thing the palette holds.
	if relativeLuminance(palette.Canvas) < relativeLuminance(palette.BorderStrong) {
		return palette.Canvas, true
	}
	return palette.BorderStrong, true
}

// relativeLuminance is a cheap perceptual weighting, enough to rank two
// palette colors by darkness.
func relativeLuminance(c color.Color) float64 {
	red, green, blue, _ := c.RGBA()
	return 0.2126*float64(red) + 0.7152*float64(green) + 0.0722*float64(blue)
}

// retroIndicatorIcon returns a pack glyph repainted in one color, and nil for
// the modern family. The status indicators reuse a single glyph across several
// readings, so the reading is carried by the ink.
func (d *ARAMDesignSystem) retroIndicatorIcon(
	name string,
	tint color.Color,
) *ebiten.Image {
	theme, ok := d.retroTheme()
	if !ok {
		return nil
	}
	return retroTintedIcon(theme, name, tint)
}
