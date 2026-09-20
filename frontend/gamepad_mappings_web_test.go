//go:build js && wasm

package frontend

import "testing"

func TestWebCustomGamepadMappingsAreSkipped(t *testing.T) {
	applied, err := loadCustomGamepadMappings()
	if err != nil {
		t.Fatalf("loadCustomGamepadMappings: %v", err)
	}
	if applied {
		t.Fatal("browser unexpectedly applied a filesystem controller database")
	}
}
