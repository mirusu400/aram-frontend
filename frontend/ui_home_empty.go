package frontend

import (
	"math"
	"strings"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (u *shellUI) homeEmptyContent(shell *Shell, tab string, folders []string, width int) *widget.Container {
	design := u.design
	content := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(design.Space.M),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionCenter,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
		})),
	)
	content.AddChild(homeText(homeEmptyMessage(shell, tab, folders, shell.libraryScanning),
		design.Type.Body, homeColorMuted, widget.RowLayoutData{Position: widget.RowLayoutPositionCenter},
		max(1, width-design.px(64))))
	addButton := func(id, label string, primary bool, action func()) {
		style := design.Components.SubtleButton
		if primary {
			style = design.Components.PrimaryButton
		}
		button := design.button(shell.tr(label), style, design.Type.Strong,
			min(design.px(260), max(1, width-design.px(64))),
			max(design.px(44), style.MinHeight), widget.TextPositionCenter, action)
		button.GetWidget().CustomData = id
		if id == "home.add_folder" && !platformLibraryFolderPickerAvailable() {
			button.GetWidget().Disabled = true
		}
		button.GetWidget().LayoutData = widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}
		content.AddChild(button)
	}
	if strings.TrimSpace(shell.homeFilterQuery) != "" {
		addButton("home.clear_search", "Clear search", true, func() { u.clearHomeSearch(shell) })
	} else if !shell.libraryScanning {
		addButton("file.open", "Open game file", true, shell.chooseFile)
		addButton("home.add_folder", "Add game folder", false, shell.chooseLibraryFolder)
		if !platformLibraryFolderPickerAvailable() {
			content.AddChild(homeText(shell.tr("In the browser, open individual game files instead of folders."),
				design.Type.Caption, homeColorMuted, widget.RowLayoutData{Position: widget.RowLayoutPositionCenter},
				max(1, width-design.px(64))))
		}
	}
	return content
}

func (s *Shell) homeFavoriteActionLabel(path string) string {
	if s.settings.isFavorite(path) {
		return s.tr("Remove favorite")
	}
	return s.tr("Add favorite")
}

// Draw the star instead of relying on a glyph absent from the pixel fonts.
func homeFavoriteIcon(design *ARAMDesignSystem) *ebiten.Image {
	size := design.px(16)
	icon := ebiten.NewImage(size, size)
	var star vector.Path
	center := float64(size) / 2
	for point := 0; point < 10; point++ {
		radius := center - normalizedRenderScale(design.Scale)
		if point%2 == 1 {
			radius *= 0.45
		}
		angle := float64(point)*math.Pi/5 - math.Pi/2
		x, y := float32(center+math.Cos(angle)*radius), float32(center+math.Sin(angle)*radius)
		if point == 0 {
			star.MoveTo(x, y)
		} else {
			star.LineTo(x, y)
		}
	}
	star.Close()
	var options vector.DrawPathOptions
	options.ColorScale.ScaleWithColor(homeColorStar)
	options.AntiAlias = true
	vector.FillPath(icon, &star, nil, &options)
	return icon
}
