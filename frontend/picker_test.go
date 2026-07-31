package frontend

import (
	"slices"
	"testing"
)

func TestInputPickerPatternsIncludeZIPPackages(t *testing.T) {
	if !slices.Contains(supportedInputPatterns(), "*.zip") {
		t.Fatal("supported input picker patterns do not include ZIP packages")
	}
	if !slices.Contains(wipiPackagePatterns(), "*.zip") {
		t.Fatal("WIPI package picker patterns do not include ZIP packages")
	}
}
