package frontend

// TextInputBackend reports that the guest currently owns a focused handset
// text field. Presentation uses the hint to reveal phone-key input controls;
// the backend remains responsible for the guest's composition rules.
type TextInputBackend interface {
	TextInputActive() bool
}
