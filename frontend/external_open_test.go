package frontend

import (
	"context"
	"strings"
	"testing"
	"time"
)

type externalAutoStartBackend struct {
	requests chan OpenRequest
	commands chan BackendCommand
}

func (backend *externalAutoStartBackend) Open(
	_ context.Context,
	request OpenRequest,
) (InputInfo, error) {
	backend.requests <- request
	return InputInfo{
		DisplayName: request.DisplayName,
		Format:      "eads",
		SHA256:      request.ExpectedSHA256,
	}, nil
}

func (*externalAutoStartBackend) State() BackendState { return StateReady }
func (*externalAutoStartBackend) Supports(command BackendCommand) bool {
	return command == CommandStart
}
func (backend *externalAutoStartBackend) Execute(
	_ context.Context,
	command BackendCommand,
) error {
	backend.commands <- command
	return nil
}
func (*externalAutoStartBackend) Close() error { return nil }

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

func TestExternalOpenRequestAutomaticallyStartsVerifiedInput(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)

	backend := &externalAutoStartBackend{
		requests: make(chan OpenRequest, 1),
		commands: make(chan BackendCommand, 1),
	}
	shell := NewShell(backend, fixedPicker{}, "")
	expected := strings.Repeat("b", 64)
	shell.OpenExternalRequest(OpenRequest{
		Path:           "cached-linked.zip",
		DisplayName:    "linked.zip",
		ExpectedSHA256: expected,
	})

	deadline := time.Now().Add(3 * time.Second)
	var request OpenRequest
	requestObserved := false
	for time.Now().Before(deadline) {
		shell.consumeResults()
		if !requestObserved {
			select {
			case request = <-backend.requests:
				requestObserved = true
			default:
			}
		}
		select {
		case command := <-backend.commands:
			if !requestObserved {
				t.Fatal("start reached backend before its open request was observed")
			}
			if request.ExpectedSHA256 != expected || request.Path != "cached-linked.zip" {
				t.Fatalf("request = %+v", request)
			}
			if command != CommandStart {
				t.Fatalf("automatic command = %q, want %q", command, CommandStart)
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("verified external request did not automatically start")
}
