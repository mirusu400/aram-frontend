package frontend

import "testing"

func TestTopChromeHeightUsesOneCompactRowOnTouch(t *testing.T) {
	if got := topChromeHeightForLayout(true); got != mobileAppBarHeight {
		t.Fatalf("touch top chrome = %d, want %d", got, mobileAppBarHeight)
	}
	if got := topChromeHeightForLayout(false); got != menuBarHeight+applicationToolbarHeight {
		t.Fatalf("desktop top chrome = %d", got)
	}
}

func TestMobileDrawerStaysInsideSmallPortraitAndLandscapeViews(t *testing.T) {
	for _, size := range [][2]int{{320, 568}, {568, 320}, {411, 914}} {
		got := mobileDrawerRect(size[0], size[1], 1)
		if got.Min.X != 0 || got.Min.Y != 0 || got.Max.Y != size[1] || got.Max.X > size[0] {
			t.Fatalf("drawer %v escapes %dx%d", got, size[0], size[1])
		}
		minimumUsefulWidth := min(size[0]*3/4, mobileDrawerMaxWidth)
		if got.Dx() < minimumUsefulWidth {
			t.Fatalf("drawer %v is too narrow for %dx%d", got, size[0], size[1])
		}
	}
}

func TestHiddenTouchChromeOpensUnifiedMenuAndReturnsToGame(t *testing.T) {
	shell := &Shell{
		menus:             defaultMenus(),
		activeMenu:        -1,
		touchChromeHidden: true,
		input:             &InputInfo{},
	}
	shell.toggleTouchChromeForLayout(true)
	if shell.touchChromeHidden {
		t.Fatal("opening the mobile menu left the UI hidden")
	}
	if shell.activeMenu != mobileMenuRootIndex(shell.menus) {
		t.Fatalf("active menu = %d, want unified mobile root", shell.activeMenu)
	}
	if !shell.touchMenuImmersive {
		t.Fatal("menu did not remember the immersive game state")
	}

	shell.activeMenu = -1
	shell.finishTouchMenu()
	if !shell.touchChromeHidden || shell.touchMenuImmersive {
		t.Fatalf("dismissed menu = hidden:%t return:%t", shell.touchChromeHidden, shell.touchMenuImmersive)
	}
}

func TestTouchMenuDoesNotHideHomeAfterTitleCloses(t *testing.T) {
	shell := &Shell{
		touchMenuImmersive: true,
		input:              nil,
	}
	shell.finishTouchMenu()
	if shell.touchChromeHidden || shell.touchMenuImmersive {
		t.Fatalf("closed title left immersive state = hidden:%t return:%t",
			shell.touchChromeHidden, shell.touchMenuImmersive)
	}
}
