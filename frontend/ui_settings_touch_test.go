package frontend

import "testing"

func TestTouchScrollDragRequiresDominantVerticalSlop(t *testing.T) {
	const slop = 8
	for _, test := range []struct {
		name     string
		dx, dy   int
		wantDrag bool
	}{
		{name: "tap wobble", dx: 2, dy: 7},
		{name: "horizontal slider", dx: 20, dy: 9},
		{name: "vertical down", dx: 2, dy: 8, wantDrag: true},
		{name: "vertical up", dx: -3, dy: -12, wantDrag: true},
		{name: "diagonal tie", dx: 10, dy: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := touchScrollDragStarted(test.dx, test.dy, slop); got != test.wantDrag {
				t.Fatalf("touchScrollDragStarted(%d,%d,%d) = %v; want %v",
					test.dx, test.dy, slop, got, test.wantDrag)
			}
		})
	}
}
