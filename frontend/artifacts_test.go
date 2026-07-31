package frontend

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

func TestOpenDebugBundleFolderCreatesAndOpensArtifactDirectory(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	original := openArtifactFolder
	t.Cleanup(func() { openArtifactFolder = original })
	var opened string
	openArtifactFolder = func(path string) error {
		opened = path
		return nil
	}
	shell := &Shell{
		backend:  NullBackend{},
		settings: defaultSettings(),
	}
	shell.openDebugBundleFolder()

	expected := filepath.Join(config, "ARAM", "debug")
	if filepath.Clean(opened) != filepath.Clean(expected) {
		t.Fatalf("opened folder = %q, want %q", opened, expected)
	}
	if info, err := os.Stat(expected); err != nil || !info.IsDir() {
		t.Fatalf("debug folder was not created: info=%v err=%v", info, err)
	}
	if !strings.Contains(shell.status, expected) {
		t.Fatalf("open-folder status = %q", shell.status)
	}
}

func TestOpenDebugBundleFolderReportsPlatformFailure(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	original := openArtifactFolder
	t.Cleanup(func() { openArtifactFolder = original })
	openArtifactFolder = func(string) error {
		return errors.New("synthetic open failure")
	}
	shell := &Shell{
		backend:  NullBackend{},
		settings: defaultSettings(),
	}
	shell.openDebugBundleFolder()
	if !strings.Contains(shell.status, "synthetic open failure") {
		t.Fatalf("open-folder failure status = %q", shell.status)
	}
}

func TestCopyFirstDroppedFileUsesPrivateCacheCopy(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)

	files := fstest.MapFS{
		"sample.dat": &fstest.MapFile{Data: []byte("synthetic input")},
	}
	results := make(chan dropResult, 1)
	copyFirstDroppedFile(files, results)
	result := <-results
	if result.err != nil {
		t.Fatal(result.err)
	}
	defer removeTemporaryDrop(result.path)

	if filepath.Ext(result.path) != ".dat" {
		t.Fatalf("cached extension = %q", filepath.Ext(result.path))
	}
	data, err := os.ReadFile(result.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "synthetic input" {
		t.Fatalf("cached data = %q", data)
	}
	if result.displayName != "sample.dat" {
		t.Fatalf("display name = %q", result.displayName)
	}
	info, err := os.Stat(result.path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cached file mode = %v, want private", info.Mode().Perm())
	}
}

func TestCopyDroppedFileRejectsEmptyDrop(t *testing.T) {
	results := make(chan dropResult, 1)
	copyFirstDroppedFile(fstest.MapFS{}, results)
	if result := <-results; result.err == nil {
		t.Fatal("empty drop succeeded")
	}
}

func TestRemoveTemporaryDropDoesNotRemoveOutsideCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "cache"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	outside := filepath.Join(root, "outside.dat")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeTemporaryDrop(outside)
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestWritePNGArtifactPreservesGuestNativePixels(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	source := image.NewRGBA(image.Rect(10, 20, 12, 22))
	source.Set(10, 20, color.RGBA{R: 0xff, G: 0x20, B: 0x10, A: 0xff})
	snapshot := cloneImage(source)
	path, err := writePNGArtifact("screenshots", "test", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer encoded.Close()
	decoded, err := png.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("screenshot bounds = %v", decoded.Bounds())
	}
	r, g, b, a := decoded.At(0, 0).RGBA()
	if r != 0xffff || g != 0x2020 || b != 0x1010 || a != 0xffff {
		t.Fatalf("screenshot pixel = %#04x %#04x %#04x %#04x", r, g, b, a)
	}
}

var _ fs.FS = fstest.MapFS{}
