package frontend

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	issueReportPrepareAction   = "prepare_report"
	issueReportFolderAction    = "open_report_folder"
	issueReportDraftAction     = "open_report_draft"
	issueReportOpenIssueAction = "open_created_issue"
	issueReportCommentAction   = "add_issue_comment"
	issueReportHistoryAction   = "open_report_history"
	issueReportHistoryView     = "view_saved_report"
	issueReportHistoryForget   = "forget_saved_report"
	issueReportNewAction       = "new_issue_report"

	issueReportPathField           = "_report_path"
	issueReportDraftURLField       = "_draft_url"
	issueReportIssueURLField       = "_issue_url"
	issueReportIDField             = "_report_id"
	issueReportHistoryField        = "_saved_report_id"
	issueReportCapabilityField     = "_report_capability"
	issueReportIdempotencyField    = "_report_idempotency"
	issueCommentIdempotencyField   = "_comment_idempotency"
	issueReportCommentField        = "comment"
	issueReportScreenshotField     = "include_screenshot"
	issueDraftURLLimit             = 7500
	issueReportCommentMaximumRunes = 5000
	issueReportHistoryLimit        = 20
)

type issueReportDraft struct {
	Situation  string
	GameTitle  string
	Carrier    string
	Repository string
}

// IssueReportRecord is the local handle for a report created through the
// relay. Capability authorizes comments for this report only; it is kept in
// the user-private settings file and is never included in a debug bundle.
type IssueReportRecord struct {
	ReportID   string    `json:"report_id"`
	IssueURL   string    `json:"issue_url"`
	Capability string    `json:"capability"`
	Repository string    `json:"repository"`
	Situation  string    `json:"situation"`
	GameTitle  string    `json:"game_title,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

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
				Value: "aram-frontend",
				Options: []ToolFieldOption{
					{Value: "aram-frontend", Label: "ARAM Frontend"},
					{Value: "aram-emu", Label: "ARAM Emulator"},
					{Value: "aram-core", Label: "ARAM Core"},
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
			"repository":               "aram-frontend",
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

func issueReportScreenshotEnabled(fields map[string]string) bool {
	value, ok := fields[issueReportScreenshotField]
	if !ok {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(value), "false")
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

func issueReportHistoryLabel(record IssueReportRecord) string {
	title := strings.TrimSpace(record.GameTitle)
	if title == "" {
		title = strings.TrimSpace(record.Situation)
	}
	if title == "" {
		title = "Unknown report"
	}
	return fmt.Sprintf(
		"%s  -  %s  -  %s",
		record.CreatedAt.Local().Format("2006-01-02 15:04"),
		record.Repository,
		shorten(strings.ReplaceAll(title, "\n", " "), 42),
	)
}

func (s *Settings) normalizeIssueReports() {
	normalized := make([]IssueReportRecord, 0, min(
		len(s.IssueReports),
		issueReportHistoryLimit,
	))
	seen := make(map[string]bool)
	for _, record := range s.IssueReports {
		record.ReportID = strings.TrimSpace(record.ReportID)
		record.IssueURL = strings.TrimSpace(record.IssueURL)
		record.Capability = strings.TrimSpace(record.Capability)
		record.Repository = normalizeIssueRepository(record.Repository)
		record.Situation = strings.TrimSpace(record.Situation)
		record.GameTitle = strings.TrimSpace(record.GameTitle)
		if record.Repository == "" ||
			record.CreatedAt.IsZero() ||
			seen[record.ReportID] ||
			utf8.RuneCountInString(record.Situation) > 500 ||
			utf8.RuneCountInString(record.GameTitle) > 200 ||
			validateIssueRelayReport(
				record.relayReport(),
				record.Repository,
			) != nil {
			continue
		}
		seen[record.ReportID] = true
		normalized = append(normalized, record)
		if len(normalized) == issueReportHistoryLimit {
			break
		}
	}
	s.IssueReports = normalized
}

func (s *Settings) rememberIssueReport(record IssueReportRecord) {
	reports := make(
		[]IssueReportRecord,
		0,
		min(len(s.IssueReports)+1, issueReportHistoryLimit),
	)
	reports = append(reports, record)
	for _, existing := range s.IssueReports {
		if existing.ReportID == record.ReportID {
			continue
		}
		reports = append(reports, existing)
		if len(reports) == issueReportHistoryLimit {
			break
		}
	}
	s.IssueReports = reports
	s.normalizeIssueReports()
}

func (s Settings) issueReport(reportID string) (IssueReportRecord, bool) {
	for _, record := range s.IssueReports {
		if record.ReportID == reportID {
			return record, true
		}
	}
	return IssueReportRecord{}, false
}

func (s *Settings) forgetIssueReport(reportID string) bool {
	for index, record := range s.IssueReports {
		if record.ReportID != reportID {
			continue
		}
		s.IssueReports = append(
			s.IssueReports[:index],
			s.IssueReports[index+1:]...,
		)
		return true
	}
	return false
}

func (record IssueReportRecord) relayReport() issueRelayReport {
	return issueRelayReport{
		ReportID:   record.ReportID,
		IssueURL:   record.IssueURL,
		Capability: record.Capability,
	}
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

func newIssueReportDraft(fields map[string]string) (issueReportDraft, error) {
	draft := issueReportDraft{
		Situation:  strings.TrimSpace(fields["situation"]),
		GameTitle:  strings.TrimSpace(fields["game_title"]),
		Carrier:    strings.TrimSpace(fields["carrier"]),
		Repository: normalizeIssueRepository(fields["repository"]),
	}
	if draft.Situation == "" {
		return issueReportDraft{}, errors.New("Situation is required.")
	}
	if utf8.RuneCountInString(draft.Situation) > 500 {
		return issueReportDraft{}, errors.New(
			"Situation must be 500 characters or fewer.",
		)
	}
	if draft.Repository == "" {
		return issueReportDraft{}, errors.New(
			"Expected repository must be frontend, emu, or core.",
		)
	}
	if utf8.RuneCountInString(draft.GameTitle) > 200 ||
		utf8.RuneCountInString(draft.Carrier) > 100 {
		return issueReportDraft{}, errors.New(
			"Game title or carrier is too long.",
		)
	}
	return draft, nil
}

func normalizeIssueRepository(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "frontend", "aram-frontend", "프론트엔드":
		return "aram-frontend"
	case "emu", "aram-emu", "에뮤":
		return "aram-emu"
	case "core", "aram-core", "코어":
		return "aram-core"
	default:
		return ""
	}
}

func buildIssueDraftURL(
	draft issueReportDraft,
	input *InputInfo,
	backend string,
	state FrontendState,
	bundlePath string,
	warning string,
) (string, error) {
	gameTitle := draft.GameTitle
	if gameTitle == "" {
		gameTitle = "Unknown title"
	}
	carrier := draft.Carrier
	if carrier == "" {
		carrier = "Unknown"
	}
	title := fmt.Sprintf(
		"[ARAM] %s - %s",
		gameTitle,
		shorten(strings.ReplaceAll(draft.Situation, "\n", " "), 80),
	)

	var body strings.Builder
	fmt.Fprintf(&body, "## Situation\n\n%s\n\n", draft.Situation)
	fmt.Fprintf(&body, "## Game\n\n- Title: %s\n- Carrier: %s\n\n", gameTitle, carrier)
	fmt.Fprintf(
		&body,
		"## ARAM diagnostics\n\n- Expected repository: `%s`\n- Backend: `%s`\n- Frontend state: `%s`\n",
		draft.Repository,
		shorten(backend, 100),
		state,
	)
	if input != nil {
		fmt.Fprintf(
			&body,
			"- Input: `%s`\n- Format: `%s`\n- Profile: `%s`\n- SHA-256: `%s`\n",
			shorten(input.DisplayName, 100),
			shorten(input.Format, 40),
			shorten(input.ProfileID, 100),
			shorten(input.SHA256, 64),
		)
	}
	fmt.Fprintf(
		&body,
		"- Debug bundle: `%s`\n\n> ARAM prepared this redacted ZIP with `screenshot.png` only when image transfer was enabled and a guest frame was available. Drag the ZIP into this issue before submitting.\n",
		filepath.Base(bundlePath),
	)
	if warning != "" {
		fmt.Fprintf(
			&body,
			"\n> Collection warning: %s\n",
			shorten(warning, 300),
		)
	}

	query := url.Values{}
	query.Set("title", title)
	query.Set("body", body.String())
	rawURL := fmt.Sprintf(
		"https://github.com/mirusu400/%s/issues/new?%s",
		draft.Repository,
		query.Encode(),
	)
	if len(rawURL) > issueDraftURLLimit {
		return "", errors.New(
			"Issue report is too long; shorten the situation or game title.",
		)
	}
	return rawURL, nil
}
