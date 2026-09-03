package frontend

import (
	"context"
	"testing"
	"time"
)

func TestSelfUpdateDisabledParsesFlag(t *testing.T) {
	previous := SelfUpdateDisabled
	t.Cleanup(func() { SelfUpdateDisabled = previous })

	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"off", false},
		{"  ", false},
		{"1", true},
		{"true", true},
		{"yes", true},
		{"play", true},
		{" 1 ", true},
	}
	for _, tc := range cases {
		SelfUpdateDisabled = tc.value
		if got := selfUpdateDisabled(); got != tc.want {
			t.Fatalf("selfUpdateDisabled() with %q = %t, want %t", tc.value, got, tc.want)
		}
	}
}

// A store build hides the whole updates section except the running version.
func TestSelfUpdateDisabledHidesUpdateSettings(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	previous := SelfUpdateDisabled
	t.Cleanup(func() { SelfUpdateDisabled = previous })

	shell := NewShell(NullBackend{}, nil, "")

	SelfUpdateDisabled = ""
	enabled := updateSettingsRowModels(shell)
	if len(enabled) <= 1 {
		t.Fatalf("enabled update rows = %d, want the download section too", len(enabled))
	}
	if !hasRow(enabled, "ARAM product") {
		t.Fatal("enabled update rows are missing the product download row")
	}

	SelfUpdateDisabled = "1"
	disabled := updateSettingsRowModels(shell)
	if len(disabled) != 1 || disabled[0].label != "Current version" {
		t.Fatalf("disabled update rows = %#v, want only the current version", disabled)
	}
	if hasRow(disabled, "ARAM product") || hasRow(disabled, "Update channel") {
		t.Fatal("a store build still exposes an update action")
	}
}

func hasRow(rows []settingsRowModel, label string) bool {
	for _, row := range rows {
		if row.label == label {
			return true
		}
	}
	return false
}

// downloadUpdate refuses to fetch anything and dispatches no request to the
// updater when self-update is off.
func TestSelfUpdateDisabledStopsProductDownload(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	previous := SelfUpdateDisabled
	t.Cleanup(func() { SelfUpdateDisabled = previous })
	SelfUpdateDisabled = "1"

	downloader := &fakeUpdateDownloader{requests: make(chan updateDownload, 1)}
	shell := NewShell(NullBackend{}, nil, "")
	shell.updater = downloader

	if shell.downloadUpdate(updateComponentProduct) {
		t.Fatal("downloadUpdate started while self-update was disabled")
	}
	select {
	case request := <-downloader.requests:
		t.Fatalf("a download request was dispatched: %#v", request)
	case <-time.After(100 * time.Millisecond):
	}
}

type checkRecordingUpdater struct {
	checked chan struct{}
}

func (u *checkRecordingUpdater) CheckLatest(
	context.Context,
	updateComponent,
	updateChannel,
) (string, error) {
	select {
	case u.checked <- struct{}{}:
	default:
	}
	return "", nil
}

func (u *checkRecordingUpdater) Download(
	context.Context,
	updateComponent,
	updateChannel,
) (updateDownload, error) {
	return updateDownload{}, nil
}

// The startup check never reaches the network for a store build, even when the
// build carries a real release version that would otherwise be checked.
func TestSelfUpdateDisabledSkipsStartupCheck(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	previousVersion := BuildVersion
	BuildVersion = "1.2.3"
	t.Cleanup(func() { BuildVersion = previousVersion })
	previousFlag := SelfUpdateDisabled
	SelfUpdateDisabled = "1"
	t.Cleanup(func() { SelfUpdateDisabled = previousFlag })

	updater := &checkRecordingUpdater{checked: make(chan struct{}, 1)}
	shell := NewShell(NullBackend{}, nil, "")
	shell.updater = updater

	shell.startUpdateCheck()
	select {
	case <-updater.checked:
		t.Fatal("startUpdateCheck contacted the network while self-update was disabled")
	case <-time.After(100 * time.Millisecond):
	}
}
