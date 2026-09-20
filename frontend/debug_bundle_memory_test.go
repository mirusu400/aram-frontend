package frontend

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"
)

func TestCollectDebugBundleDataDoesNotRequireAConfigDirectory(t *testing.T) {
	name, data, _, err := collectDebugBundleData(debugBundleSnapshot{
		CreatedAt:    time.Date(2026, 9, 21, 1, 2, 3, 0, time.UTC),
		FrontendLogs: []string{"ready"},
	}, NullBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if name == "" || len(data) == 0 {
		t.Fatalf("bundle name=%q size=%d", name, len(data))
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open in-memory debug bundle: %v", err)
	}
	if len(archive.File) < 2 || archive.File[0].Name != "manifest.json" {
		t.Fatalf("debug bundle entries = %#v", archive.File)
	}
}
