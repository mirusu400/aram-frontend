package frontend

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func awaitFaultRequest(t *testing.T, shell *Shell) (string, bool) {
	t.Helper()
	select {
	case reason := <-shell.faultReportRequests:
		return reason, true
	case <-time.After(2 * time.Second):
		return "", false
	}
}

func TestPromptFaultReportOpensPrefilledReportOnConfirm(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.faultPrompter = stubCrashPrompter{confirm: true, available: true}
	shell.frameGeneration = 3
	shell.input = &InputInfo{DisplayName: "미니게임천국4.zip"}

	shell.promptFaultReport(&FrontendProblem{Reason: "java.bridge target is null"})

	reason, ok := awaitFaultRequest(t, shell)
	if !ok {
		t.Fatal("confirmed fault dialog routed no report request")
	}
	if reason != "java.bridge target is null" {
		t.Fatalf("routed reason = %q", reason)
	}

	shell.consumeFaultReportRequest(reason)
	if shell.panel == nil || shell.panel.Kind != "issue-report" {
		t.Fatalf("panel = %+v, want issue-report", shell.panel)
	}
	situation := shell.panel.FieldValues["situation"]
	if !strings.Contains(situation, "java.bridge target is null") {
		t.Fatalf("situation = %q, want the fault reason pre-filled", situation)
	}
}

func TestPromptFaultReportDeclinedRoutesNothing(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.faultPrompter = stubCrashPrompter{confirm: false, available: true}
	shell.frameGeneration = 1

	shell.promptFaultReport(&FrontendProblem{Reason: "boom"})

	if reason, ok := receiveFaultRequestSoon(shell); ok {
		t.Fatalf("declined dialog still routed a request (%q)", reason)
	}
}

func TestPromptFaultReportWithoutPrompterIsNoop(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.faultPrompter = nil
	shell.frameGeneration = 1

	shell.promptFaultReport(&FrontendProblem{Reason: "boom"})

	if reason, ok := receiveFaultRequestSoon(shell); ok {
		t.Fatalf("nil prompter still routed a request (%q)", reason)
	}
}

func TestPromptFaultReportPromptsOncePerGeneration(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.faultPrompter = stubCrashPrompter{confirm: true, available: true}
	shell.frameGeneration = 5

	shell.promptFaultReport(&FrontendProblem{Reason: "first"})
	if _, ok := awaitFaultRequest(t, shell); !ok {
		t.Fatal("first fault at a generation routed no request")
	}

	shell.promptFaultReport(&FrontendProblem{Reason: "second"})
	if reason, ok := receiveFaultRequestSoon(shell); ok {
		t.Fatalf("second fault in the same generation prompted again (%q)", reason)
	}

	// A reload bumps the generation and re-arms the dialog.
	shell.frameGeneration = 6
	shell.promptFaultReport(&FrontendProblem{Reason: "third"})
	if _, ok := awaitFaultRequest(t, shell); !ok {
		t.Fatal("fault after a reload routed no request")
	}
}

func TestFrameFaultResultPromptsAndOpensReport(t *testing.T) {
	shell := NewShell(NullBackend{}, nil, "")
	shell.faultPrompter = stubCrashPrompter{confirm: true, available: true}
	shell.input = &InputInfo{DisplayName: "2006독일축구.zip"}
	shell.frameGeneration = 4
	shell.frameRunPending = true

	shell.frameRunResults <- frameRunResult{
		generation: 4,
		err:        errors.New("execute KTF guest: java native target is null"),
	}
	shell.consumeResults()

	if shell.problem == nil || shell.state != FrontendGuestFaulted {
		t.Fatalf("fault result did not record a problem: state=%q problem=%+v",
			shell.state, shell.problem)
	}

	reason, ok := awaitFaultRequest(t, shell)
	if !ok {
		t.Fatal("frame fault raised no report request")
	}
	// Re-queue the routed reason so the next drain opens the panel, mirroring
	// the async goroutine hand-off in the running shell.
	shell.faultReportRequests <- reason
	shell.consumeResults()
	if shell.panel == nil || shell.panel.Kind != "issue-report" {
		t.Fatalf("panel = %+v, want issue-report", shell.panel)
	}
}

// receiveFaultRequestSoon waits briefly for a routed request, expecting none.
func receiveFaultRequestSoon(shell *Shell) (string, bool) {
	select {
	case reason := <-shell.faultReportRequests:
		return reason, true
	case <-time.After(200 * time.Millisecond):
		return "", false
	}
}
