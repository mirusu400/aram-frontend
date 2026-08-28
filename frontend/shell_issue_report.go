package frontend

import (
	"context"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type issueReportResult struct {
	draft          issueReportDraft
	input          *InputInfo
	backend        string
	state          FrontendState
	path           string
	warning        string
	report         issueRelayReport
	relayErr       error
	createdAt      time.Time
	idempotencyKey string
	err            error
}

type issueCommentResult struct {
	report         issueRelayReport
	commentURL     string
	idempotencyKey string
	err            error
}

var openExternalURL = openPlatformURL

func (s *Shell) openIssueTracker() {
	gameTitle := ""
	if s.input != nil {
		gameTitle = s.input.DisplayName
	}
	s.panel = &Panel{
		Kind:  "issue-report",
		Title: "Report Issue",
		Lines: []string{
			"Describe the problem. ARAM will upload a redacted debug bundle and an optional screenshot.",
			"Submit Report creates a public GitHub issue. Do not include personal or proprietary data.",
		},
		Fields: []ToolField{
			{
				ID:          "situation",
				Label:       "What happened?",
				Placeholder: "Describe the problem and what you expected",
			},
			{
				ID:          "game_title",
				Label:       "Game title",
				Value:       gameTitle,
				Placeholder: "Title shown in ARAM",
			},
			{
				ID:          "carrier",
				Label:       "Carrier",
				Placeholder: "SKT, KTF, LGT, or unknown",
			},
			{
				ID:    "repository",
				Label: "Expected repository",
				Value: "aram-core",
				Options: []ToolFieldOption{
					{Value: "aram-core", Label: "ARAM Core"},
					{Value: "aram-emu", Label: "ARAM Emulator"},
					{Value: "aram-frontend", Label: "ARAM Frontend"},
				},
			},
			{
				ID:       issueReportScreenshotField,
				Label:    "Send screenshot",
				Value:    "true",
				Checkbox: true,
			},
		},
		Actions: []ToolAction{
			{
				ID:      issueReportPrepareAction,
				Label:   "Submit Report",
				Enabled: true,
			},
		},
		FieldValues: map[string]string{
			"situation":                "",
			"game_title":               gameTitle,
			"carrier":                  "",
			"repository":               "aram-core",
			issueReportScreenshotField: "true",
		},
	}
	if len(s.settings.IssueReports) > 0 {
		s.panel.Actions = append(s.panel.Actions, ToolAction{
			ID:      issueReportHistoryAction,
			Label:   "Submitted Reports",
			Enabled: true,
		})
	}
}

func (s *Shell) executeIssueReportAction(
	action string,
	fields map[string]string,
) {
	if s.panel == nil ||
		s.panel.Kind != "issue-report" ||
		s.panel.Busy {
		return
	}
	switch action {
	case issueReportFolderAction:
		s.openPreparedReportFolder()
	case issueReportDraftAction:
		s.reopenPreparedIssueDraft()
	case issueReportOpenIssueAction:
		s.openCreatedIssue()
	case issueReportCommentAction:
		s.addIssueComment(fields)
	case issueReportHistoryAction:
		s.openIssueReportHistory()
	case issueReportHistoryView:
		s.openSavedIssueReport(fields[issueReportHistoryField])
	case issueReportHistoryForget:
		s.forgetSavedIssueReport(fields[issueReportHistoryField])
	case issueReportNewAction:
		s.openIssueTracker()
	case issueReportPrepareAction:
		s.prepareIssueReport(fields)
	}
}

func (s *Shell) prepareIssueReport(fields map[string]string) {
	draft, err := newIssueReportDraft(fields)
	if err != nil {
		s.panel.Lines = []string{err.Error()}
		s.setStatus(s.tr("Issue report: ") + s.tr(err.Error()))
		return
	}
	idempotencyKey := strings.TrimSpace(fields[issueReportIdempotencyField])
	if !validIssueUUID(idempotencyKey) {
		idempotencyKey, err = newIssueIdempotencyKey()
		if err != nil {
			s.panel.Lines = []string{err.Error()}
			s.setStatus(s.tr("Issue report: ") + err.Error())
			return
		}
	}

	s.panel.Busy = true
	s.panel.Lines = []string{"Collecting and uploading issue report..."}
	s.panel.Actions = []ToolAction{{
		ID:      issueReportPrepareAction,
		Label:   "Submit Report",
		Enabled: false,
	}}
	s.panel.FieldValues[issueReportIdempotencyField] = idempotencyKey
	s.setStatus(s.tr("Uploading issue report..."))

	createdAt := time.Now().UTC()
	snapshot := s.captureDebugBundleSnapshot(createdAt)
	if !issueReportScreenshotEnabled(fields) {
		snapshot.Screenshot = nil
	}
	backend := s.backend
	relay := s.issueRelay
	if relay == nil {
		relay = newIssueRelayClient()
	}
	go func() {
		path, warning, bundleErr := collectDebugBundle(snapshot, backend)
		result := issueReportResult{
			draft:          draft,
			input:          snapshot.Input,
			backend:        snapshot.Backend,
			state:          snapshot.FrontendState,
			path:           path,
			warning:        warning,
			createdAt:      createdAt,
			idempotencyKey: idempotencyKey,
			err:            bundleErr,
		}
		if bundleErr == nil {
			result.report, result.relayErr = relay.Submit(
				context.Background(),
				issueRelaySubmission{
					Draft:          draft,
					Input:          snapshot.Input,
					Backend:        snapshot.Backend,
					State:          snapshot.FrontendState,
					BundlePath:     path,
					Warning:        warning,
					IdempotencyKey: idempotencyKey,
				},
			)
		}
		s.issueReportResults <- result
	}()
}

func (s *Shell) consumeIssueReportResult(result issueReportResult) {
	if result.err != nil {
		if s.panel != nil && s.panel.Kind == "issue-report" {
			s.panel.Busy = false
			s.panel.Lines = []string{
				s.tr("Issue report could not be prepared:") + " " +
					result.err.Error(),
			}
			s.panel.Actions = []ToolAction{{
				ID:      issueReportPrepareAction,
				Label:   "Retry",
				Enabled: true,
			}}
		}
		s.setStatus(s.tr("Issue report: ") + result.err.Error())
		return
	}
	if result.relayErr == nil {
		s.consumeUploadedIssueReport(result)
		return
	}
	s.consumeIssueReportFallback(result)
}

func (s *Shell) consumeUploadedIssueReport(result issueReportResult) {
	record := IssueReportRecord{
		ReportID:   result.report.ReportID,
		IssueURL:   result.report.IssueURL,
		Capability: result.report.Capability,
		Repository: result.draft.Repository,
		Situation:  result.draft.Situation,
		GameTitle:  result.draft.GameTitle,
		CreatedAt:  result.createdAt,
	}
	s.settings.rememberIssueReport(record)
	historyErr := s.settings.save()
	openErr := openExternalURL(result.report.IssueURL)
	if s.panel != nil && s.panel.Kind == "issue-report" {
		s.showIssueReportRecord(
			record,
			result.path,
			result.warning,
			historyErr,
		)
	}
	if openErr != nil {
		s.setStatus(s.tr("GitHub issue: ") + openErr.Error())
		return
	}
	if historyErr != nil {
		s.setStatus(s.tr("Report history: ") + historyErr.Error())
		return
	}
	s.setStatus(s.tr("Issue created and opened in GitHub"))
}

func (s *Shell) consumeIssueReportFallback(result issueReportResult) {

	draftURL, err := buildIssueDraftURL(
		result.draft,
		result.input,
		result.backend,
		result.state,
		result.path,
		result.warning,
	)
	if err != nil {
		result.err = err
		s.consumeIssueReportResult(result)
		return
	}
	s.appendLog(s.tr("Issue report: ") + result.relayErr.Error())
	openErr := openExternalURL(draftURL)
	folderErr := openArtifactFolder(filepath.Dir(result.path))

	if s.panel != nil && s.panel.Kind == "issue-report" {
		if s.panel.FieldValues == nil {
			s.panel.FieldValues = make(map[string]string)
		}
		s.panel.FieldValues[issueReportDraftURLField] = draftURL
		s.panel.FieldValues[issueReportPathField] = result.path
		s.panel.FieldValues[issueReportIdempotencyField] = result.idempotencyKey
		s.panel.Busy = false
		s.panel.Lines = []string{
			"Automatic upload failed; a manual GitHub draft was opened.",
			s.trf("Upload error: %s", result.relayErr.Error()),
			s.trf(
				"Attach %s to the GitHub draft, review it, then submit.",
				filepath.Base(result.path),
			),
		}
		s.panel.Actions = []ToolAction{
			{
				ID:      issueReportPrepareAction,
				Label:   "Retry Upload",
				Enabled: true,
			},
			{
				ID:      issueReportFolderAction,
				Label:   "Open Bundle Folder",
				Enabled: true,
			},
			{
				ID:      issueReportDraftAction,
				Label:   "Open Draft Again",
				Enabled: true,
			},
		}
	}

	switch {
	case openErr != nil:
		s.setStatus(s.tr("Issue draft: ") + openErr.Error())
	case folderErr != nil:
		s.setStatus(s.tr("Issue report folder: ") + folderErr.Error())
	default:
		s.setStatus(s.tr("Automatic upload failed; use the opened GitHub draft"))
	}
}

func (s *Shell) openPreparedReportFolder() {
	path := s.panel.FieldValues[issueReportPathField]
	if path == "" {
		s.setStatus(s.tr("Issue report folder: no prepared bundle"))
		return
	}
	if err := openArtifactFolder(filepath.Dir(path)); err != nil {
		s.setStatus(s.tr("Issue report folder: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Opened issue report folder"))
}

func (s *Shell) reopenPreparedIssueDraft() {
	draftURL := s.panel.FieldValues[issueReportDraftURLField]
	if draftURL == "" {
		s.setStatus(s.tr("Issue draft: no prepared draft"))
		return
	}
	if err := openExternalURL(draftURL); err != nil {
		s.setStatus(s.tr("Issue draft: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Opened GitHub issue draft"))
}

func (s *Shell) openCreatedIssue() {
	issueURL := s.panel.FieldValues[issueReportIssueURLField]
	if issueURL == "" {
		s.setStatus(s.tr("GitHub issue: no created issue"))
		return
	}
	if err := openExternalURL(issueURL); err != nil {
		s.setStatus(s.tr("GitHub issue: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Opened GitHub issue"))
}

func (s *Shell) addIssueComment(fields map[string]string) {
	comment := strings.TrimSpace(fields[issueReportCommentField])
	if comment == "" {
		s.panel.Lines = []string{"Follow-up comment is required."}
		s.setStatus(s.tr("Follow-up comment is required."))
		return
	}
	if utf8.RuneCountInString(comment) > issueReportCommentMaximumRunes {
		s.panel.Lines = []string{
			"Follow-up comment must be 5000 characters or fewer.",
		}
		s.setStatus(s.tr("Follow-up comment must be 5000 characters or fewer."))
		return
	}
	report := issueRelayReport{
		ReportID:   fields[issueReportIDField],
		IssueURL:   fields[issueReportIssueURLField],
		Capability: fields[issueReportCapabilityField],
	}
	if err := validateIssueRelayReport(report, ""); err != nil {
		s.panel.Lines = []string{"The created issue session is invalid."}
		s.setStatus(s.tr("Issue comment: ") + err.Error())
		return
	}
	idempotencyKey := strings.TrimSpace(fields[issueCommentIdempotencyField])
	var err error
	if !validIssueUUID(idempotencyKey) {
		idempotencyKey, err = newIssueIdempotencyKey()
		if err != nil {
			s.panel.Lines = []string{err.Error()}
			s.setStatus(s.tr("Issue comment: ") + err.Error())
			return
		}
		s.panel.FieldValues[issueCommentIdempotencyField] = idempotencyKey
	}
	s.panel.Busy = true
	s.panel.Lines = []string{"Adding follow-up comment..."}
	s.setStatus(s.tr("Adding follow-up comment..."))
	relay := s.issueRelay
	if relay == nil {
		relay = newIssueRelayClient()
	}
	go func() {
		commentURL, commentErr := relay.AddComment(
			context.Background(),
			report,
			comment,
			idempotencyKey,
		)
		s.issueCommentResults <- issueCommentResult{
			report:         report,
			commentURL:     commentURL,
			idempotencyKey: idempotencyKey,
			err:            commentErr,
		}
	}()
}

func (s *Shell) consumeIssueCommentResult(result issueCommentResult) {
	if s.panel == nil ||
		s.panel.Kind != "issue-report" ||
		s.panel.FieldValues[issueReportIDField] != result.report.ReportID {
		return
	}
	s.panel.Busy = false
	if result.err != nil {
		s.panel.Lines = []string{
			s.tr("Could not add follow-up comment:"),
			result.err.Error(),
		}
		s.panel.FieldValues[issueCommentIdempotencyField] = result.idempotencyKey
		s.setStatus(s.tr("Issue comment: ") + result.err.Error())
		return
	}
	s.panel.Lines = []string{
		"Follow-up comment added.",
		s.trf("GitHub comment: %s", result.commentURL),
	}
	s.panel.FieldValues[issueReportCommentField] = ""
	delete(s.panel.FieldValues, issueCommentIdempotencyField)
	if s.interfaceUI != nil {
		s.interfaceUI.panelSignature = ""
	}
	s.setStatus(s.tr("Follow-up comment added"))
}
