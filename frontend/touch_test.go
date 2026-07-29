package frontend

import "testing"

func TestTouchControlHitTargets(t *testing.T) {
	tests := []struct {
		x, y    int
		control string
	}{
		{x: 120, y: 530, control: "up"},
		{x: 60, y: 580, control: "left"},
		{x: 190, y: 580, control: "right"},
		{x: 120, y: 630, control: "down"},
		{x: 610, y: 600, control: "ok"},
	}
	for _, test := range tests {
		control, ok := touchControlAt(test.x, test.y)
		if !ok || control != test.control {
			t.Errorf("touchControlAt(%d, %d) = %q, %t", test.x, test.y, control, ok)
		}
	}
	if control, ok := touchControlAt(900, 300); ok {
		t.Fatalf("non-control point mapped to %q", control)
	}
}

func TestTouchNavigationMatchesPersistentMenus(t *testing.T) {
	buttons := touchNavigationButtons()
	menus := defaultMenus()
	if len(buttons) != len(menus) {
		t.Fatalf("touch navigation count = %d, menu count = %d", len(buttons), len(menus))
	}
	for index := range menus {
		if buttons[index].Label != menus[index].Label {
			t.Errorf("touch navigation %d = %q, want %q", index, buttons[index].Label, menus[index].Label)
		}
	}
}
