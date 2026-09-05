package frontend

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Home launcher skin — a dark feature-phone app selector, modeled on the
// handset launcher screenshot: green-underlined tabs, tan row numbers, colored
// app tiles, and a soft-key bar. These colors are deliberately their own fixed
// (always-dark) palette, not the app design system, because Home stands in for
// the device's own screen.
var (
	homeColorBg          = color.NRGBA{0x0e, 0x10, 0x12, 0xff}
	homeColorSoftbar     = color.NRGBA{0x07, 0x08, 0x0a, 0xff}
	homeColorDivider     = color.NRGBA{0x2b, 0x2f, 0x33, 0xff}
	homeColorRowSelect   = color.NRGBA{0x22, 0x3a, 0x1f, 0xff}
	homeColorTabActive   = color.NRGBA{0x8b, 0xc9, 0x3f, 0xff}
	homeColorTabIdle     = color.NRGBA{0xb6, 0xbb, 0xc0, 0xff}
	homeColorNumber      = color.NRGBA{0xd2, 0xb2, 0x5c, 0xff}
	homeColorName        = color.NRGBA{0xed, 0xef, 0xf1, 0xff}
	homeColorStar        = color.NRGBA{0xe8, 0xc0, 0x55, 0xff}
	homeColorMuted       = color.NRGBA{0x8a, 0x8f, 0x94, 0xff}
	homeColorTransparent = color.NRGBA{0, 0, 0, 0}
)

// homeIconPalette gives each title a stable, distinct tile color, standing in
// for the per-app icons on the reference handset.
var homeIconPalette = []color.NRGBA{
	{0xe0, 0x6c, 0x6f, 0xff}, {0x4f, 0xa6, 0xe6, 0xff}, {0x69, 0xc2, 0x6b, 0xff},
	{0xe2, 0xa8, 0x42, 0xff}, {0x9a, 0x7d, 0xe8, 0xff}, {0x40, 0xc4, 0xb4, 0xff},
	{0xe8, 0x84, 0xc4, 0xff}, {0xc9, 0xa5, 0x5b, 0xff},
}

const (
	homeTabBarHeight   = 46
	homeSoftkeyHeight  = 44
	homeRowHeight      = 44
	homeIconSize       = 26
	homeMaxFolderChips = 3
)

func homeBackgroundImage() *euiimage.NineSlice {
	return euiimage.NewNineSliceColor(homeColorBg)
}

// homeRow is one launcher list entry, with its 1-based position and star state.
type homeRow struct {
	number   int
	path     string
	name     string
	favorite bool
}

// syncHomeSurface shows or hides the launcher and, when shown, positions it to
// the guest viewport and rebuilds its content when anything visible changes.
func (u *shellUI) syncHomeSurface(shell *Shell) {
	if u.homeContainer == nil {
		return
	}
	if !shell.showHomeSurface() {
		u.homeContainer.GetWidget().SetVisibility(widget.Visibility_Hide)
		u.homeSignature = ""
		u.homeScroll = nil
		u.homeRowPaths = nil
		u.homeRowContainers = nil
		return
	}
	u.homeContainer.GetWidget().SetVisibility(widget.Visibility_Show)
	u.ensureHomeChrome(shell)

	rect := shell.guestViewportRect(u.viewportWidth, u.viewportHeight)
	u.homeContainer.GetWidget().LayoutData = widget.AnchorLayoutData{
		StretchHorizontal: true,
		StretchVertical:   true,
		Padding: &widget.Insets{
			Left:   max(0, rect.Min.X),
			Top:    max(0, rect.Min.Y),
			Right:  max(0, u.viewportWidth-rect.Max.X),
			Bottom: max(0, u.viewportHeight-rect.Max.Y),
		},
	}

	tab := shell.homeTab
	if tab == "" {
		tab = homeTabRecent
	}
	entries := shell.homeTabEntries(tab)
	folders := shell.homeLibraryFolders()
	signature := homeSignature(shell, tab, rect, entries, folders)
	if signature == u.homeSignature {
		return
	}
	u.homeSignature = signature
	u.rebuildHomeContent(shell, tab, entries, folders, rect)
}

