package frontend

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type fixedPicker struct {
	path string
}

type deferredPicker struct{}

func (deferredPicker) OpenFile() (string, error) {
	return "", ErrPickerDeferred
}

func (deferredPicker) OpenFirmwareDirectory(string) (string, error) {
	return "", ErrPickerDeferred
}

func (deferredPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}

func (picker fixedPicker) OpenFile() (string, error) {
	return picker.path, nil
}

func (fixedPicker) OpenFirmwareDirectory(string) (string, error) {
	return "", ErrPickerUnavailable
}

func (fixedPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}

type openRecordingBackend struct {
	requests chan OpenRequest
}

func (backend *openRecordingBackend) Open(
	_ context.Context,
	request OpenRequest,
) (InputInfo, error) {
	backend.requests <- request
	return InputInfo{DisplayName: "synthetic.dat", Format: "eads"}, nil
}

func (*openRecordingBackend) State() BackendState          { return StateReady }
func (*openRecordingBackend) Supports(BackendCommand) bool { return false }
func (*openRecordingBackend) Execute(context.Context, BackendCommand) error {
	return ErrBackendUnavailable
}
func (*openRecordingBackend) Close() error { return nil }

func TestFileOpenConvergesOnBackendOpenRequest(t *testing.T) {
	for _, name := range []string{"synthetic.dat", "synthetic.zip"} {
		t.Run(name, func(t *testing.T) {
			temporary := t.TempDir()
			t.Setenv("APPDATA", temporary)
			t.Setenv("XDG_CONFIG_HOME", temporary)
			path := filepath.Join(temporary, name)
			backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
			shell := NewShell(backend, fixedPicker{path: path}, "")

			shell.chooseFile()
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				shell.consumeResults()
				select {
				case request := <-backend.requests:
					if request.Path != path || request.Firmware {
						t.Fatalf("OpenRequest = %+v", request)
					}
					return
				default:
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatal("File/Open did not reach Backend.Open")
		})
	}
}

func TestDeferredNativePickerCanBeCanceled(t *testing.T) {
	shell := NewShell(NullBackend{}, deferredPicker{}, "")
	shell.chooseFile()
	deadline := time.Now().Add(time.Second)
	waitingStatus := shell.tr("Waiting for the native document picker...")
	for shell.status != waitingStatus && time.Now().Before(deadline) {
		shell.consumeResults()
		time.Sleep(time.Millisecond)
	}
	if shell.status != waitingStatus {
		t.Fatalf("deferred picker status = %q", shell.status)
	}

	shell.CancelExternalDocumentSelection()
	shell.consumeResults()
	if shell.status != shell.tr("Selection canceled") || shell.dialogOpen {
		t.Fatalf("canceled native picker: status=%q dialog=%t", shell.status, shell.dialogOpen)
	}
}
