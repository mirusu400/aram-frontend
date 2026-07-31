package frontend

import (
	"context"
	"errors"
	"image"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeIssueRelay struct {
	report     issueRelayReport
	submitErr  error
	submission issueRelaySubmission
	comment    string
	commentKey string
	commentURL string
	commentErr error
}

func (relay *fakeIssueRelay) Submit(
	_ context.Context,
	submission issueRelaySubmission,
) (issueRelayReport, error) {
	relay.submission = submission
	return relay.report, relay.submitErr
}

func (relay *fakeIssueRelay) AddComment(
	_ context.Context,
	_ issueRelayReport,
	comment string,
	idempotencyKey string,
) (string, error) {
	relay.comment = comment
	relay.commentKey = idempotencyKey
	return relay.commentURL, relay.commentErr
}

func successfulFakeIssueRelay() *fakeIssueRelay {
	return &fakeIssueRelay{
		report: issueRelayReport{
			ReportID:   "11111111-1111-4111-8111-111111111111",
			IssueURL:   "https://github.com/mirusu400/aram-frontend/issues/42",
			Capability: "aram_rpt_" + strings.Repeat("A", 43),
		},
		commentURL: "https://github.com/mirusu400/aram-frontend/issues/42#issuecomment-7",
	}
}

func TestOpenIssueTrackerShowsReportFormWithCurrentTitle(t *testing.T) {
	shell := &Shell{
		input: &InputInfo{DisplayName: "Maple Archer"},
	}
	shell.openIssueTracker()

	if shell.panel == nil || shell.panel.Kind != "issue-report" {
		t.Fatalf("issue panel = %#v", shell.panel)
	}
	if shell.panel.FieldValues["game_title"] != "Maple Archer" ||
		shell.panel.FieldValues["repository"] != "frontend" {
		t.Fatalf("issue form values = %#v", shell.panel.FieldValues)
	}
	if len(shell.panel.Fields) != 4 || len(shell.panel.Actions) != 1 {
		t.Fatalf("issue form = %#v", shell.panel)
	}
}

func TestIssueReportDraftValidationAndRepositoryRouting(t *testing.T) {
	draft, err := newIssueReportDraft(map[string]string{
		"situation":  "Sprites are missing after opening the title screen.",
		"game_title": "Maple Archer",
		"carrier":    "KTF",
		"repository": "core",
	})
	if err != nil {
		t.Fatal(err)
	}
	rawURL, err := buildIssueDraftURL(
		draft,
		&InputInfo{
			DisplayName: "maple.dat",
			Format:      "wipi",
			ProfileID:   "ktf/arm",
			SHA256:      strings.Repeat("a", 64),
		},
		"aram-core",
		FrontendRunning,
		filepath.Join("private", "aram-debug.zip"),
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "github.com" ||
		parsed.Path != "/mirusu400/aram-core/issues/new" {
		t.Fatalf("issue URL = %q", rawURL)
	}
	body := parsed.Query().Get("body")
	for _, expected := range []string{
		"Sprites are missing",
		"Maple Archer",
		"KTF",
		"aram-debug.zip",
		"screenshot.png",
		strings.Repeat("a", 64),
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("issue body is missing %q: %q", expected, body)
		}
	}
	if strings.Contains(body, filepath.Join("private", "aram-debug.zip")) {
		t.Fatalf("issue body leaked bundle path: %q", body)
	}
}

func TestIssueReportDraftRejectsMissingSituationAndUnknownRepository(t *testing.T) {
	if _, err := newIssueReportDraft(map[string]string{
		"repository": "frontend",
	}); err == nil {
		t.Fatal("empty situation was accepted")
	}
	if _, err := newIssueReportDraft(map[string]string{
		"situation":  "broken",
		"repository": "somewhere-else",
	}); err == nil {
		t.Fatal("unknown repository was accepted")
	}
	if normalized := normalizeIssueRepository("코어"); normalized != "aram-core" {
		t.Fatalf("Korean repository alias = %q", normalized)
	}
}

func TestPrepareIssueReportUploadsBundleAndOpensCreatedIssue(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	originalURL := openExternalURL
	originalFolder := openArtifactFolder
	t.Cleanup(func() {
		openExternalURL = originalURL
		openArtifactFolder = originalFolder
	})
	var openedURL string
	var openedFolder string
	openExternalURL = func(value string) error {
		openedURL = value
		return nil
	}
	openArtifactFolder = func(value string) error {
		openedFolder = value
		return nil
	}

	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	frame.Set(0, 0, color.RGBA{R: 0xee, G: 0x11, B: 0x22, A: 0xff})
	shell := NewShell(NullBackend{}, nil, "")
	relay := successfulFakeIssueRelay()
	shell.issueRelay = relay
	shell.frame = VideoFrame{Image: frame, Sequence: 1}
	shell.input = &InputInfo{
		DisplayName: "synthetic.dat",
		SHA256:      strings.Repeat("b", 64),
	}
	shell.openIssueTracker()
	shell.executeIssueReportAction(issueReportPrepareAction, map[string]string{
		"situation":  "The screen is incorrect.",
		"game_title": "Synthetic",
		"carrier":    "KTF",
		"repository": "frontend",
	})

	select {
	case result := <-shell.issueReportResults:
		shell.consumeIssueReportResult(result)
	case <-time.After(2 * time.Second):
		t.Fatal("issue report did not complete")
	}

	if openedURL != relay.report.IssueURL || openedFolder != "" {
		t.Fatalf("opened URL=%q folder=%q", openedURL, openedFolder)
	}
	if shell.panel == nil || shell.panel.Busy {
		t.Fatalf("completed issue panel = %#v", shell.panel)
	}
	bundlePath := shell.panel.FieldValues[issueReportPathField]
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("prepared bundle = %q: %v", bundlePath, err)
	}
	entries := readDebugZIP(t, bundlePath)
	if _, ok := entries["screenshot.png"]; !ok {
		t.Fatal("prepared report bundle has no screenshot")
	}
	if relay.submission.BundlePath != bundlePath ||
		relay.submission.Draft.Repository != "aram-frontend" ||
		!validIssueUUID(relay.submission.IdempotencyKey) {
		t.Fatalf("relay submission = %#v", relay.submission)
	}
	if shell.panel.FieldValues[issueReportCapabilityField] !=
		relay.report.Capability {
		t.Fatalf("completed issue panel = %#v", shell.panel.FieldValues)
	}
	shell.executeIssueReportAction(
		issueReportFolderAction,
		shell.panel.FieldValues,
	)
	if filepath.Clean(openedFolder) != filepath.Clean(filepath.Dir(bundlePath)) {
		t.Fatalf("opened folder = %q, bundle = %q", openedFolder, bundlePath)
	}
}

