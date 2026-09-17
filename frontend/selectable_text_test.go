package frontend

import "testing"

func TestSelectableTextReturnsDraggedSelection(t *testing.T) {
	view := &selectableText{
		value:  "first line\nsecond line",
		anchor: len("first "),
		caret:  len("first line\nsecond"),
	}
	if got, want := view.selectedText(), "line\nsecond"; got != want {
		t.Fatalf("selected text = %q, want %q", got, want)
	}

	view.anchor, view.caret = view.caret, view.anchor
	if got, want := view.selectedText(), "line\nsecond"; got != want {
		t.Fatalf("reverse selected text = %q, want %q", got, want)
	}
}

func TestLineStartOffsetsIncludeNewlines(t *testing.T) {
	got := lineStartOffsets([]string{"alpha", "", "한글"})
	want := []int{0, 6, 7}
	if len(got) != len(want) {
		t.Fatalf("line offsets = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("line offsets = %v, want %v", got, want)
		}
	}
}
