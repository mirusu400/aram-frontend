package frontend

import (
	"testing"
)

// The workspace, the guest viewport, and the touch chrome toggle all lay
// themselves out against the bar height constants rather than measuring the
// EbitenUI bars. A bar whose contents need more room than its constant would
// grow past that and overlap the viewport it is supposed to sit above, so
// every skin's bars have to fit what they seat.
func TestChromeBarsFitInsideTheHeightsTheWorkspaceAssumes(t *testing.T) {
	for _, family := range themeFamilyChoices() {
		for _, mode := range []string{"light", "dark"} {
			shell := NewShell(NullBackend{}, nil, "")
			shell.settings.ThemeMode = mode
			shell.settings.ThemeFamily = family
			shell.syncDesignSystem()
			view := shell.interfaceUI

			_, menuHeight := view.buildTopBar(shell).PreferredSize()
			if menuHeight > menuBarHeight {
				t.Fatalf("%s/%s menu bar needs %d, constant is %d",
					family, mode, menuHeight, menuBarHeight)
			}
			_, toolbarHeight := view.buildApplicationToolbar(shell).PreferredSize()
			if toolbarHeight > applicationToolbarHeight {
				t.Fatalf("%s/%s toolbar needs %d, constant is %d",
					family, mode, toolbarHeight, applicationToolbarHeight)
			}
			_, statusHeight := view.buildStatusBar().PreferredSize()
			if statusHeight > statusBarHeight {
				t.Fatalf("%s/%s status bar needs %d, constant is %d",
					family, mode, statusHeight, statusBarHeight)
			}
		}
	}
}

// The top chrome is a desktop menu row over an icon toolbar, not the
// touch-sized rows a handset deck uses. Its budget is stated here so a control
// grown for a phone cannot quietly take the guest screen's room back.
func TestTopChromeStaysDesktopSized(t *testing.T) {
	const topChromeBudget = 64
	if total := menuBarHeight + applicationToolbarHeight; total > topChromeBudget {
		t.Fatalf("top chrome is %d tall, budget is %d", total, topChromeBudget)
	}
	if menuRowHeight > menuBarHeight {
		t.Fatalf("menu row %d cannot sit in a %d bar", menuRowHeight, menuBarHeight)
	}
	if toolbarButtonHeight > applicationToolbarHeight {
		t.Fatalf("toolbar button %d cannot sit in a %d bar",
			toolbarButtonHeight, applicationToolbarHeight)
	}
}

// The chrome toggle sits inside the toolbar band when the chrome is shown, so
// it may not hang below it into the guest viewport.
func TestChromeToggleStaysInsideTheToolbar(t *testing.T) {
	bounds := touchChromeToggleBounds(1080, false)
	if bounds.Min.Y < menuBarHeight {
		t.Fatalf("toggle top %d rides up into the menu bar", bounds.Min.Y)
	}
	if bounds.Max.Y > menuBarHeight+applicationToolbarHeight {
		t.Fatalf("toggle bottom %d hangs below the toolbar band %d",
			bounds.Max.Y, menuBarHeight+applicationToolbarHeight)
	}
}
