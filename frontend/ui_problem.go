package frontend

import (
	"fmt"
	"strings"

	euiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
)

// Problems are a scrollable surface so the reason and recovery actions remain
// available on narrow screens, including a fault with a frozen guest frame.
func (u *shellUI) syncProblemSurface(shell *Shell) {
	if u.problemContainer == nil {
		return
	}
	if shell.problem == nil || shell.loading {
		u.problemContainer.GetWidget().SetVisibility(widget.Visibility_Hide)
		u.problemSignature = ""
		u.problemExpanded = false
		u.problemScroll = nil
		return
	}
	problem := *shell.problem
	if problem != u.problemIdentity {
		u.problemIdentity = problem
		u.problemExpanded = false
	}
	container := u.problemContainer
	container.GetWidget().SetVisibility(widget.Visibility_Show)
	area := shell.guestViewportRect(u.viewportWidth, u.viewportHeight)
	container.GetWidget().LayoutData = widget.AnchorLayoutData{
		StretchHorizontal: true, StretchVertical: true,
		Padding: &widget.Insets{Left: area.Min.X, Top: area.Min.Y,
			Right: u.viewportWidth - area.Max.X, Bottom: u.viewportHeight - area.Max.Y},
	}
	signature := fmt.Sprintf("%v|%s|%v|%t", problem, shell.language(), area, u.problemExpanded)
	if signature == u.problemSignature {
		return
	}
	u.problemSignature = signature
	container.RemoveChildren()
	design := u.design
	container.SetBackgroundImage(euiimage.NewNineSliceColor(design.Palette.GuestSurface))
	width := max(1, area.Dx()-2*design.Space.L)
	content := widget.NewContainer(widget.ContainerOpts.Layout(widget.NewRowLayout(
		widget.RowLayoutOpts.Direction(widget.DirectionVertical),
		widget.RowLayoutOpts.Spacing(design.Space.M),
		widget.RowLayoutOpts.Padding(&widget.Insets{Top: design.Space.L, Bottom: design.Space.L}),
	)))
	addText := func(label string, strong bool) {
		face := design.Type.Body
		if strong {
			face = design.Type.Strong
		}
		content.AddChild(homeText(label, face, design.Palette.GuestInk,
			widget.RowLayoutData{Stretch: true}, width))
	}
	title, guidance := problemPresentation(shell)
	addText(title, true)
	addText(strings.TrimSpace(problem.Reason), false)
	addText(guidance, false)
	addButton := func(id, label string, primary bool, action func()) {
		style := design.Components.SubtleButton
		if primary {
			style = design.Components.PrimaryButton
		}
		button := design.button(shell.tr(label), style, design.Type.Strong,
			width, max(design.px(44), style.MinHeight), widget.TextPositionCenter, action)
		button.GetWidget().CustomData = id
		button.GetWidget().LayoutData = widget.RowLayoutData{Stretch: true}
		content.AddChild(button)
	}
	addButton("file.open", "Open another file", true, shell.chooseFile)
	addButton("help.issue", "Report Issue", false, shell.openIssueTrackerForProblem)
	detailsLabel := "Show technical details"
	if u.problemExpanded {
		detailsLabel = "Hide technical details"
	}
	addButton("problem.details", detailsLabel, false, func() { u.problemExpanded = !u.problemExpanded })
	if u.problemExpanded {
		for _, detail := range []string{
			shell.trf("Input: %s", problem.Input),
			shell.trf("State: %s", shell.tr(stateValueLabel(string(problem.State)))),
			shell.trf("Format: %s", emptyFallback(problem.Format, shell.tr("unknown"))),
			shell.trf("Profile: %s", emptyFallback(problem.Profile, shell.tr("unselected"))),
			shell.trf("Backend: %s", emptyFallback(problem.Backend, shell.tr("unknown"))),
		} {
			addText(detail, false)
		}
	}
	scroll := widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(content),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(design.Components.Scroll),
		widget.ScrollContainerOpts.WidgetOpts(widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
			StretchHorizontal: true, StretchVertical: true,
			Padding: &widget.Insets{Left: design.Space.L, Right: design.Space.L},
		})),
	)
	scroll.GetWidget().ScrolledEvent.AddHandler(func(args any) {
		scrollContainerByWheel(scroll, args.(*widget.WidgetScrolledEventArgs).Y)
	})
	container.AddChild(scroll)
	u.problemScroll = scroll
}

func problemPresentation(shell *Shell) (string, string) {
	switch shell.problem.State {
	case FrontendMalformedInput:
		return shell.tr("This file could not be read"), shell.tr("Choose another game file. If this file should work, report the problem.")
	case FrontendUnsupportedProfile:
		return shell.tr("This game profile is not supported"), shell.tr("Try another game, or report this file so support can be investigated.")
	case FrontendBackendUnavailable:
		return shell.tr("The emulator could not start"), shell.tr("Check the details below, or report the problem with your ARAM version.")
	case FrontendGuestFaulted:
		return shell.tr("The game stopped unexpectedly"), shell.tr("Open another game, or report what happened so the problem can be investigated.")
	default:
		return shell.tr("Unable to start this input"), shell.tr("Choose another game file. If this file should work, report the problem.")
	}
}

func (s *Shell) openIssueTrackerForProblem() {
	if s.problem == nil {
		s.openIssueTracker()
		return
	}
	s.openIssueTrackerWithSituation(s.trf("Problem opening %s: %s", s.problem.Input, s.problem.Reason))
}
