package frontend

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

type recordingTextInputHost struct {
	requests []nativeTextInputRequest
	answer   func(requestID int64)
}

type nativeTextInputRequest struct {
	requestID int64
	label     string
	hint      string
	text      string
}

func (h *recordingTextInputHost) RequestTextInput(
	requestID int64,
	label, hint, text string,
) {
	h.requests = append(h.requests, nativeTextInputRequest{
		requestID: requestID,
		label:     label,
		hint:      hint,
		text:      text,
	})
	if h.answer != nil {
		h.answer(requestID)
	}
}

func attachTextInputHost(t *testing.T, host NativeTextInputHost) {
	t.Helper()
	SetNativeTextInputHost(host)
	t.Cleanup(func() { SetNativeTextInputHost(nil) })
}

func TestTextFieldEditsInPlaceWithoutANativeHost(t *testing.T) {
	field := newIMETextInput(
		newARAMDesignSystem("light", themeFamilyModern),
		imeTextInputConfig{Label: "Carrier"},
	)
	if field.startNativeEdit() {
		t.Fatal("a desktop build delegated text entry to a native host")
	}
	if field.nativeRequest != 0 {
		t.Fatalf("pending native request = %d, want 0", field.nativeRequest)
	}
}

func TestNativeTextInputCarriesTheFieldAndAnswersOnce(t *testing.T) {
	host := &recordingTextInputHost{}
	attachTextInputHost(t, host)

	field := newIMETextInput(
		newARAMDesignSystem("light", themeFamilyModern),
		imeTextInputConfig{
			Label:       "What happened?",
			Placeholder: "Describe the problem",
		},
	)
	field.SetText("튕김")

	if !field.startNativeEdit() {
		t.Fatal("the native host did not take the field")
	}
	if len(host.requests) != 1 {
		t.Fatalf("host requests = %v", host.requests)
	}
	request := host.requests[0]
	if request.label != "What happened?" ||
		request.hint != "Describe the problem" ||
		request.text != "튕김" {
		t.Fatalf("native request = %+v", request)
	}
	if field.startNativeEdit(); len(host.requests) != 1 {
		t.Fatalf("a waiting field asked again: %v", host.requests)
	}

	SubmitNativeTextInput(request.requestID, "게임이 튕겨요")
	field.applyNativeEdit()
	if got := field.GetText(); got != "게임이 튕겨요" {
		t.Fatalf("text after the native editor = %q", got)
	}
	if field.nativeRequest != 0 {
		t.Fatalf("pending native request = %d, want 0", field.nativeRequest)
	}

	// The dismissal that follows a confirmed dialog must not wipe the text.
	CancelNativeTextInput(request.requestID)
	field.applyNativeEdit()
	if got := field.GetText(); got != "게임이 튕겨요" {
		t.Fatalf("text after a stale cancel = %q", got)
	}
}

func TestCanceledNativeTextInputKeepsTheFieldUnchanged(t *testing.T) {
	host := &recordingTextInputHost{
		answer: func(requestID int64) { CancelNativeTextInput(requestID) },
	}
	attachTextInputHost(t, host)

	field := newIMETextInput(
		newARAMDesignSystem("light", themeFamilyModern),
		imeTextInputConfig{Label: "Carrier"},
	)
	field.SetText("SKT")
	if !field.startNativeEdit() {
		t.Fatal("the native host did not take the field")
	}
	field.applyNativeEdit()
	if got := field.GetText(); got != "SKT" {
		t.Fatalf("text after a canceled editor = %q, want %q", got, "SKT")
	}
	if field.nativeRequest != 0 {
		t.Fatalf("pending native request = %d, want 0", field.nativeRequest)
	}
}

func TestDetachingTheHostReleasesAWaitingField(t *testing.T) {
	attachTextInputHost(t, &recordingTextInputHost{})
	field := newIMETextInput(
		newARAMDesignSystem("light", themeFamilyModern),
		imeTextInputConfig{Label: "Carrier"},
	)
	field.SetText("KTF")
	if !field.startNativeEdit() {
		t.Fatal("the native host did not take the field")
	}

	// An Activity destroyed while its dialog is open never answers.
	SetNativeTextInputHost(nil)
	field.applyNativeEdit()
	if field.nativeRequest != 0 {
		t.Fatalf("pending native request = %d, want 0", field.nativeRequest)
	}
	if got := field.GetText(); got != "KTF" {
		t.Fatalf("text after a released field = %q, want %q", got, "KTF")
	}
}

