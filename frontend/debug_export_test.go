package frontend

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWriteDebugBundleIncludesManifestLogsAndBackendArtifacts(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	createdAt := time.Date(2026, 7, 31, 1, 2, 3, 0, time.UTC)
	snapshot := debugBundleSnapshot{
		CreatedAt: createdAt,
		Input: &InputInfo{
			DisplayName: "synthetic.dat",
			Format:      "synthetic",
			Size:        12,
			SHA256:      strings.Repeat("a", 64),
			ProfileID:   "test/profile",
		},
		Backend:       "test-core",
		BackendState:  StateFaulted,
		FrontendState: FrontendGuestFaulted,
		Problem: &FrontendProblem{
			State:  FrontendGuestFaulted,
			Reason: "synthetic fault",
		},
		Settings:     debugSettingsReport{Language: "en", Speed: 1, Volume: 80},
		FrontendLogs: []string{"01:02:03  Loaded synthetic.dat", "01:02:04  synthetic fault"},
	}
	path, warning, err := writeDebugBundle(snapshot, []DebugArtifact{
		{
			Name:      "core.json",
			MediaType: "application/json",
			Data:      []byte("{\"state\":\"faulted\"}\n"),
		},
		{
			Name:      "core.log",
			MediaType: "text/plain; charset=utf-8",
			Data:      []byte("host_call\n"),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if filepath.Ext(path) != ".zip" {
		t.Fatalf("bundle path = %q", path)
	}

	entries := readDebugZIP(t, path)
	if got, want := sortedMapKeys(entries), []string{
		"core.json",
		"core.log",
		"frontend.log",
		"manifest.json",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	if string(entries["frontend.log"]) !=
		"01:02:03  Loaded synthetic.dat\n01:02:04  synthetic fault\n" {
		t.Fatalf("frontend log = %q", entries["frontend.log"])
	}

	var manifest debugBundleManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != debugBundleSchemaVersion ||
		!manifest.CreatedAt.Equal(createdAt) ||
		manifest.Session.Input == nil ||
		manifest.Session.Input.SHA256 != strings.Repeat("a", 64) ||
		manifest.Session.Backend != "test-core" ||
		manifest.Session.Problem == nil {
		t.Fatalf("manifest = %+v", manifest)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("manifest files = %+v", manifest.Files)
	}
	for _, file := range manifest.Files {
		if file.SHA256 == "" || file.Size != len(entries[file.Name]) {
			t.Fatalf("manifest file = %+v", file)
		}
	}
}

func TestWriteDebugBundleKeepsFrontendLogsWhenBackendCollectionFails(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	path, warning, err := writeDebugBundle(
		debugBundleSnapshot{
			CreatedAt:    time.Now().UTC(),
			FrontendLogs: []string{"frontend survived"},
		},
		[]DebugArtifact{
			{Name: "../escape.log", Data: []byte("bad")},
			{Name: "frontend.log", Data: []byte("duplicate")},
			{Name: "core.log", Data: []byte("useful")},
		},
		os.ErrDeadlineExceeded,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warning, os.ErrDeadlineExceeded.Error()) ||
		!strings.Contains(warning, "invalid debug artifact") ||
		!strings.Contains(warning, "duplicate debug artifact") {
		t.Fatalf("warning = %q", warning)
	}
	entries := readDebugZIP(t, path)
	if _, ok := entries["core.log"]; !ok {
		t.Fatal("valid partial backend artifact is missing")
	}
	if _, ok := entries["../escape.log"]; ok {
		t.Fatal("unsafe backend artifact was written")
	}
	if string(entries["frontend.log"]) != "frontend survived\n" {
		t.Fatalf("frontend log = %q", entries["frontend.log"])
	}
	var manifest debugBundleManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BackendCollectionError != warning {
		t.Fatalf(
			"manifest warning = %q, want %q",
			manifest.BackendCollectionError,
			warning,
		)
	}
}

func TestCaptureDebugBundleRedactsHostPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HOME", root)
	selected := filepath.Join(root, "private", "game.dat")
	shell := &Shell{
		backend:      NullBackend{},
		settings:     defaultSettings(),
		selectedPath: selected,
		logs: []string{
			"opened " + selected,
			"artifact " + filepath.Join(root, "ARAM", "debug.zip"),
		},
		problem: &FrontendProblem{Reason: "failed at " + selected},
	}
	snapshot := shell.captureDebugBundleSnapshot(time.Now().UTC())
	joined := strings.Join(snapshot.FrontendLogs, "\n")
	if strings.Contains(joined, root) ||
		strings.Contains(snapshot.Problem.Reason, root) {
		t.Fatalf(
			"host path leaked: logs=%q problem=%q",
			joined,
			snapshot.Problem.Reason,
		)
	}
	if !strings.Contains(joined, "<redacted-path>") ||
		!strings.Contains(snapshot.Problem.Reason, "<redacted-path>") {
		t.Fatalf(
			"redaction marker missing: logs=%q problem=%q",
			joined,
			snapshot.Problem.Reason,
		)
	}
}

func readDebugZIP(t *testing.T, path string) map[string][]byte {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		entries[file.Name] = data
	}
	return entries
}

func sortedMapKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
