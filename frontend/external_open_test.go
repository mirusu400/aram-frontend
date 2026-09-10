package frontend

import (
	"strings"
	"testing"
	"time"
)

func TestExternalOpenStatusCrossesHostBoundary(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)

	shell := NewShell(&openRecordingBackend{}, fixedPicker{}, "")
	shell.ReportExternalOpenStatus("Downloading linked package...")
	shell.consumeResults()
	if shell.status != "Downloading linked package..." {
		t.Fatalf("status = %q", shell.status)
	}
}

func TestExternalOpenRequestPreservesExpectedSHA256(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)

	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, fixedPicker{}, "")
	expected := strings.Repeat("a", 64)
	shell.OpenExternalRequest(OpenRequest{
		Path:           "downloaded.zip",
		DisplayName:    "linked.zip",
		ExpectedSHA256: expected,
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		select {
		case request := <-backend.requests:
			if request.ExpectedSHA256 != expected || request.Path != "downloaded.zip" {
				t.Fatalf("request = %+v", request)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("external request did not reach backend")
}
