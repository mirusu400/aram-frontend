package frontend

import (
	"context"
	"errors"
	"testing"
)

func TestNullBackendPreservesSelectedName(t *testing.T) {
	info, err := (NullBackend{}).Open(context.Background(), OpenRequest{
		Path: `games/example.dat`,
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("Open error = %v", err)
	}
	if info.DisplayName != "example.dat" {
		t.Fatalf("DisplayName = %q", info.DisplayName)
	}
}
