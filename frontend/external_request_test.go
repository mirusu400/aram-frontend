package frontend

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

type externalRequestTestBackend struct {
	NullBackend
	requests []ExternalRequest
	cleared  []uint64
}

func (backend *externalRequestTestBackend) ExternalRequests() []ExternalRequest {
	result := append([]ExternalRequest(nil), backend.requests...)
	for index := range result {
		result[index].Data = append([]byte(nil), result[index].Data...)
	}
	return result
}

func (backend *externalRequestTestBackend) ClearExternalRequests(through uint64) {
	backend.cleared = append(backend.cleared, through)
	for len(backend.requests) != 0 && backend.requests[0].Sequence <= through {
		backend.requests = backend.requests[1:]
	}
}

func externalRequestTestShell(backend Backend) *Shell {
	return &Shell{
		backend: backend,
		input:   &InputInfo{DisplayName: "guest"},
		settings: Settings{
			Language: string(LanguageEnglish),
		},
	}
}

func TestExternalBrowserRequestRequiresConfirmationAndAcknowledgesSuccess(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 4,
		GuestNS:  10,
		Kind:     ExternalRequestBrowser,
		Target:   "HTTPS://example.invalid/play?from=guest",
	}}}
	shell := externalRequestTestShell(backend)
	var opened string
	shell.openGuestURL = func(rawURL string) error {
		opened = rawURL
		return nil
	}
	shell.syncExternalRequests()
	if shell.panel == nil || shell.panel.Kind != externalRequestPanelKind {
		t.Fatalf("confirmation panel = %+v", shell.panel)
	}
	if len(backend.cleared) != 0 || opened != "" {
		t.Fatalf("request acted before confirmation: opened=%q cleared=%v", opened, backend.cleared)
	}

	shell.executeExternalRequestAction("open")
	if opened != "https://example.invalid/play?from=guest" {
		t.Fatalf("opened URL = %q", opened)
	}
	if len(backend.cleared) != 1 || backend.cleared[0] != 4 {
		t.Fatalf("acknowledgements = %v", backend.cleared)
	}
	if shell.pendingExternalRequest != nil || shell.panel != nil {
		t.Fatal("successful request left its confirmation open")
	}
}

func TestExternalBrowserOpenFailureCanRetryOrIgnore(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 9,
		Kind:     ExternalRequestBrowser,
		Target:   "https://example.invalid/fail",
	}}}
	shell := externalRequestTestShell(backend)
	shell.openGuestURL = func(string) error { return errors.New("no browser") }
	shell.syncExternalRequests()
	shell.executeExternalRequestAction("open")
	if len(backend.cleared) != 0 || shell.pendingExternalRequest == nil || shell.panel == nil {
		t.Fatal("failed open acknowledged or lost the pending request")
	}
	if joined := strings.Join(shell.panel.Lines, "\n"); !strings.Contains(joined, "no browser") {
		t.Fatalf("failure was not shown in the panel: %q", joined)
	}

	shell.executeExternalRequestAction("ignore")
	if len(backend.cleared) != 1 || backend.cleared[0] != 9 {
		t.Fatalf("ignore acknowledgements = %v", backend.cleared)
	}
}

func TestExternalBrowserFailureSurvivesPanelAndDialogReplacement(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 10,
		Kind:     ExternalRequestBrowser,
		Target:   "https://example.invalid/retry",
	}}}
	shell := externalRequestTestShell(backend)
	attempts := 0
	shell.openGuestURL = func(string) error {
		attempts++
		if attempts == 1 {
			return errors.New("popup denied")
		}
		return nil
	}
	shell.syncExternalRequests()
	shell.executeExternalRequestAction("open")
	shell.panel = &Panel{Kind: "settings"}
	shell.syncExternalRequests()
	if shell.panel.Kind != "settings" {
		t.Fatalf("pending request replaced another panel: %+v", shell.panel)
	}
	shell.panel = nil
	shell.dialogOpen = true
	shell.syncExternalRequests()
	if shell.panel != nil {
		t.Fatalf("pending request appeared over a native dialog: %+v", shell.panel)
	}
	shell.dialogOpen = false
	shell.syncExternalRequests()
	if shell.panel == nil || !strings.Contains(strings.Join(shell.panel.Lines, "\n"), "popup denied") {
		t.Fatalf("re-presented panel lost the open failure: %+v", shell.panel)
	}
	if len(backend.cleared) != 0 {
		t.Fatalf("failed/re-presented request was acknowledged: %v", backend.cleared)
	}

	shell.executeExternalRequestAction("open")
	if attempts != 2 || len(backend.cleared) != 1 || backend.cleared[0] != 10 {
		t.Fatalf("retry result: attempts=%d acknowledgements=%v", attempts, backend.cleared)
	}
}

