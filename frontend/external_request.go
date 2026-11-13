package frontend

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const externalRequestPanelKind = "external-request"

// syncExternalRequests presents at most one guest-owned handset action at a
// time. It runs on the UI goroutine so opening a browser remains tied to an
// explicit button press and never happens merely because a title was loaded.
func (s *Shell) syncExternalRequests() {
	backend, ok := s.backend.(ExternalRequestBackend)
	if !ok || s.input == nil || s.loading {
		return
	}
	if pending := s.pendingExternalRequest; pending != nil {
		if pending.generation != s.frameGeneration {
			s.resetExternalRequestPrompt()
			return
		}
		// Keep an acknowledged-by-the-user decision mandatory even if another
		// UI path happened to dismiss the panel without going through its close
		// handler. Re-present the same request instead of leaving it hidden.
		if s.panel == nil && !s.dialogOpen {
			s.panel = externalRequestPanel(s, pending.request, pending.failure)
		}
		return
	}
	if s.panel != nil || s.dialogOpen {
		return
	}
	requests := backend.ExternalRequests()
	if len(requests) == 0 {
		return
	}
	request := requests[0]
	if err := validateExternalRequest(request); err != nil {
		backend.ClearExternalRequests(request.Sequence)
		s.appendLog(s.trf("Blocked external request: %s", err.Error()))
		s.setStatus(s.trf("Blocked external request: %s", err.Error()))
		return
	}
	if request.Kind == ExternalRequestBrowser {
		target, err := validateGuestBrowserURL(request.Target)
		if err != nil {
			backend.ClearExternalRequests(request.Sequence)
			s.appendLog(s.trf("Blocked external link: %s", err.Error()))
			s.setStatus(s.trf("Blocked external link: %s", err.Error()))
			return
		}
		request.Target = target
	}

	s.pendingExternalRequest = &pendingExternalRequest{
		request:    request,
		generation: s.frameGeneration,
	}
	s.panel = externalRequestPanel(s, request, "")
	s.setStatus(s.tr("Waiting for external action confirmation"))
}

func validateExternalRequest(request ExternalRequest) error {
	if request.Sequence == 0 {
		return fmt.Errorf("request sequence is zero")
	}
	if request.GuestNS < 0 {
		return fmt.Errorf("request timestamp is negative")
	}
	if len(request.Target) > 4096 || len(request.Data) > 4096 {
		return fmt.Errorf("request data exceeds the frontend limit")
	}
	switch request.Kind {
	case ExternalRequestBrowser, ExternalRequestPhone, ExternalRequestSMS:
	default:
		return fmt.Errorf("unknown request kind %q", request.Kind)
	}
	if strings.TrimSpace(request.Target) == "" || strings.IndexByte(request.Target, 0) >= 0 {
		return fmt.Errorf("request target is empty or contains NUL")
	}
	return nil
}

func validateGuestBrowserURL(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("URL is not valid UTF-8")
	}
	if raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("URL contains leading or trailing whitespace")
	}
	for _, r := range raw {
		if unsafeGuestURLRune(r) {
			return "", fmt.Errorf("URL contains an unsafe Unicode character")
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("URL scheme %q is not allowed", parsed.Scheme)
	}
	if parsed.Hostname() == "" || parsed.Opaque != "" {
		return "", fmt.Errorf("URL has no network host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("URL credentials are not allowed")
	}
	for _, r := range parsed.Hostname() {
		if unsafeGuestURLRune(r) {
			return "", fmt.Errorf("URL host contains an unsafe Unicode character")
		}
	}
	// Preserve the guest's path escaping and fragment byte-for-byte. Rebuilding
	// with URL.String can turn an escaped slash into a path separator and a
	// fragment into an escaped path suffix, changing what the user approved.
	schemeEnd := strings.IndexByte(raw, ':')
	if schemeEnd < 0 {
		return "", fmt.Errorf("URL has no scheme separator")
	}
	return scheme + raw[schemeEnd:], nil
}

func unsafeGuestURLRune(r rune) bool {
	return unicode.IsControl(r) || unicode.IsSpace(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

func externalRequestPanel(s *Shell, request ExternalRequest, failure string) *Panel {
	panel := &Panel{
		Kind:  externalRequestPanelKind,
		Title: "External Request",
	}
	switch request.Kind {
	case ExternalRequestBrowser:
		parsed, _ := url.Parse(request.Target)
		panel.Title = "Open External Link"
		panel.Lines = []string{
			"The game wants to open a link in your browser.",
			s.trf("Host: %s", parsed.Host),
			s.trf("URL: %s", request.Target),
			"Open it outside ARAM?",
		}
		panel.Actions = []ToolAction{{
			ID: "open", Label: "Open Link", Enabled: true,
		}}
	case ExternalRequestPhone:
		panel.Lines = []string{
			"The game requested a phone call.",
			s.trf("Target: %s", request.Target),
			"Phone calls are not available from ARAM yet.",
		}
	case ExternalRequestSMS:
		panel.Lines = []string{
			"The game requested an SMS action.",
			s.trf("Target: %s", request.Target),
			"SMS actions are not available from ARAM yet.",
		}
	}
	if failure != "" {
		panel.Lines = append(panel.Lines, "", s.tr("The link could not be opened:"), failure)
	}
	return panel
}

func (s *Shell) executeExternalRequestAction(action string) {
	pending := s.pendingExternalRequest
	if pending == nil || pending.generation != s.frameGeneration {
		s.resetExternalRequestPrompt()
		return
	}
	backend, ok := s.backend.(ExternalRequestBackend)
	if !ok {
		s.setStatus(s.tr("External request backend is unavailable"))
		return
	}
	request := pending.request
	switch action {
	case "ignore":
		backend.ClearExternalRequests(request.Sequence)
		s.resetExternalRequestPrompt()
		s.setStatus(s.tr("External request ignored"))
	case "open":
		if request.Kind != ExternalRequestBrowser {
			return
		}
		opener := s.openGuestURL
		if opener == nil {
			opener = openPlatformURL
		}
		if err := invokeExternalURLOpener(opener, request.Target); err != nil {
			pending.failure = err.Error()
			s.panel = externalRequestPanel(s, request, err.Error())
			if s.interfaceUI != nil {
				s.interfaceUI.panelSignature = ""
			}
			s.appendLog(s.trf("External link: %s", err.Error()))
			s.setStatus(s.trf("External link: %s", err.Error()))
			return
		}
		backend.ClearExternalRequests(request.Sequence)
		s.resetExternalRequestPrompt()
		s.setStatus(s.tr("Opened external link"))
	}
}

func invokeExternalURLOpener(opener func(string) error, target string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("URL opener panicked: %v", recovered)
		}
	}()
	return opener(target)
}

func (s *Shell) resetExternalRequestPrompt() {
	s.pendingExternalRequest = nil
	if s.panel != nil && s.panel.Kind == externalRequestPanelKind {
		s.panel = nil
	}
	if s.interfaceUI != nil {
		s.interfaceUI.panelSignature = ""
	}
}
