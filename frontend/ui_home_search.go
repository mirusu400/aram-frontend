package frontend

import "github.com/ebitenui/ebitenui/widget"

// Home launcher search field. It lives outside rebuildHomeContent's
// teardown-and-rebuild cycle (ui_home.go): homeContainer holds exactly two
// persistent children — this search field, created once, and homeBody, the
// sub-container rebuildHomeContent freely tears down and repopulates. Typing
// a character narrows the list (shell.setHomeFilter → homeTabEntries), which
// changes homeSignature and triggers a rebuild of homeBody only, so the
// field's own IME session and focus are never disturbed by it.

// homeSearchBarHeight is the fixed footprint of the search field, including
// its own top/bottom padding; homeBody's top padding is offset by exactly
// this much so the two never overlap.
const homeSearchBarHeight = 40

// ensureHomeChrome lazily builds the persistent search field and the homeBody
// wrapper the first time Home is shown, and is a no-op afterward.
func (u *shellUI) ensureHomeChrome(shell *Shell) {
	if u.homeBody != nil {
		return
	}
	design := u.design
	u.homeSearchInput = newIMETextInput(design, imeTextInputConfig{
		Placeholder: shell.tr("Search titles"),
		Text:        shell.homeFilterQuery,
		MinHeight:   homeSearchBarHeight - 12,
		LayoutData: widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			Padding: &widget.Insets{
				Left: 22, Right: 22, Top: 6, Bottom: 6,
			},
		},
		Changed: func(value string) {
			shell.setHomeFilter(value)
		},
	})
	u.homeContainer.AddChild(u.homeSearchInput)

	u.homeBody = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			StretchVertical:    true,
			Padding:            &widget.Insets{Top: homeSearchBarHeight},
		})),
	)
	u.homeContainer.AddChild(u.homeBody)
}