// rebuildHomeContent replaces the launcher widgets inside homeBody, the
// rebuilt sub-container below the persistent search field (see
// ensureHomeChrome in ui_home_search.go) — so typing in the search box never
// tears down and recreates its own IME session.
func (u *shellUI) rebuildHomeContent(
	shell *Shell,
	tab string,
	entries []LibraryEntry,
	folders []string,
	rect image.Rectangle,
) {
	u.homeBody.RemoveChildren()
	u.homeRowContainers = make(map[string]*widget.Container)
	u.homeRowPaths = u.homeRowPaths[:0]

	rows := make([]homeRow, 0, len(entries))
	for index, entry := range entries {
		rows = append(rows, homeRow{
			number:   index + 1,
			path:     entry.Path,
			name:     entry.Name,
			favorite: shell.settings.isFavorite(entry.Path),
		})
	}

	installed := tab == homeTabInstalled
	contentTop := homeTabBarHeight + 6
	if installed {
		contentTop = homeTabBarHeight + homeSoftkeyHeight
	}

	selectedPath := u.homeSelectedPath
	if !homeContainsPath(rows, selectedPath) {
		selectedPath = ""
		if len(rows) > 0 {
			selectedPath = rows[0].path
		}
	}
	u.homeSelectedPath = selectedPath

	u.homeBody.AddChild(u.homeTabBar(shell, tab))
	u.homeBody.AddChild(homeDivider(homeTabBarHeight))
	if installed {
		u.homeBody.AddChild(u.homeFolderBar(shell, folders))
	}

	u.homeBody.AddChild(u.homeRowScroll(shell, rows, contentTop, selectedPath))

	if len(rows) == 0 {
		u.homeBody.AddChild(homeText(
			homeEmptyMessage(shell, tab, folders, shell.libraryScanning),
			u.design.Type.Body, homeColorMuted,
			widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			},
			max(240, rect.Dx()-64),
		))
	}

	u.homeBody.AddChild(u.homeSoftkeyBar(shell, selectedPath))
}

// homeTabBar builds the green-underlined tab strip.
func (u *shellUI) homeTabBar(shell *Shell, active string) *widget.Container {
	bar := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(28),
			widget.RowLayoutOpts.Padding(&widget.Insets{Left: 22, Top: 6}),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
		})),
	)
	labels := map[string]string{
		homeTabRecent:    shell.tr("Recent"),
		homeTabInstalled: shell.tr("Installed"),
		homeTabFavorites: shell.tr("Favorites"),
	}
	for _, tab := range homeTabs() {
		bar.AddChild(u.homeTabCell(shell, tab, labels[tab], tab == active))
	}
	return bar
}

// homeTabCell is one tab: a label over a green underline shown only when active.
func (u *shellUI) homeTabCell(shell *Shell, tab, label string, active bool) *widget.Container {
	textColor := homeColorTabIdle
	underline := homeColorTransparent
	if active {
		textColor = homeColorTabActive
		underline = homeColorTabActive
	}
	tabName := tab
	cell := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(homeColorTransparent)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(6),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MouseButtonReleasedHandler(func(args *widget.WidgetMouseButtonReleasedEventArgs) {
				if args.Inside {
					shell.setHomeTab(tabName)
				}
			}),
		),
	)
	cell.AddChild(homeText(label, u.design.Type.Strong, textColor,
		widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}, 0))
	cell.AddChild(widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(underline)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(48, 3),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
		),
	))
	return cell
}

// homeFolderBar builds the Installed-tab controls: add a scan root plus a
// removable chip per configured root.
func (u *shellUI) homeFolderBar(shell *Shell, folders []string) *widget.Container {
	bar := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(10),
			widget.RowLayoutOpts.Padding(&widget.Insets{Left: 22, Top: homeTabBarHeight + 8}),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
		})),
	)
	bar.AddChild(homeFlatButton(u, "+ "+shell.tr("Add folder"), homeColorTabActive, func() {
		shell.chooseLibraryFolder()
	}))
	for shown, folder := range folders {
		if shown >= homeMaxFolderChips {
			bar.AddChild(homeText(shell.trf("+%d", len(folders)-homeMaxFolderChips),
				u.design.Type.Caption, homeColorMuted,
				widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}, 0))
			break
		}
		target := folder
		bar.AddChild(homeFlatButton(u, "x "+shorten(filepath.Base(folder), 16), homeColorMuted, func() {
			shell.removeLibraryFolderPath(target)
		}))
	}
	return bar
}

// homeRowScroll builds the scrollable numbered title list.
func (u *shellUI) homeRowScroll(shell *Shell, rows []homeRow, top int, selectedPath string) *widget.ScrollContainer {
	content := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
		)),
	)
	for _, row := range rows {
		content.AddChild(u.homeRowWidget(shell, row, row.path == selectedPath))
		u.homeRowPaths = append(u.homeRowPaths, row.path)
	}
	var scroll *widget.ScrollContainer
	scroll = widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(content),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(&widget.ScrollContainerImage{
			Idle: euiimage.NewNineSliceColor(homeColorTransparent),
			Mask: euiimage.NewNineSliceColor(color.NRGBA{0xff, 0xff, 0xff, 0xff}),
		}),
		widget.ScrollContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				StretchHorizontal: true,
				StretchVertical:   true,
				Padding:           &widget.Insets{Top: top, Bottom: homeSoftkeyHeight},
			}),
			widget.WidgetOpts.ScrolledHandler(func(args *widget.WidgetScrolledEventArgs) {
				scrollContainerByWheel(scroll, args.Y)
			}),
		),
	)
	u.homeScroll = scroll
	return scroll
}

