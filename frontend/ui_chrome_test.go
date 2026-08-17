package frontend

import (
	"image"
	"testing"
)

func TestTouchMenuLayoutStaysInsideTheViewport(t *testing.T) {
	menuSizes := make([]int, 0, len(defaultMenus()))
	for _, menu := range defaultMenus() {
		menuSizes = append(menuSizes, len(menu.Commands))
	}
	for _, size := range [][2]int{{411, 914}, {914, 411}, {640, 360}, {360, 640}} {
		viewWidth, viewHeight := size[0], size[1]
		safe := image.Rect(
			0,
			menuBarHeight,
			viewWidth,
			viewHeight-statusBarHeight,
		)
		for _, count := range menuSizes {
			layout := touchMenuLayoutFor(viewWidth, viewHeight, 44, 24, count)
			if !layout.window.In(safe) {
				t.Errorf(
					"touch menu window escapes %dx%d for %d commands: %v",
					viewWidth,
					viewHeight,
					count,
					layout.window,
				)
			}
			if layout.columns*layout.perColumn < count {
				t.Errorf(
					"touch menu drops commands at %dx%d: %d columns x %d rows < %d",
					viewWidth,
					viewHeight,
					layout.columns,
					layout.perColumn,
					count,
				)
			}
			content := 24 + layout.perColumn*44
			if layout.window.Dy() < content {
				t.Errorf(
					"touch menu clips its rows at %dx%d for %d commands: window %d < content %d",
					viewWidth,
					viewHeight,
					count,
					layout.window.Dy(),
					content,
				)
			}
		}
	}
}

func TestTouchMenuLayoutUsesOneColumnOnTallScreens(t *testing.T) {
	layout := touchMenuLayoutFor(411, 914, 44, 24, 12)
	if layout.columns != 1 {
		t.Fatalf("portrait columns = %d, want 1", layout.columns)
	}
	if layout.perColumn != 12 {
		t.Fatalf("portrait rows = %d, want 12", layout.perColumn)
	}
}
