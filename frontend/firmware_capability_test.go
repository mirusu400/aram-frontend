package frontend

import "testing"

type firmwareCapableBackend struct {
	NullBackend
	supported bool
}

func (b firmwareCapableBackend) SupportsFirmware() bool { return b.supported }

func firmwareCommand(t *testing.T) Command {
	t.Helper()
	for _, menu := range defaultMenus() {
		for _, command := range menu.Commands {
			if command.ID == "file.open_firmware" {
				return command
			}
		}
	}
	t.Fatal("file.open_firmware is not registered")
	return Command{}
}

// An application-only backend must not offer a firmware menu entry whose only
// possible outcome is a "not implemented" error.
func TestFirmwareCommandIsDisabledWithoutBackendSupport(t *testing.T) {
	command := firmwareCommand(t)
	for _, testCase := range []struct {
		name      string
		backend   Backend
		supported bool
	}{
		{"no capability", NullBackend{}, false},
		{"capability declined", firmwareCapableBackend{supported: false}, false},
		{"capability granted", firmwareCapableBackend{supported: true}, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			shell := NewShell(testCase.backend, nil, "")
			t.Cleanup(func() { _ = shell.backend.Close() })
			capability := command.Availability(shell)
			if capability.Supported != testCase.supported {
				t.Fatalf("firmware command supported = %v, want %v (reason %q)",
					capability.Supported, testCase.supported, capability.Reason)
			}
			if !testCase.supported && capability.Reason == "" {
				t.Fatal("a disabled firmware command gave no reason")
			}
		})
	}
}
