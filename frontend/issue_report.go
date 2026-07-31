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

	issueReportPathField           = "_report_path"
	issueReportDraftURLField       = "_draft_url"
	issueReportIssueURLField       = "_issue_url"
	issueReportIDField             = "_report_id"
	issueReportCapabilityField     = "_report_capability"
	issueReportIdempotencyField    = "_report_idempotency"
	issueCommentIdempotencyField   = "_comment_idempotency"
	issueReportCommentField        = "comment"
	issueDraftURLLimit             = 7500
	issueReportCommentMaximumRunes = 5000
)

type issueReportDraft struct {
	Situation  string
	GameTitle  string
	Carrier    string
	Repository string
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
			"Describe the problem. ARAM will upload a redacted debug bundle and a screenshot when available.",
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
				ID:          "repository",
				Label:       "Expected repository",
				Value:       "frontend",
				Placeholder: "frontend, emu, or core",
			},
		},
		Actions: []ToolAction{{
			ID:      issueReportPrepareAction,
			Label:   "Submit Report",
			Enabled: true,
		}},
		FieldValues: map[string]string{
			"situation":  "",
			"game_title": gameTitle,
			"carrier":    "",
			"repository": "frontend",
		},
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

	snapshot := s.captureDebugBundleSnapshot(time.Now().UTC())
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
	openErr := openExternalURL(result.report.IssueURL)
	if s.panel != nil && s.panel.Kind == "issue-report" {
		s.panel.Busy = false
		s.panel.Lines = []string{
			"Issue created with the debug bundle.",
			s.trf("GitHub issue: %s", result.report.IssueURL),
		}
		if result.warning != "" {
			s.panel.Lines = append(
				s.panel.Lines,
				s.trf("Collection warning: %s", result.warning),
			)
		}
		s.panel.Fields = []ToolField{{
			ID:          issueReportCommentField,
			Label:       "Follow-up comment",
			Placeholder: "Add more details to the created issue",
		}}
		s.panel.Actions = []ToolAction{
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
			{
				ID:      issueReportFolderAction,
				Label:   "Open Bundle Folder",
				Enabled: true,
			},
		}
		s.panel.FieldValues = map[string]string{
			issueReportCommentField:    "",
			issueReportPathField:       result.path,
			issueReportIssueURLField:   result.report.IssueURL,
			issueReportIDField:         result.report.ReportID,
			issueReportCapabilityField: result.report.Capability,
		}
	}
	if openErr != nil {
		s.setStatus(s.tr("GitHub issue: ") + openErr.Error())
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
		"- Debug bundle: `%s`\n\n> ARAM prepared this redacted ZIP with `screenshot.png` when a guest frame was available. Drag the ZIP into this issue before submitting.\n",
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
