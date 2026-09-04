package frontend

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
	"time"
)

// iconStubBackend is a preview-style backend that also supplies a fixed icon,
// so the launcher's IconBackend path can be exercised without aram-emu.
type iconStubBackend struct {
	NullBackend
	png   []byte
	calls int
}

func (b *iconStubBackend) Icon(string) ([]byte, error) {
	b.calls++
	return b.png, nil
}

func frontendTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: 0x30, G: uint8(x * 6), B: uint8(y * 6), A: 0xff})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestHomeIconLoadsFromBackend(t *testing.T) {
	isolateSettledSettings(t) // isolates APPDATA, so the icon cache is temporary
	backend := &iconStubBackend{png: frontendTestPNG(t, 20, 20)}
	shell := NewShell(backend, nil, "")

	path := filepath.Join("games", "hero.dat")
	shell.requestHomeIcon(path)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		if shell.homeIcon(path) != nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("icon was not loaded from the backend")
}

func TestHomeIconMarksMissingWithoutBackend(t *testing.T) {
	isolateSettledSettings(t)
	shell := NewShell(NullBackend{}, nil, "")

	path := filepath.Join("games", "no-backend.dat")
	shell.requestHomeIcon(path)
	if !shell.iconMissing[path] {
		t.Fatal("a backend without IconBackend should mark the icon missing, not fetch")
	}
	if shell.homeIcon(path) != nil {
		t.Fatal("homeIcon should be nil without an IconBackend")
	}
}