// homeRowWidget is one clickable title row: number, colored tile, name, star.
func (u *shellUI) homeRowWidget(shell *Shell, row homeRow, selected bool) *widget.Container {
	background := euiimage.NewNineSliceColor(homeColorTransparent)
	if selected {
		background = euiimage.NewNineSliceColor(homeColorRowSelect)
	}
	path := row.path
	container := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(background),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, homeRowHeight),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Stretch: true}),
			widget.WidgetOpts.MouseButtonReleasedHandler(func(args *widget.WidgetMouseButtonReleasedEventArgs) {
				if args.Inside {
					u.onHomeRowClicked(shell, path)
				}
			}),
		),
	)
	u.homeRowContainers[path] = container

	left := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(12),
			widget.RowLayoutOpts.Padding(&widget.Insets{Left: 22}),
		)),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionCenter,
		})),
	)
	number := homeText(fmt.Sprintf("%02d", row.number), u.design.Type.Strong, homeColorNumber,
		widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}, 0)
	number.GetWidget().MinWidth = 26
	left.AddChild(number)
	left.AddChild(u.homeIconWidget(shell, row.path, row.name))
	left.AddChild(homeText(shorten(row.name, 48), u.design.Type.Body, homeColorName,
		widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}, 0))
	container.AddChild(left)

	if row.favorite {
		// A small gold tile marks a favorite; the pixel fonts have no star
		// glyph, so a drawn swatch reads better than tofu.
		container.AddChild(widget.NewContainer(
			widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(homeColorStar)),
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(12, 12),
				widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
					HorizontalPosition: widget.AnchorLayoutPositionEnd,
					VerticalPosition:   widget.AnchorLayoutPositionCenter,
					Padding:            &widget.Insets{Right: 22},
				}),
			),
		))
	}
	return container
}

// homeSoftkeyBar is the bottom bar: Favorite (left) and Open (right).
func (u *shellUI) homeSoftkeyBar(shell *Shell, selectedPath string) *widget.Container {
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(homeColorSoftbar)),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, homeSoftkeyHeight),
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionEnd,
				StretchHorizontal:  true,
			}),
		),
	)
	fav := homeFlatButton(u, shell.tr("Favorite"), homeColorStar, func() {
		shell.toggleFavoritePath(u.homeSelectedPath)
	})
	fav.GetWidget().Disabled = selectedPath == ""
	fav.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionStart,
		VerticalPosition:   widget.AnchorLayoutPositionCenter,
		Padding:            &widget.Insets{Left: 18},
	}
	bar.AddChild(fav)
	u.homeFavButton = fav

	open := homeFlatButton(u, shell.tr("Open"), homeColorTabActive, func() {
		shell.homeOpenPath(u.homeSelectedPath)
	})
	open.GetWidget().Disabled = selectedPath == ""
	open.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionCenter,
		Padding:            &widget.Insets{Right: 18},
	}
	bar.AddChild(open)
	u.homeOpenButton = open
	return bar
}

// homeText is a plain colored label with an optional wrap width and layout data.
func homeText(label string, face *text.Face, textColor color.NRGBA, layoutData any, maxWidth int) *widget.Text {
	options := []widget.TextOpt{widget.TextOpts.Text(label, face, textColor)}
	if maxWidth > 0 {
		options = append(options, widget.TextOpts.MaxWidth(float64(maxWidth)))
	}
	if layoutData != nil {
		options = append(options, widget.TextOpts.WidgetOpts(widget.WidgetOpts.LayoutData(layoutData)))
	}
	return widget.NewText(options...)
}

// homeFlatButton is a borderless, colored-text tappable label for the soft-key
// bar and folder controls.
func homeFlatButton(u *shellUI, label string, textColor color.NRGBA, clicked func()) *widget.Button {
	transparent := euiimage.NewNineSliceColor(homeColorTransparent)
	return widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    transparent,
			Hover:   euiimage.NewNineSliceColor(color.NRGBA{0xff, 0xff, 0xff, 0x14}),
			Pressed: euiimage.NewNineSliceColor(color.NRGBA{0xff, 0xff, 0xff, 0x24}),
		}),
		widget.ButtonOpts.Text(label, u.design.Type.Strong, &widget.ButtonTextColor{
			Idle:     textColor,
			Disabled: homeColorMuted,
		}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 12, Right: 12, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(*widget.ButtonClickedEventArgs) {
			if clicked != nil {
				clicked()
			}
		}),
	)
}

