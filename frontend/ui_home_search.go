package frontend

import "github.com/ebitenui/ebitenui/widget"

// Home launcher search row (field + clear button). It lives outside
// rebuildHomeContent's teardown-and-rebuild cycle (ui_home.go): homeContainer
// holds exactly two persistent children — this row, created once, and
// homeBody, the sub-container rebuildHomeContent freely tears down and
// repopulates. Typing a character narrows the list (shell.setHomeFilter →
// homeTabEntries), which changes homeSignature and triggers a rebuild of
// homeBody only, so the field's own IME session and focus are never disturbed
// by it. Ctrl+F / "/" focus the field without a mouse (see handleShortcuts
// in shell.go and focusHomeSearch in shell_library.go).

// homeSearchBarHeight is the fixed footprint of the search field, including
// its own top/bottom padding; homeBody's top padding is offset by exactly
// this much so the two never overlap.
const homeSearchBarHeight = 40

// homeSearchFieldHeight is the search field's own height, before the
// homeSearchBarHeight row's top/bottom padding.
const homeSearchFieldHeight = homeSearchBarHeight - 12

// ensureHomeChrome lazily builds the persistent search row (field + clear
// button) and the homeBody wrapper the first time Home is shown, and is a
// no-op afterward.
func (u *shellUI) ensureHomeChrome(shell *Shell) {
	if u.homeBody != nil {
		return
	}
	design := u.design
	// RowLayoutData.Stretch only stretches the cross axis (ebitenui), which
	// for a horizontal row is height, not width — so the field and button are
	// each positioned directly in homeContainer's AnchorLayout instead: the
	// field stretches horizontally with its right edge held clear of the
	// button's reserved width, and the button anchors to that same edge.
	u.homeSearchInput = newIMETextInput(design, imeTextInputConfig{
		Placeholder: shell.tr("Search titles"),
		Text:        shell.homeFilterQuery,
		MinHeight:   homeSearchFieldHeight,
		LayoutData: widget.AnchorLayoutData{
			HorizontalPosition: widget.AnchorLayoutPositionStart,
			VerticalPosition:   widget.AnchorLayoutPositionStart,
			StretchHorizontal:  true,
			Padding: &widget.Insets{
				Left:  22,
				Right: 22 + homeSearchFieldHeight + design.Space.XS,
				Top:   6,
			},
		},
		Changed: func(value string) {
			shell.setHomeFilter(value)
		},
	})
	u.homeContainer.AddChild(u.homeSearchInput)

	clearButton := design.button(
		"×",
		design.Components.SubtleButton,
		design.Type.Strong,
		homeSearchFieldHeight,
		homeSearchFieldHeight,
		widget.TextPositionCenter,
		func() { u.clearHomeSearch(shell) },
	)
	clearButton.GetWidget().LayoutData = widget.AnchorLayoutData{
		HorizontalPosition: widget.AnchorLayoutPositionEnd,
		VerticalPosition:   widget.AnchorLayoutPositionStart,
		Padding:            &widget.Insets{Right: 22, Top: 6},
	}
	u.homeContainer.AddChild(clearButton)

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

// clearHomeSearch empties the search field and its filter. It is the search
// row's clear (×) button action, factored out so it is testable without a
// real button click.
func (u *shellUI) clearHomeSearch(shell *Shell) {
	if u.homeSearchInput != nil {
		// SetText alone would not notify Changed (it treats the value as
		// already current), so the filter is cleared explicitly too.
		u.homeSearchInput.SetText("")
	}
	shell.setHomeFilter("")
}
