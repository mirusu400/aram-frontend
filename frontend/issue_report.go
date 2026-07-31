package frontend

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	issueReportPrepareAction = "prepare_report"
	issueReportFolderAction  = "open_report_folder"
	issueReportDraftAction   = "open_report_draft"

	issueReportURLField  = "_report_url"
	issueReportPathField = "_report_path"
	issueDraftURLLimit   = 7500
)

type issueReportDraft struct {
	Situation  string
	GameTitle  string
	Carrier    string
	Repository string
}

type issueReportResult struct {
	draft   issueReportDraft
	input   *InputInfo
	backend string
	state   FrontendState
	path    string
	warning string
	err     error
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
			"Describe the problem. ARAM will prepare a redacted debug bundle with the current screenshot.",
			"GitHub will open a reviewable draft; attach the prepared ZIP before submitting.",
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
			Label:   "Prepare Report",
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

	s.panel.Busy = true
	s.panel.Lines = []string{"Collecting debug bundle and screenshot..."}
	s.panel.Actions = []ToolAction{{
		ID:      issueReportPrepareAction,
		Label:   "Prepare Report",
		Enabled: false,
	}}
	s.setStatus(s.tr("Collecting issue report..."))

	snapshot := s.captureDebugBundleSnapshot(time.Now().UTC())
	backend := s.backend
	go func() {
		path, warning, bundleErr := collectDebugBundle(snapshot, backend)
		s.issueReportResults <- issueReportResult{
			draft:   draft,
			input:   snapshot.Input,
			backend: snapshot.Backend,
			state:   snapshot.FrontendState,
			path:    path,
			warning: warning,
			err:     bundleErr,
		}
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
		s.panel.FieldValues[issueReportURLField] = draftURL
		s.panel.FieldValues[issueReportPathField] = result.path
		s.panel.Busy = false
		s.panel.Lines = []string{
			"Debug bundle and screenshot are ready.",
			s.trf(
				"Attach %s to the GitHub draft, review it, then submit.",
				filepath.Base(result.path),
			),
		}
		s.panel.Actions = []ToolAction{
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
	case result.warning != "":
		s.setStatus(s.trf(
			"Issue report ready with warning: %s",
			result.warning,
		))
	default:
		s.setStatus(s.tr("Issue report ready; attach the ZIP and submit the GitHub draft"))
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
	draftURL := s.panel.FieldValues[issueReportURLField]
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
		"- Debug bundle: `%s`\n\n> ARAM prepared this redacted ZIP with `screenshot.png`. Drag the ZIP into this issue before submitting.\n",
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