func homeDivider(top int) *widget.Container {
	return widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(euiimage.NewNineSliceColor(homeColorDivider)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, 1),
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
				Padding:            &widget.Insets{Top: top},
			}),
		),
	)
}

func homeIconColor(path string) color.NRGBA {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(path))
	return homeIconPalette[hasher.Sum32()%uint32(len(homeIconPalette))]
}

// homeIconWidget returns the row's icon tile: the extracted game icon when the
// backend has provided one, otherwise a monogram placeholder (see
// homeIconPlaceholder). Requesting the icon here means only visible rows are
// fetched.
func (u *shellUI) homeIconWidget(shell *Shell, path, name string) widget.PreferredSizeLocateableWidget {
	shell.requestHomeIcon(path)
	icon := homeIconPlaceholder(u.design, path, name)
	if loaded := shell.homeIcon(path); loaded != nil {
		icon = scaleIconToTile(loaded)
	}
	return widget.NewGraphic(
		widget.GraphicOpts.Image(icon),
		widget.GraphicOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(homeIconSize, homeIconSize),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
		),
	)
}

// homeIconPlaceholder builds a stand-in tile for a title with no extracted
// icon (not fetched yet, no backend, or the format carries none — e.g. a
// ktf-wipi title whose resource layout defeats the icon heuristic): a
// hash-colored square bearing the title's first letter, so the tile reads as
// an intentional avatar rather than a blank, possibly-broken swatch. Shared
// with the Open Recent dialog (ui_recent.go) so a title looks the same
// wherever it is listed.
func homeIconPlaceholder(design *ARAMDesignSystem, path, name string) *ebiten.Image {
	tile := ebiten.NewImage(homeIconSize, homeIconSize)
	tile.Fill(homeIconColor(path))
	letter := monogramLetter(name)
	if letter == "" {
		return tile
	}
	face := design.Type.Strong
	bounds := tile.Bounds()
	top := centeredTextTop(face, bounds, 0)
	drawCenteredText(tile, letter, face, color.NRGBA{0xff, 0xff, 0xff, 0xff}, bounds, top)
	return tile
}

// monogramLetter is the uppercased first rune of name, or "" for a blank name.
func monogramLetter(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(string([]rune(trimmed)[0]))
}

// scaleIconToTile renders icon at the launcher tile size.
func scaleIconToTile(icon *ebiten.Image) *ebiten.Image {
	bounds := icon.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 ||
		(bounds.Dx() == homeIconSize && bounds.Dy() == homeIconSize) {
		return icon
	}
	tile := ebiten.NewImage(homeIconSize, homeIconSize)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Scale(
		float64(homeIconSize)/float64(bounds.Dx()),
		float64(homeIconSize)/float64(bounds.Dy()),
	)
	tile.DrawImage(icon, options)
	return tile
}

func homeContainsPath(rows []homeRow, path string) bool {
	if path == "" {
		return false
	}
	for _, row := range rows {
		if row.path == path {
			return true
		}
	}
	return false
}

// homeEmptyMessage explains an empty tab, or an active search filter that
// matched nothing (checked first: an empty Installed tab with a filter typed
// should explain the filter, not tell the user to add a library folder).
func homeEmptyMessage(shell *Shell, tab string, folders []string, scanning bool) string {
	if query := strings.TrimSpace(shell.homeFilterQuery); query != "" {
		return shell.trf("No titles match “%s”.", query)
	}
	switch tab {
	case homeTabInstalled:
		if scanning {
			return shell.tr("Scanning library…")
		}
		if len(folders) == 0 {
			return shell.tr("No library folder yet. Use “Add folder” to choose where your games live.")
		}
		return shell.tr("No installed games found under the library folders.")
	case homeTabFavorites:
		return shell.tr("No favorites yet. Star a title to keep it here.")
	default:
		return shell.tr("No recent titles yet. Open a title from File or the Installed tab.")
	}
}

// homeSignature encodes everything the surface renders so a rebuild happens only
// when something visible changes.
func homeSignature(shell *Shell, tab string, rect image.Rectangle, entries []LibraryEntry, folders []string) string {
	builder := make([]byte, 0, 128)
	builder = fmt.Appendf(builder, "home|%dx%d|%s|scan=%t|", rect.Dx(), rect.Dy(), tab, shell.libraryScanning)
	for _, folder := range folders {
		builder = append(builder, folder...)
		builder = append(builder, 0x1f)
	}
	builder = append(builder, '|')
	for _, entry := range entries {
		builder = append(builder, entry.Path...)
		if shell.settings.isFavorite(entry.Path) {
			builder = append(builder, 0x02)
		}
		if shell.homeIcon(entry.Path) != nil {
			builder = append(builder, 0x03)
		}
		builder = append(builder, 0x00)
	}
	return string(builder)
}
