package frontend

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// cheatToolBackend answers the Cheat Manager with one self-applying toggle per
// cheat, the shape the integration adapter publishes.
type cheatToolBackend struct {
	NullBackend
	enabled  bool
	requests chan ToolRequest
}

func (backend *cheatToolBackend) snapshot() ToolSnapshot {
	state := "false"
	if backend.enabled {
		state = "true"
	}
	return ToolSnapshot{
		Title: "Cheat Manager",
		Lines: []string{"Title: Synthetic"},
		Fields: []ToolField{{
			ID:       "cheat.skip-server-authentication",
			Label:    "Skip server authentication",
			Detail:   "Starts the game without the authentication server.",
			Value:    state,
			Checkbox: true,
			Action:   "toggle",
		}},
		Actions: []ToolAction{{ID: "refresh", Label: "Update", Enabled: true}},
	}
}

func (backend *cheatToolBackend) ToolSnapshot(
	context.Context,
	ToolKind,
) (ToolSnapshot, error) {
	return backend.snapshot(), nil
}

func (backend *cheatToolBackend) ExecuteToolAction(
	_ context.Context,
	request ToolRequest,
) (ToolSnapshot, error) {
	backend.requests <- request
	if request.Action == "toggle" {
		backend.enabled = strings.EqualFold(
			request.Fields["cheat.skip-server-authentication"],
			"true",
		)
	}
	return backend.snapshot(), nil
}

func TestCheatTogglesApplyThroughTheBackendBoundary(t *testing.T) {
	backend := &cheatToolBackend{requests: make(chan ToolRequest, 1)}
	shell := NewShell(backend, nil, "")
	shell.openToolPanel(ToolCheats)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))

	if len(shell.panel.Fields) != 1 {
		t.Fatalf("cheat panel = %#v", shell.panel)
	}
	field := shell.panel.Fields[0]
	if !field.Checkbox || field.Action != "toggle" || field.Detail == "" {
		t.Fatalf("cheat toggle = %#v", field)
	}
	if shell.panel.FieldValues[field.ID] != "false" {
		t.Fatalf("initial toggle state = %q", shell.panel.FieldValues[field.ID])
	}

	// Flipping the control is what a self-applying field does on change.
	shell.panel.FieldValues[field.ID] = "true"
	shell.executeToolAction(field.Action, shell.panel.FieldValues)
	request := <-backend.requests
	if request.Kind != ToolCheats || request.Action != "toggle" ||
		request.Fields[field.ID] != "true" {
		t.Fatalf("tool request = %#v", request)
	}
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))

	if !backend.enabled {
		t.Fatal("the backend did not enable the cheat")
	}
	if shell.panel.Busy || shell.panel.FieldValues[field.ID] != "true" {
		t.Fatalf("cheat panel after toggle = %#v", shell.panel)
	}
}

func issueDraftBody(t *testing.T, input InputInfo) string {
	t.Helper()
	draft := issueReportDraft{
		Situation:  "cheat needed",
		Repository: "aram-emu",
	}
	raw, err := buildIssueDraftURL(draft, &input, "aram-core", FrontendReady, "bundle.zip", "")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestIssueDiagnosticsCarryTheImageIdentity(t *testing.T) {
	body := issueDraftBody(t, InputInfo{
		DisplayName: "synthetic.zip",
		Format:      "raptor-wipi-c",
		SHA256:      strings.Repeat("ab", 32),
		ImageSHA256: strings.Repeat("cd", 32),
	})
	if !strings.Contains(body, "Image SHA-256: `"+strings.Repeat("cd", 32)+"`") {
		t.Fatalf("issue body = %q", body)
	}

	withoutImage := issueDraftBody(t, InputInfo{
		DisplayName: "synthetic.zip",
		SHA256:      strings.Repeat("ab", 32),
	})
	if strings.Contains(withoutImage, "Image SHA-256") {
		t.Fatalf("issue body named an absent image identity = %q", withoutImage)
	}
}
