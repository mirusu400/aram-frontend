package frontend

import (
	"testing"
	"time"
)

// TestDroppedBytesOpenThroughDataRequest covers the web drag-and-drop path: a
// dropResult that carries in-memory bytes (no filesystem path) must reach
// Backend.Open through OpenRequest.Data, mirroring the web picker, rather than
// as a temporary filesystem path the browser build cannot produce.
func TestDroppedBytesOpenThroughDataRequest(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)

	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, fixedPicker{}, "")

	shell.dropResults <- dropResult{
		data:        []byte("synthetic input"),
		displayName: "drop.dat",
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if string(request.Data) != "synthetic input" {
				t.Fatalf("dropped-bytes data = %q", request.Data)
			}
			if request.Path != "" || request.Temporary {
				t.Fatalf("dropped-bytes request kept a filesystem path: %+v", request)
			}
			if request.DisplayName != "drop.dat" {
				t.Fatalf("dropped-bytes display name = %q", request.DisplayName)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("dropped bytes did not reach Backend.Open with Data")
}
