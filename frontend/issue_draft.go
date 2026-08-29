package frontend

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
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
)

type issueReportDraft struct {
	Situation  string
	GameTitle  string
	Carrier    string
	Repository string
}

func issueReportScreenshotEnabled(fields map[string]string) bool {
	value, ok := fields[issueReportScreenshotField]
	if !ok {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(value), "false")
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
			"Expected repository must be frontend, emu, core, or cheat.",
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
	case "cheat", "aram-cheat", "치트":
		return "aram-cheat"
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
		if input.ImageSHA256 != "" {
			// Cheat catalogs are keyed on the image, so a report that asks for
			// one has to carry the identity that answers.
			fmt.Fprintf(
				&body,
				"- Image SHA-256: `%s`\n",
				shorten(input.ImageSHA256, 64),
			)
		}
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