func TestExternalBrowserOpenerPanicDoesNotAcknowledge(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 11,
		Kind:     ExternalRequestBrowser,
		Target:   "https://example.invalid/panic",
	}}}
	shell := externalRequestTestShell(backend)
	shell.openGuestURL = func(string) error { panic("host bridge failed") }
	shell.syncExternalRequests()
	shell.executeExternalRequestAction("open")

	if len(backend.cleared) != 0 || shell.pendingExternalRequest == nil || shell.panel == nil {
		t.Fatalf("opener panic acknowledged or lost request: cleared=%v pending=%+v panel=%+v", backend.cleared, shell.pendingExternalRequest, shell.panel)
	}
	if joined := strings.Join(shell.panel.Lines, "\n"); !strings.Contains(joined, "host bridge failed") {
		t.Fatalf("opener panic was not reported: %q", joined)
	}
}

func TestPendingExternalRequestPanelIsRePresented(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 12,
		Kind:     ExternalRequestBrowser,
		Target:   "https://example.invalid/pending",
	}}}
	shell := externalRequestTestShell(backend)
	shell.syncExternalRequests()
	shell.panel = nil

	shell.syncExternalRequests()
	if shell.panel == nil || shell.panel.Kind != externalRequestPanelKind {
		t.Fatalf("pending confirmation was not restored: %+v", shell.panel)
	}
	if len(backend.cleared) != 0 {
		t.Fatalf("restoring confirmation acknowledged request: %v", backend.cleared)
	}
}

func TestExternalRequestFromPriorGenerationCannotOpenOrAcknowledge(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 13,
		Kind:     ExternalRequestBrowser,
		Target:   "https://example.invalid/old-session",
	}}}
	shell := externalRequestTestShell(backend)
	opened := false
	shell.openGuestURL = func(string) error {
		opened = true
		return nil
	}
	shell.syncExternalRequests()
	shell.frameGeneration++

	shell.executeExternalRequestAction("open")
	if opened {
		t.Fatal("request from an earlier title generation was opened")
	}
	if len(backend.cleared) != 0 {
		t.Fatalf("request from an earlier title generation was acknowledged: %v", backend.cleared)
	}
	if shell.pendingExternalRequest != nil || shell.panel != nil {
		t.Fatal("request from an earlier title generation remained visible")
	}
}

func TestExternalRequestGenerationTransitionPresentsOnlyNewOutbox(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 13,
		Kind:     ExternalRequestBrowser,
		Target:   "https://old.example.invalid/",
	}}}
	shell := externalRequestTestShell(backend)
	shell.syncExternalRequests()
	shell.frameGeneration++
	backend.requests = []ExternalRequest{{
		Sequence: 1,
		Kind:     ExternalRequestBrowser,
		Target:   "https://new.example.invalid/",
	}}

	shell.syncExternalRequests()
	if shell.pendingExternalRequest != nil || shell.panel != nil {
		t.Fatal("generation transition retained the previous prompt")
	}
	if len(backend.cleared) != 0 {
		t.Fatalf("generation transition acknowledged the previous outbox: %v", backend.cleared)
	}
	shell.syncExternalRequests()
	if shell.pendingExternalRequest == nil || shell.pendingExternalRequest.request.Sequence != 1 ||
		shell.pendingExternalRequest.request.Target != "https://new.example.invalid/" {
		t.Fatalf("new generation prompt = %+v", shell.pendingExternalRequest)
	}
}

