package frontend

// Native dialog copy for a guest fault. faultReportDialogBody and
// faultReportSituation each carry one %s for the fault reason.
const (
	faultReportDialogTitle = "ARAM stopped"
	faultReportDialogBody  = "The emulated machine faulted and the title stopped.\n\n" +
		"Reason: %s\n\n" +
		"Open the issue report form so this can be fixed? A redacted debug " +
		"bundle (host file paths removed) is attached to a public GitHub issue."
	faultReportSituation = "Emulator fault: %s"
)

// promptFaultReport raises a native dialog after a guest fault freezes the
// running title, offering to open the pre-filled issue report form. Unlike the
// crash path this fires while the app is still alive, so the blocking dialog is
// shown off the update goroutine and the still-live ebiten window keeps painting
// behind it. A Yes answer is routed back through faultReportRequests so the
// report panel is opened on the update goroutine that owns it.
//
// It is a no-op unless a native prompter has been wired, which the desktop
// bootstrap does and mobile, web, and tests do not: there the fault stays
// visible only through the on-screen problem panel, exactly as before.
func (s *Shell) promptFaultReport(problem *FrontendProblem) {
	if s.faultPrompter == nil || s.faultReportRequests == nil || problem == nil {
		return
	}
	// One dialog per loaded title. A fault stops the frame worker, so the same
	// generation cannot fault a second time, and a reload bumps the generation
	// and re-arms the prompt.
	if s.faultPrompted && s.faultPromptGeneration == s.frameGeneration {
		return
	}
	s.faultPrompted = true
	s.faultPromptGeneration = s.frameGeneration

	prompter := s.faultPrompter
	requests := s.faultReportRequests
	reason := problem.Reason
	title := s.tr(faultReportDialogTitle)
	body := s.trf(faultReportDialogBody, reason)
	go func() {
		confirmed, available := prompter.confirmReport(title, body)
		if !available || !confirmed {
			return
		}
		// Buffered depth one; drop if a prior request is still queued so a
		// double fault can never block this goroutine on the dialog thread.
		select {
		case requests <- reason:
		default:
		}
	}()
}

// consumeFaultReportRequest runs on the update goroutine when the user accepts
// the native fault dialog. It opens the ordinary issue report form so the report
// is reviewed and submitted exactly like a manual one.
func (s *Shell) consumeFaultReportRequest(reason string) {
	s.openIssueTrackerForFault(reason)
}

// openIssueTrackerForFault opens the issue report panel with the fault reason
// pre-filled into the description, so the user only has to review and submit.
func (s *Shell) openIssueTrackerForFault(reason string) {
	s.openIssueTracker()
	if s.panel == nil {
		return
	}
	situation := s.trf(faultReportSituation, reason)
	if s.panel.FieldValues == nil {
		s.panel.FieldValues = make(map[string]string)
	}
	s.panel.FieldValues["situation"] = situation
	for index := range s.panel.Fields {
		if s.panel.Fields[index].ID == "situation" {
			s.panel.Fields[index].Value = situation
			break
		}
	}
	// Force the UI to rebuild the panel so the text input re-seeds from the
	// pre-filled value instead of the empty one openIssueTracker installed.
	if s.interfaceUI != nil {
		s.interfaceUI.panelSignature = ""
	}
}
