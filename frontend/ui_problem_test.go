package frontend

import (
	"strings"
	"testing"

	"github.com/ebitenui/ebitenui/widget"
)

func TestProblemSurfaceDetailsResetForAnotherFailure(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.problem = &FrontendProblem{State: FrontendMalformedInput, Input: "bad.dat", Reason: "invalid header"}
	shell.focusMode = true
	shell.touchChromeHidden = true
	if shell.focusModeActive() || shell.touchChromeHiddenActive() {
		t.Fatal("immersive playback hides the recovery UI after a failure")
	}
	view := shell.interfaceUI
	view.sync(shell)
	if view.problemContainer.GetWidget().GetVisibility() != widget.Visibility_Show || view.problemScroll == nil {
		t.Fatal("failure has no interactive recovery surface")
	}
	if view.problemExpanded {
		t.Fatal("technical details start expanded")
	}
	view.problemExpanded = true
	view.sync(shell)
	if !view.problemExpanded {
		t.Fatal("details collapsed during ordinary UI sync")
	}
	shell.problem = &FrontendProblem{State: FrontendUnsupportedProfile, Input: "other.dat", Reason: "unsupported carrier"}
	view.sync(shell)
	if view.problemExpanded {
		t.Fatal("a new failure inherited the old expanded details")
	}
	shell.problem = nil
	view.sync(shell)
	if view.problemContainer.GetWidget().GetVisibility() != widget.Visibility_Hide {
		t.Fatal("recovered title still shows the problem surface")
	}
}

func TestLateOpenProgressPreservesFailureStateAndStatus(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.loading = true
	shell.consumeBackendResult(backendResult{
		request: OpenRequest{DisplayName: "broken.dat"},
		err:     &BackendError{Kind: FailureMalformedInput, Reason: "invalid header"},
	})
	wantStatus := shell.status
	shell.openStageResults <- OpenStageLoading
	shell.consumeResults()
	if shell.state != FrontendMalformedInput || shell.status != wantStatus {
		t.Fatalf("late progress changed failure state/status: %s, %q", shell.state, shell.status)
	}
}

func TestProblemReportPrefillsReasonWithoutSubmitting(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	reason := strings.Repeat("long failure reason ", 12)
	shell.problem = &FrontendProblem{State: FrontendMalformedInput, Input: "broken.dat", Reason: reason}
	shell.openIssueTrackerForProblem()
	if shell.panel == nil || shell.panel.Kind != "issue-report" {
		t.Fatal("recovery action did not open the report form")
	}
	situation := shell.panel.FieldValues["situation"]
	if !strings.Contains(situation, "broken.dat") || !strings.Contains(situation, reason) {
		t.Fatalf("report lost the input or complete reason: %q", situation)
	}
	if shell.panel.Busy {
		t.Fatal("opening the recovery form started a submission")
	}
}
