package frontend

import (
	"image/color"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

// applyRetroSkin swaps the palette and component images of a freshly built
// modern design system for the sprite-backed retro skin. Typography, spacing
// and radius tokens are shared: the sprites carry their own shape, so radius
// stays a layout-only concern.
func applyRetroSkin(ds *ARAMDesignSystem, family string) {
	theme := retroThemeID(family, ds.Mode)
	ds.Family = family
	ds.Palette = retroPalette(theme, ds.Palette)
	ds.Components = retroComponents(theme, ds.Palette)
	ds.Theme = &widget.Theme{
		DefaultFace:      ds.Type.Body,
		DefaultTextColor: ds.Palette.Text,
	}
}

// retroComponents maps the pack's slice vocabulary (panel, titlebar, selection,
// …) onto the component roles the shell consumes. The checkbox stays
// vector-drawn from the retro palette because the pack ships no checkbox tile.
func retroComponents(theme string, palette ARAMPalette) ARAMComponents {
	transparent := euiimage.NewNineSliceColor(color.NRGBA{})
	button := retroButtonImage(theme, "button")
	primary := retroButtonImage(theme, "button_primary")
	hover := retroNineSlice(theme, "button_hover")
	selection := retroNineSlice(theme, "selection")
	panel := retroNineSlice(theme, "panel")
	sunken := retroNineSlice(theme, "panel_sunken")
	statusBar := retroNineSlice(theme, "statusbar")
	progressTrack := retroNineSlice(theme, "progress_track")

	return ARAMComponents{
		MenuBar:       statusBar,
		Toolbar:       panel,
		StatusBar:     statusBar,
		Surface:       panel,
		SurfaceRaised: panel,
		DialogTitle:   retroNineSlice(theme, "titlebar"),
		DialogBody:    panel,
		NavRail:       sunken,
		Dropdown:      retroNineSlice(theme, "tooltip"),
		Badge:         selection,
		Divider:       euiimage.NewNineSliceColor(palette.Border),
		ControlGroup:  sunken,
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
			Image: buttonImages(transparent, hover, selection, selection, transparent),
			Text: buttonTextColors(
				palette.Text, palette.Text, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 8, Right: 8},
			MinHeight: 28,
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
			Image: primary,
			Text: buttonTextColors(
				palette.OnAccent, palette.OnAccent, palette.OnAccent, palette.TextDisabled),
			Padding:   widget.Insets{Left: 18, Right: 18},
			MinHeight: 36,
		},
		TouchButton: ARAMButtonStyle{
			Image: button,
			Text: buttonTextColors(
				palette.Text, palette.Text, palette.Text, palette.TextDisabled),
			Padding:   widget.Insets{Left: 10, Right: 10},
			MinHeight: 44,
		},
	}
}
