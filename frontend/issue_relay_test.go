package frontend

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueRelayUploadsMultipartReport(t *testing.T) {
	bundlePath := writeIssueRelayTestBundle(t, true)
	idempotencyKey := "22222222-2222-4222-8222-222222222222"
	var received issueRelayMetadata

	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method != http.MethodPost ||
			request.URL.Path != "/v1/reports" {
			http.Error(response, "unexpected route", http.StatusNotFound)
			return
		}
		if request.Header.Get("Idempotency-Key") != idempotencyKey ||
			request.Header.Get("User-Agent") != "aram-frontend" {
			http.Error(response, "missing headers", http.StatusBadRequest)
			return
		}
		if err := request.ParseMultipartForm(40 << 20); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(
			[]byte(request.FormValue("report")),
			&received,
		); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		for _, name := range []string{"bundle", "screenshot"} {
			file, _, err := request.FormFile(name)
			if err != nil {
				http.Error(response, err.Error(), http.StatusBadRequest)
				return
			}
			data, readErr := io.ReadAll(file)
			_ = file.Close()
			if readErr != nil || len(data) < 4 {
				http.Error(response, "empty upload", http.StatusBadRequest)
				return
			}
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{
			"report_id":"11111111-1111-4111-8111-111111111111",
			"issue_url":"https://github.com/mirusu400/aram-core/issues/42",
			"capability":"aram_rpt_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}`))
	}))
	t.Cleanup(server.Close)

	client := &issueRelayClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	report, err := client.Submit(context.Background(), issueRelaySubmission{
		Draft: issueReportDraft{
			Situation:  "Guest execution stops.",
			GameTitle:  "Synthetic",
			Carrier:    "KTF",
			Repository: "aram-core",
		},
		Input: &InputInfo{
			DisplayName: "synthetic.dat",
			Format:      "wipi",
			ProfileID:   "ktf/arm",
			SHA256:      strings.Repeat("b", 64),
		},
		Backend:        "aram-core",
		State:          FrontendRunning,
		BundlePath:     bundlePath,
		Warning:        "partial backend diagnostics",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReportID != "11111111-1111-4111-8111-111111111111" ||
		report.IssueURL != "https://github.com/mirusu400/aram-core/issues/42" {
		t.Fatalf("relay report = %#v", report)
	}
	if received.SchemaVersion != 1 ||
		received.Repository != "aram-core" ||
		received.Diagnostics.InputDisplayName != "synthetic.dat" ||
		received.Diagnostics.Platform == "" ||
		received.Diagnostics.AppVersion == "" {
		t.Fatalf("relay metadata = %#v", received)
	}
}

func TestIssueRelayAddsAuthorizedComment(t *testing.T) {
	idempotencyKey := "33333333-3333-4333-8333-333333333333"
	report := issueRelayReport{
		ReportID:   "11111111-1111-4111-8111-111111111111",
		IssueURL:   "https://github.com/mirusu400/aram-frontend/issues/42",
		Capability: "aram_rpt_" + strings.Repeat("A", 43),
	}
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path !=
			"/v1/reports/11111111-1111-4111-8111-111111111111/comments" {
			http.Error(response, "unexpected route", http.StatusNotFound)
			return
		}
		if request.Header.Get("Authorization") !=
			"Bearer "+report.Capability ||
			request.Header.Get("Idempotency-Key") != idempotencyKey {
			http.Error(response, "missing authorization", http.StatusUnauthorized)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil ||
			body["body"] != "Additional detail." {
			http.Error(response, "invalid comment", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{
			"comment_url":"https://github.com/mirusu400/aram-frontend/issues/42#issuecomment-7"
		}`))
	}))
	t.Cleanup(server.Close)

	client := &issueRelayClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	commentURL, err := client.AddComment(
		context.Background(),
		report,
		"Additional detail.",
		idempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if commentURL !=
		"https://github.com/mirusu400/aram-frontend/issues/42#issuecomment-7" {
		t.Fatalf("comment URL = %q", commentURL)
	}
}

func TestIssueRelayUploadsWithoutScreenshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		if err := request.ParseMultipartForm(40 << 20); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		bundle, _, err := request.FormFile("bundle")
		if err != nil {
			http.Error(response, "missing bundle", http.StatusBadRequest)
			return
		}
		_ = bundle.Close()
		if _, _, err := request.FormFile("screenshot"); err == nil {
			http.Error(response, "unexpected screenshot", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{
			"report_id":"11111111-1111-4111-8111-111111111111",
			"issue_url":"https://github.com/mirusu400/aram-frontend/issues/42",
			"capability":"aram_rpt_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}`))
	}))
	t.Cleanup(server.Close)

	client := &issueRelayClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	if _, err := client.Submit(context.Background(), issueRelaySubmission{
		Draft: issueReportDraft{
			Situation:  "Broken",
			Repository: "aram-frontend",
		},
		BundlePath:     writeIssueRelayTestBundle(t, false),
		IdempotencyKey: "22222222-2222-4222-8222-222222222222",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIssueRelayRejectsUnexpectedIssueHost(t *testing.T) {
	report := issueRelayReport{
		ReportID:   "11111111-1111-4111-8111-111111111111",
		IssueURL:   "https://example.com/mirusu400/aram-frontend/issues/1",
		Capability: "aram_rpt_" + strings.Repeat("A", 43),
	}
	if err := validateIssueRelayReport(report, "aram-frontend"); err == nil {
		t.Fatal("unexpected issue host was accepted")
	}
	if err := validateGitHubCommentURL(
		"https://github.com/mirusu400/aram-frontend/issues/2#issuecomment-7",
		"https://github.com/mirusu400/aram-frontend/issues/1",
	); err == nil {
		t.Fatal("comment URL for another issue was accepted")
	}
}

func TestIssueIdempotencyKeysAreRFC4122Version4(t *testing.T) {
	seen := make(map[string]bool)
	for range 32 {
		key, err := newIssueIdempotencyKey()
		if err != nil {
			t.Fatal(err)
		}
		if !validIssueUUID(key) ||
			key[14] != '4' ||
			!strings.ContainsRune("89ab", rune(key[19])) {
			t.Fatalf("idempotency key = %q", key)
		}
		if seen[key] {
			t.Fatalf("duplicate idempotency key = %q", key)
		}
		seen[key] = true
	}
}

func writeIssueRelayTestBundle(t *testing.T, screenshot bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aram-debug-test.zip")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	manifest, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.Write([]byte(`{"schema_version":1}`)); err != nil {
		t.Fatal(err)
	}
	if screenshot {
		entry, err := writer.Create("screenshot.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
