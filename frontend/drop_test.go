package frontend

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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

// locatedFS mimics Ebitengine's desktop dropped-file system: every opened
// file reports its real absolute path through AbsPath.
type locatedFS struct {
	root string
}

type locatedFile struct {
	fs.File
	absPath string
}

func (f locatedFile) AbsPath() string { return f.absPath }

func (l locatedFS) Open(name string) (fs.File, error) {
	file, err := os.DirFS(l.root).Open(name)
	if err != nil {
		return nil, err
	}
	return locatedFile{File: file, absPath: filepath.Join(l.root, filepath.FromSlash(name))}, nil
}

func (l locatedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(os.DirFS(l.root), name)
}

// TestDroppedFileWithRealPathOpensInPlace covers a desktop drag-and-drop: the
// dropped file's own path must be opened (not a private cache copy marked
// temporary), so the title is recorded in the recent list like one chosen
// through the file dialog.
func TestDroppedFileWithRealPathOpensInPlace(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	root := t.TempDir()
	dropped := filepath.Join(root, "dropped.dat")
	if err := os.WriteFile(dropped, []byte("synthetic input"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := make(chan dropResult, 1)
	readFirstDroppedFile(locatedFS{root: root}, results)
	result := <-results
	if result.err != nil {
		t.Fatalf("drop error: %v", result.err)
	}
	if result.temporary || result.path != dropped || result.displayName != "dropped.dat" {
		t.Fatalf("drop result = %+v, want the real path opened in place", result)
	}

	backend := &openRecordingBackend{requests: make(chan OpenRequest, 1)}
	shell := NewShell(backend, fixedPicker{}, "")
	shell.dropResults <- result
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		shell.consumeResults()
		if len(shell.settings.RecentFiles) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(shell.settings.RecentFiles) != 1 || shell.settings.RecentFiles[0].Path != dropped {
		t.Fatalf("recent after drop = %#v, want %q", shell.settings.RecentFiles, dropped)
	}
	if shell.temporaryPath != "" {
		t.Fatalf("an in-place drop was tracked as a temporary copy: %q", shell.temporaryPath)
	}
}

// TestDroppedFileWithoutRealPathIsCopied keeps the fallback: a file system
// that cannot name the real file is copied to the private cache and opened as
// a temporary input.
func TestDroppedFileWithoutRealPathIsCopied(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	results := make(chan dropResult, 1)
	readFirstDroppedFile(fstest.MapFS{"drop.dat": {Data: []byte("bytes")}}, results)
	result := <-results
	if result.err != nil {
		t.Fatalf("drop error: %v", result.err)
	}
	if !result.temporary || result.path == "" || result.displayName != "drop.dat" {
		t.Fatalf("fallback drop result = %+v, want a temporary cache copy", result)
	}
	data, err := os.ReadFile(result.path)
	if err != nil || string(data) != "bytes" {
		t.Fatalf("cache copy = %q, %v", data, err)
	}
	_ = os.Remove(result.path)
}
