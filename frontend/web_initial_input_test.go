package frontend

import (
	"testing"
	"time"
)

// TestPermalinkBytesOpenThroughExternalBridge pins the host/WASM boundary used
// by the web player: an integrity-checked remote package must reach Backend.Open
// as in-memory data, never as a browser-inaccessible filesystem path.
func TestPermalinkBytesOpenThroughExternalBridge(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)

	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, fixedPicker{}, "")
	shell.OpenExternalBytes("permalink.zip", []byte("verified package"), false)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if request.DisplayName != "permalink.zip" {
				t.Fatalf("display name = %q", request.DisplayName)
			}
			if string(request.Data) != "verified package" {
				t.Fatalf("data = %q", request.Data)
			}
			if request.Path != "" || request.Temporary || request.Firmware {
				t.Fatalf("permalink request crossed the wrong boundary: %+v", request)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("permalink bytes did not reach Backend.Open")
}