func TestExternalRequestsRemainFIFOAcrossAcceptAndIgnore(t *testing.T) {
	backend := &externalRequestTestBackend{requests: []ExternalRequest{
		{Sequence: 20, Kind: ExternalRequestBrowser, Target: "https://one.example.invalid/"},
		{Sequence: 21, Kind: ExternalRequestBrowser, Target: "https://two.example.invalid/"},
		{Sequence: 22, Kind: ExternalRequestBrowser, Target: "https://three.example.invalid/"},
	}}
	shell := externalRequestTestShell(backend)
	var opened []string
	shell.openGuestURL = func(target string) error {
		opened = append(opened, target)
		return nil
	}

	shell.syncExternalRequests()
	if got := shell.pendingExternalRequest.request.Sequence; got != 20 {
		t.Fatalf("first pending sequence = %d", got)
	}
	shell.executeExternalRequestAction("open")
	if len(opened) != 1 || opened[0] != "https://one.example.invalid/" {
		t.Fatalf("opened = %v", opened)
	}
	shell.syncExternalRequests()
	if got := shell.pendingExternalRequest.request.Sequence; got != 21 {
		t.Fatalf("second pending sequence = %d", got)
	}
	shell.executeExternalRequestAction("ignore")
	shell.syncExternalRequests()
	if got := shell.pendingExternalRequest.request.Sequence; got != 22 {
		t.Fatalf("third pending sequence = %d", got)
	}
	if len(opened) != 1 {
		t.Fatalf("ignored request was opened: %v", opened)
	}
	if got := backend.cleared; len(got) != 2 || got[0] != 20 || got[1] != 21 {
		t.Fatalf("FIFO acknowledgements = %v", got)
	}
}

func TestMalformedExternalRequestsAreRejectedAndAcknowledged(t *testing.T) {
	cases := []struct {
		name    string
		request ExternalRequest
	}{
		{name: "zero sequence", request: ExternalRequest{Kind: ExternalRequestBrowser, Target: "https://example.invalid/"}},
		{name: "negative timestamp", request: ExternalRequest{Sequence: 2, GuestNS: -1, Kind: ExternalRequestBrowser, Target: "https://example.invalid/"}},
		{name: "unknown kind", request: ExternalRequest{Sequence: 3, Kind: "share", Target: "https://example.invalid/"}},
		{name: "oversize target", request: ExternalRequest{Sequence: 4, Kind: ExternalRequestBrowser, Target: strings.Repeat("x", 4097)}},
		{name: "oversize data", request: ExternalRequest{Sequence: 5, Kind: ExternalRequestSMS, Target: "123", Data: make([]byte, 4097)}},
		{name: "empty target", request: ExternalRequest{Sequence: 6, Kind: ExternalRequestPhone, Target: " \t"}},
		{name: "NUL target", request: ExternalRequest{Sequence: 7, Kind: ExternalRequestSMS, Target: "123\x00456"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			backend := &externalRequestTestBackend{requests: []ExternalRequest{test.request}}
			shell := externalRequestTestShell(backend)
			opened := false
			shell.openGuestURL = func(string) error { opened = true; return nil }
			shell.syncExternalRequests()
			if opened || shell.pendingExternalRequest != nil || shell.panel != nil {
				t.Fatalf("malformed request was presented/opened: opened=%t pending=%+v panel=%+v", opened, shell.pendingExternalRequest, shell.panel)
			}
			if len(backend.cleared) != 1 || backend.cleared[0] != test.request.Sequence {
				t.Fatalf("malformed request acknowledgements = %v", backend.cleared)
			}
		})
	}
}

