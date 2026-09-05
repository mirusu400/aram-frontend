package frontend

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

// recentWindowWidth is the fixed preferred width of the Open Recent dialog.
// Entry labels are budgeted against this width (clamped by the viewport), not
// the app viewport, so the focused row never overflows its cell.
const recentWindowWidth = 760

func (u *shellUI) syncRecentPanel(shell *Shell) {
	recent := append([]RecentEntry(nil), shell.settings.RecentFiles...)
	signature := recentPanelSignature(shell, u.viewportWidth, u.viewportHeight, recent)
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

	// The Open Recent list lives inside a fixed-width window (see the
	// centeredWindowRect call below), not the full app viewport. Budget each
	// entry label to the list cell width, minus room for the icon tile, so
	// the widest row can never overflow its cell.
	windowWidth := min(recentWindowWidth, u.viewportWidth-2*centeredWindowMargin)
	labelWidth := max(28, min(90, (windowWidth-174)/7))
	detailWidth := max(28, min(104, (windowWidth-96)/7))

	selectedPath := u.recentSelectedPath
	if !recentContainsPath(recent, selectedPath) {
		selectedPath = ""
		if len(recent) > 0 {
			selectedPath = recent[0].Path
		}
	}
	u.recentSelectedPath = selectedPath

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
		u.recentScroll = nil
		u.recentRowPaths = nil
		u.recentSelectedPath = ""
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

	// Each row is a real widget.Button (not a plain clickable container) so
	// it keeps whatever focus/keyboard behavior ebitenui gives buttons
	// generally; only its background image tracks selection.
	unselectedImage := &widget.ButtonImage{
		Idle:    euiimage.NewNineSliceColor(color.NRGBA{}),
		Hover:   euiimage.NewNineSliceColor(design.Palette.SurfaceHover),
		Pressed: euiimage.NewNineSliceColor(design.Palette.AccentSoft),
	}
	selectedImage := &widget.ButtonImage{
		Idle:    euiimage.NewNineSliceColor(design.Palette.AccentSoft),
		Hover:   euiimage.NewNineSliceColor(design.Palette.AccentSoft),
		Pressed: euiimage.NewNineSliceColor(design.Palette.AccentSoft),
	}
	textColor := &widget.ButtonTextColor{
		Idle:     design.Palette.Text,
		Disabled: design.Palette.TextDisabled,
	}

	rowButtons := make(map[string]*widget.Button, len(recent))
	selectRow := func(path string) {
		if previous, ok := rowButtons[u.recentSelectedPath]; ok {
			previous.SetImage(unselectedImage)
		}
		u.recentSelectedPath = path
		selectedPath = path
		if current, ok := rowButtons[path]; ok {
			current.SetImage(selectedImage)
		}
		pathText.Label = recentPathDetails(path, detailWidth, 5, shell.language())
		openButton.GetWidget().Disabled = path == ""
	}

	rowContent := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
		)),
	)
	u.recentRowPaths = u.recentRowPaths[:0]
	for _, entry := range recent {
		entry := entry
		name := entry.Name
		if name == "" {
			name = filepath.Base(filepath.Clean(entry.Path))
		}
		image := unselectedImage
		if entry.Path == selectedPath {
			image = selectedImage
		}
		row := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
				widget.WidgetOpts.MinSize(0, 40),
			),
			widget.ButtonOpts.Image(image),
			widget.ButtonOpts.TextAndImage(
				recentEntryLabel(entry.Name, entry.Path, labelWidth),
				design.Type.Body,
				&widget.GraphicImage{Idle: recentRowIconImage(shell, design, entry.Path, name)},
				textColor,
			),
			widget.ButtonOpts.TextPosition(widget.TextPositionStart, widget.TextPositionCenter),
			widget.ButtonOpts.TextPadding(&widget.Insets{
				Left:   design.Space.S,
				Top:    8,
				Right:  design.Space.S,
				Bottom: 8,
			}),
			widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
				selectRow(entry.Path)
			}),
		)
		rowButtons[entry.Path] = row
		rowContent.AddChild(row)
		u.recentRowPaths = append(u.recentRowPaths, entry.Path)
	}

	var recentScroll *widget.ScrollContainer
	recentScroll = widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(rowContent),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(design.Components.Scroll),
		widget.ScrollContainerOpts.WidgetOpts(
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
			widget.WidgetOpts.ScrolledHandler(func(args *widget.WidgetScrolledEventArgs) {
				scrollContainerByWheel(recentScroll, args.Y)
			}),
		),
	)
	u.recentScroll = recentScroll
	contents.AddChild(recentScroll)

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
			recentWindowWidth,
			580,
		)),
	)
	u.panelWindow = recentWindow
	u.ui.AddWindow(recentWindow)
}

// recentPanelSignature encodes everything the Open Recent dialog renders, so a
// rebuild happens only when something visible changes — including an icon
// that finishes loading after the dialog is already open.
func recentPanelSignature(shell *Shell, viewportWidth, viewportHeight int, entries []RecentEntry) string {
	builder := make([]byte, 0, 128)
	builder = fmt.Appendf(builder, "recent|%dx%d|", viewportWidth, viewportHeight)
	for _, entry := range entries {
		builder = append(builder, entry.Path...)
		builder = append(builder, 0x1f)
		builder = append(builder, entry.Name...)
		if shell.homeIcon(entry.Path) != nil {
			builder = append(builder, 0x03)
		}
		builder = append(builder, 0x00)
	}
	return string(builder)
}

// recentRowIconImage returns the entry's extracted icon when the backend has
// provided one, otherwise the same monogram placeholder as the Home launcher
// rows (see homeIconPlaceholder in ui_home.go), so a title looks the same
// wherever it is listed.
func recentRowIconImage(shell *Shell, design *ARAMDesignSystem, path, name string) *ebiten.Image {
	shell.requestHomeIcon(path)
	if icon := shell.homeIcon(path); icon != nil {
		return scaleIconToTile(icon)
	}
	return homeIconPlaceholder(design, path, name)
}

func recentContainsPath(entries []RecentEntry, path string) bool {
	if path == "" {
		return false
	}
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

// recentEntryLabel formats one row as "name — parent directory", preferring
// the display name captured when the input was opened (see RecentEntry) over
// one derived from path, since path is not always readable on its own (a
// temporary drop copy, an Android content:// URI).
func recentEntryLabel(name, path string, width int) string {
	if name == "" {
		name = filepath.Base(filepath.Clean(path))
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = path
		}
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
