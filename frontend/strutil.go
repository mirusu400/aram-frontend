package frontend

import (
	"path/filepath"
	"strings"
)

func effectiveMenuItemHeight() int {
	if platformUsesTouchLayout() {
		return 44
	}
	return menuItemHeight
}

func menuWidths(menus []Menu) []int {
	widths := make([]int, len(menus))
	for index, menu := range menus {
		width := len(menu.Label)*7 + 22
		if width < 58 {
			width = 58
		}
		widths[index] = width
	}
	return widths
}

func menuStartX(menus []Menu, index int) int {
	offset := 0
	widths := menuWidths(menus)
	for current := 0; current < index; current++ {
		offset += widths[current]
	}
	return offset
}

func displayName(request OpenRequest) string {
	if request.DisplayName != "" {
		return request.DisplayName
	}
	if request.Path != "" {
		return filepath.Base(request.Path)
	}
	return "document"
}

func displayNameForInfo(info *InputInfo) string {
	if info == nil {
		return ""
	}
	return info.DisplayName
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func shorten(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