func TestExternalBrowserRejectsUnsafeURLWithoutOpeningIt(t *testing.T) {
	for _, target := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"//example.invalid/protocol-relative",
		"https://example.invalid\\@attacker.invalid/",
		"https://user:secret@example.invalid/",
		"https://user%40example.invalid@attacker.invalid/",
		"https://example.invalid:invalid/",
		"https://\u202eexample.invalid/",
		"https://example\u200b.invalid/",
		"https://example\u00a0.invalid/",
		"https://%E2%80%AEexample.invalid/",
		" https://example.invalid/",
		"https://example.invalid/\r\nHost:%20attacker.invalid",
		"https://example.invalid/\xff",
	} {
		t.Run(target, func(t *testing.T) {
			backend := &externalRequestTestBackend{requests: []ExternalRequest{{
				Sequence: 1,
				Kind:     ExternalRequestBrowser,
				Target:   target,
			}}}
			shell := externalRequestTestShell(backend)
			called := false
			shell.openGuestURL = func(string) error { called = true; return nil }
			shell.syncExternalRequests()
			if called || shell.panel != nil || shell.pendingExternalRequest != nil {
				t.Fatalf("unsafe URL reached confirmation/opener: called=%t panel=%+v", called, shell.panel)
			}
			if len(backend.cleared) != 1 || backend.cleared[0] != 1 {
				t.Fatalf("unsafe URL acknowledgements = %v", backend.cleared)
			}
		})
	}
}

func TestExternalBrowserPanelShowsParsedHostBeforeEscapedAuthorityText(t *testing.T) {
	target := "https://trusted.example.invalid/%2F%2Fattacker.example.invalid/path#@other.example.invalid"
	backend := &externalRequestTestBackend{requests: []ExternalRequest{{
		Sequence: 30,
		Kind:     ExternalRequestBrowser,
		Target:   target,
	}}}
	shell := externalRequestTestShell(backend)
	shell.syncExternalRequests()
	if shell.panel == nil || len(shell.panel.Lines) < 3 {
		t.Fatalf("confirmation panel = %+v", shell.panel)
	}
	if got := shell.panel.Lines[1]; got != "Host: trusted.example.invalid" {
		t.Fatalf("displayed host = %q", got)
	}
	if got := shell.panel.Lines[2]; got != "URL: "+target {
		t.Fatalf("displayed URL = %q", got)
	}
}

func TestExternalBrowserURLPreservesUnicodePathAndFragment(t *testing.T) {
	raw := "HTTPS://예시.invalid/경로/%2F?q=한글#조각"
	got, err := validateGuestBrowserURL(raw)
	if err != nil {
		t.Fatalf("validate Unicode URL: %v", err)
	}
	want := "https://예시.invalid/경로/%2F?q=한글#조각"
	if got != want {
		t.Fatalf("normalized URL = %q, want %q", got, want)
	}
}

func FuzzValidateGuestBrowserURL(f *testing.F) {
	for _, seed := range []string{
		"https://example.invalid/",
		"HTTP://localhost:8080/path?q=x#fragment",
		"javascript:alert(1)",
		"https://user@example.invalid/",
		"https://\u202eexample.invalid/",
		"https://example.invalid\\@attacker.invalid/",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		normalized, err := validateGuestBrowserURL(raw)
		if err != nil {
			return
		}
		parsed, err := url.Parse(normalized)
		if err != nil {
			t.Fatalf("accepted URL did not round-trip: %q: %v", normalized, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			t.Fatalf("accepted scheme = %q", parsed.Scheme)
		}
		if parsed.Hostname() == "" || parsed.Opaque != "" || parsed.User != nil {
			t.Fatalf("accepted unsafe authority: %+v", parsed)
		}
		schemeEnd := strings.IndexByte(raw, ':')
		if schemeEnd < 0 || normalized != strings.ToLower(raw[:schemeEnd])+raw[schemeEnd:] {
			t.Fatalf("accepted URL was semantically rewritten: raw=%q normalized=%q", raw, normalized)
		}
		if repeated, err := validateGuestBrowserURL(normalized); err != nil || repeated != normalized {
			t.Fatalf("normalization is not idempotent: normalized=%q repeated=%q err=%v", normalized, repeated, err)
		}
		for _, r := range normalized {
			if unsafeGuestURLRune(r) {
				t.Fatalf("accepted unsafe URL rune %U in %q", r, normalized)
			}
		}
		for _, r := range parsed.Hostname() {
			if unsafeGuestURLRune(r) {
				t.Fatalf("accepted unsafe host rune %U in %q", r, normalized)
			}
		}
	})
}

var _ ExternalRequestBackend = (*externalRequestTestBackend)(nil)
