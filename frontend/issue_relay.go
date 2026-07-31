package frontend

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultIssueRelayURL    = "https://aram-report-relay.mirusu400.workers.dev"
	issueRelayResponseLimit = 1 << 20
	issueScreenshotLimit    = 8 << 20
)

type issueRelayService interface {
	Submit(context.Context, issueRelaySubmission) (issueRelayReport, error)
	AddComment(context.Context, issueRelayReport, string, string) (string, error)
}

type issueRelaySubmission struct {
	Draft          issueReportDraft
	Input          *InputInfo
	Backend        string
	State          FrontendState
	BundlePath     string
	Warning        string
	IdempotencyKey string
}

type issueRelayReport struct {
	ReportID   string
	IssueURL   string
	Capability string
}

type issueRelayClient struct {
	baseURL string
	client  *http.Client
}

type issueRelayMetadata struct {
	SchemaVersion int                   `json:"schema_version"`
	Repository    string                `json:"repository"`
	Situation     string                `json:"situation"`
	GameTitle     string                `json:"game_title,omitempty"`
	Carrier       string                `json:"carrier,omitempty"`
	Diagnostics   issueRelayDiagnostics `json:"diagnostics"`
}

type issueRelayDiagnostics struct {
	Backend          string `json:"backend,omitempty"`
	FrontendState    string `json:"frontend_state,omitempty"`
	InputDisplayName string `json:"input_display_name,omitempty"`
	InputFormat      string `json:"input_format,omitempty"`
	ProfileID        string `json:"profile_id,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
	Warning          string `json:"warning,omitempty"`
	AppVersion       string `json:"app_version,omitempty"`
	Platform         string `json:"platform,omitempty"`
}

type issueRelayCreateResponse struct {
	ReportID   string `json:"report_id"`
	IssueURL   string `json:"issue_url"`
	Capability string `json:"capability"`
}

type issueRelayCommentResponse struct {
	CommentURL string `json:"comment_url"`
}

type issueRelayErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newIssueRelayClient() issueRelayService {
	return &issueRelayClient{
		baseURL: defaultIssueRelayURL,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *issueRelayClient) Submit(
	ctx context.Context,
	submission issueRelaySubmission,
) (issueRelayReport, error) {
	if submission.IdempotencyKey == "" {
		return issueRelayReport{}, errors.New("missing report idempotency key")
	}
	screenshot, err := readIssueScreenshot(submission.BundlePath)
	if err != nil {
		return issueRelayReport{}, err
	}
	metadata, err := json.Marshal(issueRelayMetadataFor(submission))
	if err != nil {
		return issueRelayReport{}, fmt.Errorf("encode issue report: %w", err)
	}
	body, contentType := streamIssueMultipart(
		ctx,
		submission.BundlePath,
		metadata,
		screenshot,
	)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1/reports",
		body,
	)
	if err != nil {
		_ = body.Close()
		return issueRelayReport{}, fmt.Errorf("create issue upload request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Idempotency-Key", submission.IdempotencyKey)
	request.Header.Set("User-Agent", "aram-frontend")

	response, err := c.client.Do(request)
	if err != nil {
		return issueRelayReport{}, fmt.Errorf("upload issue report: %w", err)
	}
	defer response.Body.Close()

	var payload issueRelayCreateResponse
	if err := decodeIssueRelayResponse(response, &payload); err != nil {
		return issueRelayReport{}, err
	}
	report := issueRelayReport{
		ReportID:   payload.ReportID,
		IssueURL:   payload.IssueURL,
		Capability: payload.Capability,
	}
	if err := validateIssueRelayReport(report, submission.Draft.Repository); err != nil {
		return issueRelayReport{}, err
	}
	return report, nil
}

func (c *issueRelayClient) AddComment(
	ctx context.Context,
	report issueRelayReport,
	comment string,
	idempotencyKey string,
) (string, error) {
	if idempotencyKey == "" {
		return "", errors.New("missing comment idempotency key")
	}
	if err := validateIssueRelayReport(report, ""); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{"body": comment})
	if err != nil {
		return "", fmt.Errorf("encode issue comment: %w", err)
	}
	requestURL := fmt.Sprintf(
		"%s/v1/reports/%s/comments",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(report.ReportID),
	)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", fmt.Errorf("create issue comment request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+report.Capability)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("User-Agent", "aram-frontend")

	response, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("add issue comment: %w", err)
	}
	defer response.Body.Close()

	var result issueRelayCommentResponse
	if err := decodeIssueRelayResponse(response, &result); err != nil {
		return "", err
	}
	if err := validateGitHubCommentURL(
		result.CommentURL,
		report.IssueURL,
	); err != nil {
		return "", fmt.Errorf("invalid relay comment response: %w", err)
	}
	return result.CommentURL, nil
}

func issueRelayMetadataFor(
	submission issueRelaySubmission,
) issueRelayMetadata {
	diagnostics := issueRelayDiagnostics{
		Backend:       shorten(submission.Backend, 100),
		FrontendState: string(submission.State),
		Warning:       shorten(submission.Warning, 300),
		AppVersion:    currentIssueReportVersion(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
	}
	if submission.Input != nil {
		diagnostics.InputDisplayName = shorten(
			submission.Input.DisplayName,
			100,
		)
		diagnostics.InputFormat = shorten(submission.Input.Format, 40)
		diagnostics.ProfileID = shorten(submission.Input.ProfileID, 100)
		if validIssueSHA256(submission.Input.SHA256) {
			diagnostics.SHA256 = strings.ToLower(submission.Input.SHA256)
		}
	}
	return issueRelayMetadata{
		SchemaVersion: 1,
		Repository:    submission.Draft.Repository,
		Situation:     submission.Draft.Situation,
		GameTitle:     submission.Draft.GameTitle,
		Carrier:       submission.Draft.Carrier,
		Diagnostics:   diagnostics,
	}
}

func validIssueSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}

func currentIssueReportVersion() string {
	return currentApplicationVersion()
}

func readIssueScreenshot(bundlePath string) ([]byte, error) {
	archive, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open debug bundle for upload: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "screenshot.png" || file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > issueScreenshotLimit {
			return nil, errors.New("debug screenshot exceeds the upload limit")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open debug screenshot: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, issueScreenshotLimit+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read debug screenshot: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close debug screenshot: %w", closeErr)
		}
		if len(data) > issueScreenshotLimit {
			return nil, errors.New("debug screenshot exceeds the upload limit")
		}
		if len(data) < 8 ||
			!bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
			return nil, errors.New("debug screenshot is not a PNG")
		}
		return data, nil
	}
	return nil, nil
}

func streamIssueMultipart(
	ctx context.Context,
	bundlePath string,
	metadata []byte,
	screenshot []byte,
) (io.ReadCloser, string) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()
	go func() {
		err := writeIssueMultipart(
			ctx,
			multipartWriter,
			bundlePath,
			metadata,
			screenshot,
		)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
	}()
	return reader, contentType
}

func writeIssueMultipart(
	ctx context.Context,
	writer *multipart.Writer,
	bundlePath string,
	metadata []byte,
	screenshot []byte,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	metadataPart, err := writer.CreateFormField("report")
	if err != nil {
		return fmt.Errorf("create report metadata part: %w", err)
	}
	if _, err := metadataPart.Write(metadata); err != nil {
		return fmt.Errorf("write report metadata: %w", err)
	}

	bundle, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("open debug bundle: %w", err)
	}
	defer bundle.Close()
	bundlePart, err := writer.CreateFormFile(
		"bundle",
		filepath.Base(bundlePath),
	)
	if err != nil {
		return fmt.Errorf("create debug bundle part: %w", err)
	}
	if _, err := io.Copy(bundlePart, bundle); err != nil {
		return fmt.Errorf("write debug bundle: %w", err)
	}

	if len(screenshot) > 0 {
		screenshotPart, err := writer.CreateFormFile(
			"screenshot",
			"screenshot.png",
		)
		if err != nil {
			return fmt.Errorf("create screenshot part: %w", err)
		}
		if _, err := screenshotPart.Write(screenshot); err != nil {
			return fmt.Errorf("write screenshot: %w", err)
		}
	}
	return ctx.Err()
}

func decodeIssueRelayResponse(
	response *http.Response,
	target any,
) error {
	data, err := io.ReadAll(io.LimitReader(
		response.Body,
		issueRelayResponseLimit+1,
	))
	if err != nil {
		return fmt.Errorf("read report relay response: %w", err)
	}
	if len(data) > issueRelayResponseLimit {
		return errors.New("report relay response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiError issueRelayErrorResponse
		if json.Unmarshal(data, &apiError) == nil &&
			strings.TrimSpace(apiError.Error.Message) != "" {
			return fmt.Errorf(
				"report relay %s: %s",
				emptyFallback(apiError.Error.Code, strconv.Itoa(response.StatusCode)),
				apiError.Error.Message,
			)
		}
		return fmt.Errorf(
			"report relay returned HTTP %d",
			response.StatusCode,
		)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode report relay response: %w", err)
	}
	return nil
}

func validateIssueRelayReport(
	report issueRelayReport,
	repository string,
) error {
	if !validIssueUUID(report.ReportID) {
		return errors.New("invalid report ID")
	}
	const capabilityPrefix = "aram_rpt_"
	if !strings.HasPrefix(report.Capability, capabilityPrefix) ||
		len(report.Capability) != len(capabilityPrefix)+43 {
		return errors.New("invalid report capability")
	}
	for _, character := range strings.TrimPrefix(
		report.Capability,
		capabilityPrefix,
	) {
		if !strings.ContainsRune(
			"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_",
			character,
		) {
			return errors.New("invalid report capability")
		}
	}
	if err := validateGitHubIssueURL(report.IssueURL, repository); err != nil {
		return fmt.Errorf("invalid issue URL: %w", err)
	}
	return nil
}

func validateGitHubIssueURL(value, repository string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" ||
		parsed.Hostname() != "github.com" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return errors.New("expected an HTTPS github.com URL")
	}
	segments := strings.Split(strings.Trim(path.Clean(parsed.Path), "/"), "/")
	if len(segments) != 4 ||
		segments[0] != "mirusu400" ||
		segments[2] != "issues" {
		return errors.New("expected an ARAM GitHub issue URL")
	}
	if repository != "" && segments[1] != repository {
		return errors.New("issue repository does not match the report")
	}
	if _, err := strconv.ParseUint(segments[3], 10, 64); err != nil {
		return errors.New("issue number is invalid")
	}
	return nil
}

func validateGitHubCommentURL(value, issueURL string) error {
	comment, err := url.Parse(value)
	if err != nil {
		return err
	}
	issue, err := url.Parse(issueURL)
	if err != nil {
		return err
	}
	if comment.Scheme != issue.Scheme ||
		comment.Host != issue.Host ||
		path.Clean(comment.Path) != path.Clean(issue.Path) ||
		comment.RawQuery != "" ||
		!strings.HasPrefix(comment.Fragment, "issuecomment-") {
		return errors.New("comment URL does not match the created issue")
	}
	return nil
}

func newIssueIdempotencyKey() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create report idempotency key: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}

func validIssueUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
				return false
			}
		}
	}
	return true
}
