package frontend

func (s *Shell) openIssueReportHistory() {
	reports := s.settings.IssueReports
	if len(reports) == 0 {
		s.panel = &Panel{
			Kind:  "issue-report",
			Title: "Submitted Reports",
			Lines: []string{
				"No submitted reports are saved on this device.",
				"Reports created through the relay will appear here.",
			},
			Actions: []ToolAction{{
				ID:      issueReportNewAction,
				Label:   "New Report",
				Enabled: true,
			}},
		}
		s.setStatus(s.tr("No submitted reports are saved on this device."))
		return
	}

	options := make([]ToolFieldOption, 0, len(reports))
	for _, record := range reports {
		options = append(options, ToolFieldOption{
			Value: record.ReportID,
			Label: issueReportHistoryLabel(record),
		})
	}
	s.panel = &Panel{
		Kind:  "issue-report",
		Title: "Submitted Reports",
		Lines: []string{
			"Choose a submitted report to reopen it or add a comment.",
			"Comment access is stored only on this device.",
		},
		Fields: []ToolField{{
			ID:      issueReportHistoryField,
			Label:   "Submitted report",
			Value:   reports[0].ReportID,
			Options: options,
		}},
		Actions: []ToolAction{
			{
				ID:      issueReportHistoryView,
				Label:   "View Report",
				Enabled: true,
			},
			{
				ID:      issueReportNewAction,
				Label:   "New Report",
				Enabled: true,
			},
			{
				ID:      issueReportHistoryForget,
				Label:   "Forget Report",
				Enabled: true,
			},
		},
		FieldValues: map[string]string{
			issueReportHistoryField: reports[0].ReportID,
		},
	}
	s.setStatus(s.tr("Submitted report history opened"))
}

func (s *Shell) openSavedIssueReport(reportID string) {
	record, ok := s.settings.issueReport(reportID)
	if !ok {
		s.setStatus(s.tr("Report history: selected report was not found"))
		s.openIssueReportHistory()
		return
	}
	s.showIssueReportRecord(record, "", "", nil)
	s.setStatus(s.tr("Submitted report opened"))
}

func (s *Shell) forgetSavedIssueReport(reportID string) {
	previous := append([]IssueReportRecord(nil), s.settings.IssueReports...)
	if !s.settings.forgetIssueReport(reportID) {
		s.setStatus(s.tr("Report history: selected report was not found"))
		return
	}
	if err := s.settings.save(); err != nil {
		s.settings.IssueReports = previous
		s.setStatus(s.tr("Report history: ") + err.Error())
		return
	}
	s.openIssueReportHistory()
	s.setStatus(s.tr("Submitted report removed from this device"))
}

func (s *Shell) showIssueReportRecord(
	record IssueReportRecord,
	bundlePath string,
	warning string,
	historyErr error,
) {
	lines := []string{
		"Issue created with the debug bundle.",
		s.trf("GitHub issue: %s", record.IssueURL),
		s.trf(
			"Submitted: %s",
			record.CreatedAt.Local().Format("2006-01-02 15:04 MST"),
		),
		s.trf("Repository: %s", record.Repository),
		s.trf("Situation: %s", record.Situation),
	}
	if warning != "" {
		lines = append(lines, s.trf("Collection warning: %s", warning))
	}
	if historyErr != nil {
		lines = append(
			lines,
			s.trf("Could not save this report to history: %s", historyErr),
		)
	} else {
		lines = append(
			lines,
			"This report is saved under Help > Submitted Reports.",
		)
	}
	actions := []ToolAction{
		{
			ID:      issueReportOpenIssueAction,
			Label:   "Open GitHub Issue",
			Enabled: true,
		},
		{
			ID:      issueReportCommentAction,
			Label:   "Add Comment",
			Enabled: true,
		},
	}
	if bundlePath != "" {
		actions = append(actions, ToolAction{
			ID:      issueReportFolderAction,
			Label:   "Open Bundle Folder",
			Enabled: true,
		})
	}
	if bundlePath == "" {
		actions = append(actions, ToolAction{
			ID:      issueReportHistoryAction,
			Label:   "Submitted Reports",
			Enabled: true,
		})
	}

	s.panel = &Panel{
		Kind:  "issue-report",
		Title: "Submitted Report",
		Lines: lines,
		Fields: []ToolField{{
			ID:          issueReportCommentField,
			Label:       "Follow-up comment",
			Placeholder: "Add more details to the created issue",
		}},
		Actions: actions,
		FieldValues: map[string]string{
			issueReportCommentField:    "",
			issueReportPathField:       bundlePath,
			issueReportIssueURLField:   record.IssueURL,
			issueReportIDField:         record.ReportID,
			issueReportCapabilityField: record.Capability,
		},
	}
}

func (record IssueReportRecord) relayReport() issueRelayReport {
	return issueRelayReport{
		ReportID:   record.ReportID,
		IssueURL:   record.IssueURL,
		Capability: record.Capability,
	}
}
