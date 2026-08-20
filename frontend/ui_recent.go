package frontend

import (
	"fmt"
	"path/filepath"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

func (u *shellUI) syncRecentPanel(shell *Shell) {
	recent := append([]string(nil), shell.settings.RecentFiles...)
	signature := fmt.Sprintf(
		"recent|%dx%d|%s",
		u.viewportWidth,
		u.viewportHeight,
		strings.Join(recent, "\x00"),
	)
	if signature == u.panelSignature && u.panelWindow != nil {
		return
	}

	u.closePanel()
	u.panelSignature = signature
	u.scrim.GetWidget().SetVisibility(widget.Visibility_Show)
	design := u.design
	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.DialogBody),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	contents.AddChild(design.text(
		shell.trf(
			"%d recent inputs — select one to inspect its full path.",
			len(recent),
		),
		design.Type.Body,
		design.Palette.TextMuted,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			Padding: &widget.Insets{
				Left: design.Space.XL,
				Top:  design.Space.L,
			},
		},
	))

	labelWidth := max(28, min(104, (u.viewportWidth-96)/7))
	detailWidth := max(28, min(104, (u.viewportWidth-84)/7))
	selectedPath := ""
	if len(recent) > 0 {
		selectedPath = recent[0]
	}
	pathText := widget.NewText(
		widget.TextOpts.Text(
			recentPathDetails(
				selectedPath,
				detailWidth,
				5,
				shell.language(),
			),
			design.Type.Caption,
			design.Palette.TextMuted,
		),
		widget.TextOpts.MaxWidth(float64(max(240, u.viewportWidth-96))),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionEnd,
				StretchHorizontal:  true,
				Padding: &widget.Insets{
					Left:   design.Space.XL,
					Right:  design.Space.XL,
					Bottom: 62,
				},
			}),
			widget.WidgetOpts.MinSize(0, 92),
		),
	)
	contents.AddChild(pathText)

	var recentWindow *widget.Window
	closeRecent := func() {
		shell.panel = nil
		if recentWindow != nil {
			recentWindow.Close()
		}
		u.panelWindow = nil
		u.panelSignature = ""
		u.recentList = nil
		u.scrim.GetWidget().SetVisibility(widget.Visibility_Hide)
	}
	openButton := design.button(
		shell.tr("Open"),
		design.Components.PrimaryButton,
		design.Type.Strong,
		96,
		design.Components.PrimaryButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			path := selectedPath
			closeRecent()
			shell.openRecentPath(path)
		},
	)
	openButton.GetWidget().Disabled = selectedPath == ""
	openButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(openButton)

	cancelButton := design.button(
		shell.tr("Cancel"),
		design.Components.SubtleButton,
		design.Type.Strong,
		96,
		design.Components.SubtleButton.MinHeight,
		widget.TextPositionCenter,
		func() {
			closeRecent()
			shell.setStatus(shell.tr("Open recent canceled"))
		},
	)
	cancelButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionEnd,
		Padding: &widget.Insets{
			Right:  design.Space.L + 104,
			Bottom: design.Space.M,
		},
	}
	contents.AddChild(cancelButton)

	entries := make([]any, len(recent))
	for index, path := range recent {
		entries[index] = path
	}
	trackIdle := euiimage.NewNineSliceColor(design.Palette.Border)
	trackHover := euiimage.NewNineSliceColor(design.Palette.BorderStrong)
	recentList := widget.NewList(
		widget.ListOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
					HorizontalPosition: widget.AnchorLayoutPositionStart,
					VerticalPosition:   widget.AnchorLayoutPositionStart,
					StretchHorizontal:  true,
					StretchVertical:    true,
					Padding: &widget.Insets{
						Left:   design.Space.XL,
						Top:    48,
						Right:  design.Space.XL,
						Bottom: 172,
					},
				}),
			),
		),
		widget.ListOpts.Entries(entries),
		widget.ListOpts.ScrollContainerImage(design.Components.Scroll),
		widget.ListOpts.SliderParams(&widget.SliderParams{
			TrackImage: &widget.SliderTrackImage{
				Idle:     trackIdle,
				Hover:    trackHover,
				Disabled: trackIdle,
			},
			HandleImage:   design.Components.TouchButton.Image,
			MinHandleSize: intPointer(28),
			TrackPadding:  &widget.Insets{Left: 3, Right: 3},
		}),
		widget.ListOpts.ControlWidgetSpacing(design.Space.XS),
		widget.ListOpts.HideHorizontalSlider(),
		widget.ListOpts.EntryFontFace(design.Type.Body),
		widget.ListOpts.EntryColor(&widget.ListEntryColor{
			Unselected:                 design.Palette.Text,
			Selected:                   design.Palette.Text,
			DisabledUnselected:         design.Palette.TextDisabled,
			DisabledSelected:           design.Palette.TextDisabled,
			SelectingBackground:        design.Palette.SurfaceHover,
			SelectedBackground:         design.Palette.AccentSoft,
			FocusedBackground:          design.Palette.SurfaceHover,
			SelectingFocusedBackground: design.Palette.AccentSoft,
			SelectedFocusedBackground:  design.Palette.AccentSoft,
			DisabledSelectedBackground: design.Palette.Surface,
		}),
		widget.ListOpts.EntryLabelFunc(func(entry any) string {
			path, _ := entry.(string)
			return recentEntryLabel(path, labelWidth)
		}),
		widget.ListOpts.EntryTextPadding(&widget.Insets{
			Left:   design.Space.S,
			Top:    11,
			Right:  design.Space.S,
			Bottom: 11,
		}),
		widget.ListOpts.EntryTextPosition(
			widget.TextPositionStart,
			widget.TextPositionCenter,
		),
		widget.ListOpts.SelectFocus(),
		widget.ListOpts.EntrySelectedHandler(func(args *widget.ListEntrySelectedEventArgs) {
			path, _ := args.Entry.(string)
			selectedPath = path
			pathText.Label = recentPathDetails(
				path,
				detailWidth,
				5,
				shell.language(),
			)
			openButton.GetWidget().Disabled = path == ""
		}),
	)
	u.recentList = recentList
	contents.AddChild(recentList)

	titleBar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(design.Components.DialogTitle),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	titleBar.AddChild(design.text(
		shell.tr("Open Recent"),
		design.Type.Heading,
		design.Palette.OnTitle,
		widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
			Padding:            &widget.Insets{Left: design.Space.XL},
		},
	))
	recentWindow = widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.TitleBar(titleBar, 42),
		widget.WindowOpts.Modal(),
		widget.WindowOpts.Draggable(),
		widget.WindowOpts.Location(centeredWindowRect(
			u.viewportWidth,
			u.viewportHeight,
			760,
			580,
		)),
	)
	u.panelWindow = recentWindow
	u.ui.AddWindow(recentWindow)
	if selectedPath != "" {
		recentList.SetSelectedEntry(selectedPath)
	}
}

func recentEntryLabel(path string, width int) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = path
	}
	parent := filepath.Dir(path)
	nameLimit := max(12, width*2/5)
	parentLimit := max(10, width-nameLimit-5)
	return shorten(name, nameLimit) + "  —  " + shorten(parent, parentLimit)
}

func recentPathDetails(path string, width, limit int, languages ...Language) string {
	language := LanguageEnglish
	if len(languages) > 0 {
		language = languages[0]
	}
	if strings.TrimSpace(path) == "" {
		return translate(language, "Select an input to view its full path.")
	}
	lines := wrapPanelLines(
		[]string{translatef(language, "Full path: %s", path)},
		max(12, width),
		1024,
	)
	if len(lines) > limit {
		lines = lines[:limit]
		last := strings.TrimSpace(lines[limit-1])
		lines[limit-1] = strings.TrimSuffix(last, "...") + "..."
	}
	return strings.Join(lines, "\n")
}
