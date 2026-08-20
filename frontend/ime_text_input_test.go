package frontend

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestRuneBoundariesCoverEveryCaretPosition(t *testing.T) {
	got := runeBoundaries("한a글")
	want := []int{0, 3, 4, 7}
	if len(got) != len(want) {
		t.Fatalf("rune boundaries = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("rune boundaries = %v, want %v", got, want)
		}
	}
	if boundaries := runeBoundaries(""); len(boundaries) != 1 ||
		boundaries[0] != 0 {
		t.Fatalf("empty rune boundaries = %v", boundaries)
	}
}

func TestRuneBoundaryNavigationStepsWholeRunes(t *testing.T) {
	const value = "한글"
	if got := previousRuneBoundary(value, len(value)); got != 3 {
		t.Fatalf("previousRuneBoundary = %d, want 3", got)
	}
	if got := previousRuneBoundary(value, 0); got != 0 {
		t.Fatalf("previousRuneBoundary at start = %d, want 0", got)
	}
	if got := nextRuneBoundary(value, 0); got != 3 {
		t.Fatalf("nextRuneBoundary = %d, want 3", got)
	}
	if got := nextRuneBoundary(value, len(value)); got != len(value) {
		t.Fatalf("nextRuneBoundary at end = %d, want %d", got, len(value))
	}
}

func TestByteIndexAtAdvanceSnapsToNearestRune(t *testing.T) {
	face := newARAMDesignSystem("light", themeFamilyModern).Type.Body
	const value = "한글 입력"
	for _, boundary := range runeBoundaries(value) {
		offset := text.Advance(value[:boundary], *face)
		if got := byteIndexAtAdvance(value, face, offset); got != boundary {
			t.Fatalf(
				"byteIndexAtAdvance(%.2f) = %d, want %d",
				offset,
				got,
				boundary,
			)
		}
	}
	if got := byteIndexAtAdvance(value, face, -12); got != 0 {
		t.Fatalf("byteIndexAtAdvance before the text = %d, want 0", got)
	}
	full := text.Advance(value, *face)
	if got := byteIndexAtAdvance(value, face, full+240); got != len(value) {
		t.Fatalf("byteIndexAtAdvance past the text = %d", got)
	}
}

func TestIMETextInputEditsWholeRunes(t *testing.T) {
	changes := []string{}
	field := newIMETextInput(newARAMDesignSystem("light", themeFamilyModern), imeTextInputConfig{
		Changed: func(value string) { changes = append(changes, value) },
	})
	field.SetText("한글")

	field.deleteBackward()
	if got := field.GetText(); got != "한" {
		t.Fatalf("text after backspace = %q, want %q", got, "한")
	}
	field.moveCaret(0, false)
	field.deleteForward()
	if got := field.GetText(); got != "" {
		t.Fatalf("text after delete = %q, want empty", got)
	}

	field.SetText("에픽")
	field.moveCaret(0, false)
	field.moveCaret(nextRuneBoundary(field.GetText(), 0), true)
	field.replaceSelection("")
	if got := field.GetText(); got != "픽" {
		t.Fatalf("text after selection delete = %q, want %q", got, "픽")
	}

	field.notifyChanged()
	if len(changes) != 1 || changes[0] != "픽" {
		t.Fatalf("change notifications = %v", changes)
	}
}

func TestIssueReportFormUsesIMETextFields(t *testing.T) {
	isolateSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.openIssueTracker()
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(logicalWidth, logicalHeight))

	situation, ok := shell.interfaceUI.panelTextInputs["situation"]
	if !ok {
		t.Fatalf(
			"issue report form has no IME text field: %v",
			shell.interfaceUI.panelTextInputs,
		)
	}

	// A committed IME composition lands in the field the same way this
	// replacement does; the panel must pick the text up from there.
	const composed = "에픽 크로니클이 멈춥니다"
	situation.replaceSelection(composed)
	situation.notifyChanged()
	if got := shell.panel.FieldValues["situation"]; got != composed {
		t.Fatalf("situation field value = %q, want %q", got, composed)
	}

	shell.interfaceUI.ui.Update()
	shell.Draw(ebiten.NewImage(logicalWidth, logicalHeight))
	if got := situation.GetText(); got != composed {
		t.Fatalf("situation text after redraw = %q, want %q", got, composed)
	}
}
