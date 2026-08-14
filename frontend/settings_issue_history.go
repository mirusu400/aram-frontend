package frontend

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const issueReportHistoryLimit = 20

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
