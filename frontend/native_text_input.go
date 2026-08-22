package frontend

import (
	"strings"
	"sync"
	"unicode"
)

// Ebitengine exposes no soft keyboard on Android or iOS: exp/textinput has no
// backend there, and a mobile window never raises the system keyboard on its
// own. A text field would therefore stay empty forever, which made the issue
// report form unusable on a handset. A tap on a field instead asks the native
// host to present its own single line editor, so the platform IME - Hangul
// composition, clipboard, autocorrect - does the typing and hands the finished
// text back.
//
// Only one editor is presented at a time, so a single pending request is
// tracked rather than a queue. A host that never answers, for example an
// Activity destroyed while its dialog is open, is released by detaching the
// host.
type NativeTextInputHost interface {
	// RequestTextInput presents the platform text editor for one field. The
	// host must answer exactly once with SubmitNativeTextInput or
	// CancelNativeTextInput, quoting the same request ID.
	RequestTextInput(requestID int64, label, hint, text string)
}

type nativeTextInputAnswer struct {
	text      string
	submitted bool
}

var nativeTextInput struct {
	sync.Mutex
	host      NativeTextInputHost
	lastID    int64
	pendingID int64
	answerID  int64
	answer    nativeTextInputAnswer
}

// SetNativeTextInputHost attaches or detaches the platform text editor.
// Detaching cancels a request the host can no longer answer.
func SetNativeTextInputHost(host NativeTextInputHost) {
	nativeTextInput.Lock()
	defer nativeTextInput.Unlock()
	nativeTextInput.host = host
	if host != nil || nativeTextInput.pendingID == 0 {
		return
	}
	nativeTextInput.answerID = nativeTextInput.pendingID
	nativeTextInput.answer = nativeTextInputAnswer{}
	nativeTextInput.pendingID = 0
}

// requestNativeTextInput asks the host to edit value and reports the request
// ID to poll for. It reports 0 when no host is attached, which is every
// desktop build, and the caller then edits the field in place as before.
func requestNativeTextInput(label, hint, value string) int64 {
	nativeTextInput.Lock()
	host := nativeTextInput.host
	if host == nil {
		nativeTextInput.Unlock()
		return 0
	}
	nativeTextInput.lastID++
	requestID := nativeTextInput.lastID
	nativeTextInput.pendingID = requestID
	nativeTextInput.answerID = 0
	nativeTextInput.Unlock()

	// The host presents platform UI and may block on its own main thread, so
	// it is called without holding the bridge lock.
	host.RequestTextInput(requestID, label, hint, value)
	return requestID
}

// SubmitNativeTextInput reports the text the platform editor accepted.
func SubmitNativeTextInput(requestID int64, text string) {
	answerNativeTextInput(requestID, nativeTextInputAnswer{
		text:      singleLineText(text),
		submitted: true,
	})
}

// CancelNativeTextInput reports that the platform editor was dismissed
// without a change.
func CancelNativeTextInput(requestID int64) {
	answerNativeTextInput(requestID, nativeTextInputAnswer{})
}

func answerNativeTextInput(requestID int64, answer nativeTextInputAnswer) {
	nativeTextInput.Lock()
	defer nativeTextInput.Unlock()
	if requestID == 0 || requestID != nativeTextInput.pendingID {
		// A stale or duplicate answer, for example the dismissal that follows
		// the confirmation of the same dialog.
		return
	}
	nativeTextInput.pendingID = 0
	nativeTextInput.answerID = requestID
	nativeTextInput.answer = answer
}

// takeNativeTextInputAnswer reports a finished request exactly once. The
// widget polls it from Update, so the answer never crosses into the host
// thread that produced it.
func takeNativeTextInputAnswer(requestID int64) (nativeTextInputAnswer, bool) {
	nativeTextInput.Lock()
	defer nativeTextInput.Unlock()
	if requestID == 0 || requestID != nativeTextInput.answerID {
		return nativeTextInputAnswer{}, false
	}
	answer := nativeTextInput.answer
	nativeTextInput.answerID = 0
	nativeTextInput.answer = nativeTextInputAnswer{}
	return answer, true
}

// nativeTextInputPending reports whether the host still owes an answer for
// requestID.
func nativeTextInputPending(requestID int64) bool {
	nativeTextInput.Lock()
	defer nativeTextInput.Unlock()
	return requestID != 0 && requestID == nativeTextInput.pendingID
}

// singleLineText folds what a platform editor may return but a single line
// field cannot render. A line break becomes a space so a pasted log keeps its
// word boundaries; every other control rune is dropped.
func singleLineText(value string) string {
	if strings.IndexFunc(value, isFoldedControlRune) < 0 {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, symbol := range value {
		switch {
		case symbol == '\n' || symbol == '\r' || symbol == '\t':
			builder.WriteRune(' ')
		case isFoldedControlRune(symbol):
		default:
			builder.WriteRune(symbol)
		}
	}
	return builder.String()
}

func isFoldedControlRune(symbol rune) bool {
	return symbol != ' ' && unicode.IsControl(symbol)
}

/** Widget side **/

// startNativeEdit hands the field to the platform editor and reports whether
// it took over. A field already waiting for an answer keeps waiting.
func (t *imeTextInput) startNativeEdit() bool {
	if t.nativeRequest != 0 {
		return true
	}
	requestID := requestNativeTextInput(t.label, t.placeholder, t.field.Text())
	if requestID == 0 {
		return false
	}
	t.nativeRequest = requestID
	t.blink = 0
	return true
}

// adoptNativeEdit takes over the request of the field this one replaces. A
// soft keyboard resizes the window, which re-lays out and rebuilds the whole
// form, so the field that asked for the editor is usually gone by the time the
// answer arrives.
func (t *imeTextInput) adoptNativeEdit(previous *imeTextInput) {
	if previous == nil || previous.nativeRequest == 0 {
		return
	}
	t.nativeRequest = previous.nativeRequest
	previous.nativeRequest = 0
}

func (t *imeTextInput) applyNativeEdit() {
	answer, ok := takeNativeTextInputAnswer(t.nativeRequest)
	if !ok {
		if !nativeTextInputPending(t.nativeRequest) {
			// Another field superseded this request, so no answer is coming.
			// Releasing the field keeps it tappable instead of frozen.
			t.nativeRequest = 0
		}
		return
	}
	t.nativeRequest = 0
	if answer.submitted {
		t.setTextValue(answer.text)
	}
}
