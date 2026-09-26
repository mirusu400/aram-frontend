package frontend

import (
	"bytes"
	"image/png"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
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
		if !bytes.Equal(request.icon, backend.png) {
			t.Fatal("shortcut did not keep the game's extracted icon")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("selected game was not sent to the native shortcut host")
	}
}

func TestHomeShortcutWithoutGameIconIsNotPinned(t *testing.T) {
	isolateSettledSettings(t)
	host := &recordingShortcutHost{requests: make(chan shortcutRequest, 1)}
	SetNativeShortcutHost(host)
	t.Cleanup(func() { SetNativeShortcutHost(nil) })

	shell := NewShell(&iconStubBackend{}, nil, "")
	shell.pinHomeShortcut(filepath.Join(t.TempDir(), "no-icon.zip"))
	select {
	case <-shell.externalOpenStatus:
	case <-time.After(3 * time.Second):
		t.Fatal("missing game icon was not reported")
	}
	select {
	case request := <-host.requests:
		t.Fatalf("pinned a shortcut without a game icon: %#v", request)
	default:
	}
}

func TestHomeShortcutActionFitsPhoneWidth(t *testing.T) {
	isolateSettledSettings(t)
	host := &recordingShortcutHost{requests: make(chan shortcutRequest, 1)}
	SetNativeShortcutHost(host)
	t.Cleanup(func() { SetNativeShortcutHost(nil) })

	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.Language = string(LanguageKorean)
	shell.settings.RecentFiles = []RecentEntry{{Path: "game.zip", Name: "Game"}}
	shell.layoutAtScale(320, 720, 1)
	shell.interfaceUI.sync(shell)
	shell.interfaceUI.ui.Update()
	buttons := shell.interfaceUI
	shell.Draw(ebiten.NewImage(320, 720))
	shortcutRect := buttons.homeShortcutButton.GetWidget().Rect
	if shortcutRect.Empty() {
		t.Fatal("Create shortcut has no laid-out button")
	}
	for _, other := range []struct {
		name   string
		button *widget.Button
	}{
		{"Favorite", buttons.homeFavButton},
		{"Open", buttons.homeOpenButton},
	} {
		if shortcutRect.Overlaps(other.button.GetWidget().Rect) {
			t.Fatalf("Create shortcut %v overlaps %s %v at phone width",
				shortcutRect, other.name, other.button.GetWidget().Rect)
		}
	}
}
