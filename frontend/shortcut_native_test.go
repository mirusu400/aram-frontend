package frontend

import (
	"bytes"
	"image/png"
	"path/filepath"
	"testing"
	"time"
)

type recordingShortcutHost struct {
	requests chan shortcutRequest
}

type shortcutRequest struct {
	path, title string
	icon        []byte
}

func (h *recordingShortcutHost) PinGameShortcut(path, title string, icon []byte) error {
	h.requests <- shortcutRequest{path, title, icon}
	return nil
}

func TestHomeShortcutUsesSelectedGameNameAndIcon(t *testing.T) {
	isolateSettledSettings(t)
	host := &recordingShortcutHost{requests: make(chan shortcutRequest, 1)}
	SetNativeShortcutHost(host)
	t.Cleanup(func() { SetNativeShortcutHost(nil) })

	path := filepath.Join(t.TempDir(), "game.zip")
	backend := &iconStubBackend{png: frontendTestPNG(t, 2, 2)}
	shell := NewShell(backend, nil, "")
	shell.settings.RecentFiles = []RecentEntry{{Path: path, Name: "My Game"}}
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	if shell.interfaceUI.homeShortcutButton == nil ||
		shell.interfaceUI.homeShortcutButton.GetWidget().Disabled {
		t.Fatal("selected game has no enabled Home shortcut action")
	}
	shell.pinHomeShortcut(path)

	select {
	case request := <-host.requests:
		if request.path != path || request.title != "My Game" {
			t.Fatalf("shortcut = %#v", request)
		}
		if _, err := png.Decode(bytes.NewReader(request.icon)); err != nil {
			t.Fatalf("shortcut icon is not PNG: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("selected game was not sent to the native shortcut host")
	}
}