func TestIssueReportFallsBackToManualDraftWhenRelayFails(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	originalURL := openExternalURL
	originalFolder := openArtifactFolder
	t.Cleanup(func() {
		openExternalURL = originalURL
		openArtifactFolder = originalFolder
	})
	var openedURL string
	var openedFolder string
	openExternalURL = func(value string) error {
		openedURL = value
		return nil
	}
	openArtifactFolder = func(value string) error {
		openedFolder = value
		return nil
	}

	frame := image.NewRGBA(image.Rect(0, 0, 1, 1))
	shell := NewShell(NullBackend{}, nil, "")
	shell.issueRelay = &fakeIssueRelay{
		submitErr: errors.New("relay unavailable"),
	}
	shell.frame = VideoFrame{Image: frame, Sequence: 1}
	shell.openIssueTracker()
	shell.executeIssueReportAction(issueReportPrepareAction, map[string]string{
		"situation":  "The screen is incorrect.",
		"game_title": "Synthetic",
		"repository": "frontend",
	})

	select {
	case result := <-shell.issueReportResults:
		shell.consumeIssueReportResult(result)
	case <-time.After(2 * time.Second):
		t.Fatal("issue report did not complete")
	}

	parsed, err := url.Parse(openedURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "github.com" ||
		parsed.Path != "/mirusu400/aram-frontend/issues/new" ||
		openedFolder == "" {
		t.Fatalf("fallback URL=%q folder=%q", openedURL, openedFolder)
	}
	if shell.panel.FieldValues[issueReportDraftURLField] == "" ||
		!validIssueUUID(
			shell.panel.FieldValues[issueReportIdempotencyField],
		) {
		t.Fatalf("fallback fields = %#v", shell.panel.FieldValues)
	}
	if len(shell.panel.Actions) != 3 ||
		shell.panel.Actions[0].ID != issueReportPrepareAction {
		t.Fatalf("fallback actions = %#v", shell.panel.Actions)
	}
}

func TestCreatedIssueAcceptsFollowUpComment(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	originalURL := openExternalURL
	originalFolder := openArtifactFolder
	t.Cleanup(func() {
		openExternalURL = originalURL
		openArtifactFolder = originalFolder
	})
	openExternalURL = func(string) error { return nil }
	openArtifactFolder = func(string) error { return nil }

	frame := image.NewRGBA(image.Rect(0, 0, 1, 1))
	shell := NewShell(NullBackend{}, nil, "")
	relay := successfulFakeIssueRelay()
	shell.issueRelay = relay
	shell.frame = VideoFrame{Image: frame, Sequence: 1}
	shell.openIssueTracker()
	shell.executeIssueReportAction(issueReportPrepareAction, map[string]string{
		"situation":  "The screen is incorrect.",
		"repository": "frontend",
	})
	select {
	case result := <-shell.issueReportResults:
		shell.consumeIssueReportResult(result)
	case <-time.After(2 * time.Second):
		t.Fatal("issue report did not complete")
	}

	shell.panel.FieldValues[issueReportCommentField] = "It also fails after reset."
	shell.executeIssueReportAction(
		issueReportCommentAction,
		shell.panel.FieldValues,
	)
	select {
	case result := <-shell.issueCommentResults:
		shell.consumeIssueCommentResult(result)
	case <-time.After(2 * time.Second):
		t.Fatal("issue comment did not complete")
	}

	if relay.comment != "It also fails after reset." ||
		!validIssueUUID(relay.commentKey) {
		t.Fatalf("comment=%q key=%q", relay.comment, relay.commentKey)
	}
	if shell.panel.Busy ||
		shell.panel.FieldValues[issueReportCommentField] != "" ||
		shell.panel.FieldValues[issueCommentIdempotencyField] != "" {
		t.Fatalf("completed comment panel = %#v", shell.panel)
	}
}