func TestNativeTextInputFoldsWhatOneLineCannotShow(t *testing.T) {
	host := &recordingTextInputHost{
		answer: func(requestID int64) {
			SubmitNativeTextInput(
				requestID,
				"첫 줄\r\n둘째 줄\tTAB\x07",
			)
		},
	}
	attachTextInputHost(t, host)

	field := newIMETextInput(
		newARAMDesignSystem("light", themeFamilyModern),
		imeTextInputConfig{Label: "What happened?"},
	)
	if !field.startNativeEdit() {
		t.Fatal("the native host did not take the field")
	}
	field.applyNativeEdit()
	if got := field.GetText(); got != "첫 줄  둘째 줄 TAB" {
		t.Fatalf("folded text = %q", got)
	}
}

// The issue report form is the reason this bridge exists: on a handset every
// one of its text fields must reach the platform keyboard, and the answer must
// land back in the submitted values.
func TestIssueReportFieldsReachTheNativeKeyboard(t *testing.T) {
	isolateSettings(t)
	host := &recordingTextInputHost{
		answer: func(requestID int64) {
			SubmitNativeTextInput(requestID, "게임이 튕겨요")
		},
	}
	attachTextInputHost(t, host)

	shell := NewShell(NullBackend{}, nil, "")
	shell.openIssueTracker()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(logicalWidth, logicalHeight))

	situation, ok := shell.interfaceUI.panelTextInputs["situation"]
	if !ok {
		t.Fatalf(
			"issue report form has no text field: %v",
			shell.interfaceUI.panelTextInputs,
		)
	}
	if !situation.startNativeEdit() {
		t.Fatal("the issue report field did not reach the native keyboard")
	}
	if len(host.requests) != 1 || host.requests[0].label == "" {
		t.Fatalf("native request = %v", host.requests)
	}

	shell.interfaceUI.ui.Update()
	if got := shell.panel.FieldValues["situation"]; got != "게임이 튕겨요" {
		t.Fatalf("submitted situation = %q", got)
	}
}

func TestASupersededFieldDoesNotStayFrozen(t *testing.T) {
	attachTextInputHost(t, &recordingTextInputHost{})
	design := newARAMDesignSystem("light", themeFamilyModern)
	first := newIMETextInput(design, imeTextInputConfig{Label: "Carrier"})
	second := newIMETextInput(design, imeTextInputConfig{Label: "Game title"})

	if !first.startNativeEdit() || !second.startNativeEdit() {
		t.Fatal("the native host did not take both fields")
	}
	first.applyNativeEdit()
	if first.nativeRequest != 0 {
		t.Fatalf("superseded field still waits on %d", first.nativeRequest)
	}
	if second.nativeRequest == 0 {
		t.Fatal("the newest field lost its request")
	}
}

// A soft keyboard resizes the window, which re-lays out the form and rebuilds
// every field widget while the platform editor is still open. The answer must
// still reach the field that asked for it.
func TestAResizeWhileTypingKeepsTheAnswer(t *testing.T) {
	isolateSettings(t)
	var pending int64
	host := &recordingTextInputHost{
		answer: func(requestID int64) { pending = requestID },
	}
	attachTextInputHost(t, host)

	shell := NewShell(NullBackend{}, nil, "")
	shell.Layout(logicalWidth, logicalHeight)
	shell.openIssueTracker()
	shell.interfaceUI.sync(shell)

	asking := shell.interfaceUI.panelTextInputs["situation"]
	if !asking.startNativeEdit() {
		t.Fatal("the issue report field did not reach the native keyboard")
	}

	shell.Layout(logicalWidth, logicalHeight/2)
	shell.interfaceUI.sync(shell)
	rebuilt := shell.interfaceUI.panelTextInputs["situation"]
	if rebuilt == asking {
		t.Fatal("the resize did not rebuild the form")
	}
	if asking.nativeRequest != 0 {
		t.Fatalf("the replaced field still waits on %d", asking.nativeRequest)
	}
	if rebuilt.nativeRequest != pending {
		t.Fatalf(
			"rebuilt field waits on %d, want %d",
			rebuilt.nativeRequest,
			pending,
		)
	}

	SubmitNativeTextInput(pending, "게임이 튕겨요")
	shell.interfaceUI.ui.Update()
	if got := shell.panel.FieldValues["situation"]; got != "게임이 튕겨요" {
		t.Fatalf("submitted situation = %q", got)
	}
}
