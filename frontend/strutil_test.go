package frontend

import "testing"

func TestDisplayNamePrefersExplicitName(t *testing.T) {
	if got := displayName(OpenRequest{DisplayName: "Custom", Path: "roms/other.gba"}); got != "Custom" {
		t.Fatalf("displayName = %q, want %q", got, "Custom")
	}
	if got := displayName(OpenRequest{Path: "roms/game.gba"}); got != "game.gba" {
		t.Fatalf("displayName = %q, want %q", got, "game.gba")
	}
	if got := displayName(OpenRequest{}); got != "document" {
		t.Fatalf("displayName = %q, want %q", got, "document")
	}
}

func TestDisplayNameForInfoHandlesNil(t *testing.T) {
	if got := displayNameForInfo(nil); got != "" {
		t.Fatalf("displayNameForInfo(nil) = %q, want empty", got)
	}
	if got := displayNameForInfo(&InputInfo{DisplayName: "Game"}); got != "Game" {
		t.Fatalf("displayNameForInfo = %q, want %q", got, "Game")
	}
}

func TestEmptyFallback(t *testing.T) {
	if got := emptyFallback("", "fallback"); got != "fallback" {
		t.Fatalf("emptyFallback = %q, want %q", got, "fallback")
	}
	if got := emptyFallback("value", "fallback"); got != "value" {
		t.Fatalf("emptyFallback = %q, want %q", got, "value")
	}
}

func TestMenuWidthsPinsCurrentGeometry(t *testing.T) {
	menus := []Menu{{Label: "File"}, {Label: "A"}, {Label: "Settings"}}
	got := menuWidths(menus)
	want := []int{58, 58, 78}
	if len(got) != len(want) {
		t.Fatalf("menuWidths length = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("menuWidths[%d] = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestMenuStartXAccumulatesWidths(t *testing.T) {
	menus := []Menu{{Label: "File"}, {Label: "A"}, {Label: "Settings"}}
	if got := menuStartX(menus, 0); got != 0 {
		t.Fatalf("menuStartX(0) = %d, want 0", got)
	}
	if got := menuStartX(menus, 2); got != 116 {
		t.Fatalf("menuStartX(2) = %d, want 116", got)
	}
}
