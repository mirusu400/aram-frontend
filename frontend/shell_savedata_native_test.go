package frontend

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recordingPickerHost stands in for the Android/iOS Activity that owns the
// system document picker.
type recordingPickerHost struct {
	kinds []string
}

func (host *recordingPickerHost) RequestDocument(kind string) {
	host.kinds = append(host.kinds, kind)
}

// recordingShareHost stands in for a native host that can hand a private file
// to another app.
type recordingShareHost struct {
	paths []string
	mimes []string
	err   error
}

func (host *recordingShareHost) ShareFile(path, mimeType, _ string) error {
	host.paths = append(host.paths, path)
	host.mimes = append(host.mimes, mimeType)
	return host.err
}

func useShareHost(t *testing.T, host NativeShareHost) {
	t.Helper()
	previous := shareNativeFile
	SetNativeShareHost(host)
	t.Cleanup(func() {
		SetNativeShareHost(nil)
		shareNativeFile = previous
	})
}

// TestNativePickerAsksForTheSaveBackupKind is the regression for a handset
// where "Restore Save..." was a dead end: the mobile picker answered
// ErrPickerUnavailable instead of asking the host for a file, so a backup
// could never be restored on Android.
func TestNativePickerAsksForTheSaveBackupKind(t *testing.T) {
	host := &recordingPickerHost{}
	SetNativePickerHost(host)
	t.Cleanup(func() { SetNativePickerHost(nil) })

	picker := nativeHostPicker{}
	if _, err := picker.OpenSaveBackupFile(); !errors.Is(err, ErrPickerDeferred) {
		t.Fatalf("OpenSaveBackupFile err = %v, want ErrPickerDeferred", err)
	}
	if _, err := picker.OpenFile(); !errors.Is(err, ErrPickerDeferred) {
		t.Fatalf("OpenFile err = %v, want ErrPickerDeferred", err)
	}
	if _, err := picker.OpenFirmwareDirectory(""); !errors.Is(err, ErrPickerDeferred) {
		t.Fatalf("OpenFirmwareDirectory err = %v, want ErrPickerDeferred", err)
	}

	want := []string{DocumentKindSaveBackup, DocumentKindInput, DocumentKindFirmware}
	if len(host.kinds) != len(want) {
		t.Fatalf("host saw %v, want %v", host.kinds, want)
	}
	for index, kind := range want {
		if host.kinds[index] != kind {
			t.Fatalf("request %d = %q, want %q", index, host.kinds[index], kind)
		}
	}
}

func TestNativePickerWithoutHostIsUnavailable(t *testing.T) {
	SetNativePickerHost(nil)
	if _, err := (nativeHostPicker{}).OpenSaveBackupFile(); !errors.Is(err, ErrPickerUnavailable) {
		t.Fatalf("err = %v, want ErrPickerUnavailable", err)
	}
}

// TestImportExternalSaveBackupRestoresOnTheUpdateLoop covers the answer half
// of the native restore: the host copies the picked backup into private
// storage and hands the path back.
func TestImportExternalSaveBackupRestoresOnTheUpdateLoop(t *testing.T) {
	backend := &saveTransferTestBackend{}
	shell := newSaveShell(backend)
	shell.input = &InputInfo{DisplayName: "title", SHA256: "00"}
	shell.externalSaveBackups = make(chan string, 1)
	shell.dialogOpen = true

	path := filepath.Join(t.TempDir(), "native.aramsave")
	if err := os.WriteFile(path, []byte("NATIVE-BYTES"), 0o600); err != nil {
		t.Fatal(err)
	}

	shell.ImportExternalSaveBackup(path)
	shell.consumeResults()
	if shell.dialogOpen {
		t.Fatal("dialog stayed open after the native picker answered")
	}
	select {
	case result := <-shell.saveRestoreResults:
		if result.err != nil {
			t.Fatalf("restore reported error: %v", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("the restore never reached the backend")
	}
	if string(backend.imported) != "NATIVE-BYTES" {
		t.Fatalf("backend received %q", backend.imported)
	}
}

// TestOpenSaveBackupFolderSharesNewestBackup covers the export half. On a
// handset the backup is written below app-private storage, which no file
// manager can reach, so revealing the folder is not merely unsupported - it
// would leave the backup unreachable forever.
func TestOpenSaveBackupFolderSharesNewestBackup(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	directory, err := artifactDirectory("save-backups")
	if err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(directory, "title-20260101-000000.000.aramsave")
	newer := filepath.Join(directory, "title-20260102-000000.000.aramsave")
	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte("BACKUP"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	previousFolder := openArtifactFolder
	openArtifactFolder = func(string) error { return ErrFolderBrowserUnavailable }
	t.Cleanup(func() { openArtifactFolder = previousFolder })

	host := &recordingShareHost{}
	useShareHost(t, host)

	shell := newSaveShell(&saveTransferTestBackend{})
	shell.openSaveBackupFolder()

	if len(host.paths) != 1 {
		t.Fatalf("host was handed %v, want exactly the newest backup", host.paths)
	}
	if host.paths[0] != newer {
		t.Fatalf("shared %q, want the newest backup %q", host.paths[0], newer)
	}
	if host.mimes[0] != saveBackupMIMEType {
		t.Fatalf("shared as %q, want %q", host.mimes[0], saveBackupMIMEType)
	}
	if !strings.Contains(shell.status, filepath.Base(newer)) {
		t.Fatalf("status = %q, want the shared backup name", shell.status)
	}
}

func TestOpenSaveBackupFolderReportsAnEmptyBackupFolder(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	previousFolder := openArtifactFolder
	openArtifactFolder = func(string) error { return ErrFolderBrowserUnavailable }
	t.Cleanup(func() { openArtifactFolder = previousFolder })

	host := &recordingShareHost{}
	useShareHost(t, host)

	shell := newSaveShell(&saveTransferTestBackend{})
	shell.openSaveBackupFolder()

	if len(host.paths) != 0 {
		t.Fatalf("host was handed %v from an empty folder", host.paths)
	}
	if !strings.Contains(shell.status, "no backup") {
		t.Fatalf("status = %q, want an empty-folder message", shell.status)
	}
}

// TestExportSaveDataOffersTheBackupToTheHost proves a fresh backup leaves the
// app on a host that can share, and that a desktop host - which cannot - is
// left with its own folder-based status.
func TestExportSaveDataOffersTheBackupToTheHost(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	host := &recordingShareHost{}
	useShareHost(t, host)

	shell := newSaveShell(&saveTransferTestBackend{exportData: []byte("BACKUP")})
	shell.input = &InputInfo{DisplayName: "이노티아1.zip", SHA256: "93d5b6b8"}
	shell.exportSaveData()

	result := <-shell.artifactResults
	if result.err != nil {
		t.Fatalf("export reported error: %v", result.err)
	}
	if result.shareMIME != saveBackupMIMEType {
		t.Fatalf("shareMIME = %q, want %q", result.shareMIME, saveBackupMIMEType)
	}
	shell.offerArtifact(result)
	if len(host.paths) != 1 || host.paths[0] != result.path {
		t.Fatalf("host was handed %v, want %q", host.paths, result.path)
	}

	SetNativeShareHost(nil)
	shell.status = ""
	shell.offerArtifact(result)
	if shell.status != "" {
		t.Fatalf("a host that cannot share reported %q", shell.status)
	}
}

// TestOfferArtifactIgnoresArtifactsWithoutAShareType keeps screenshots, logs
// and debug bundles on their existing "saved: path" behaviour.
func TestOfferArtifactIgnoresArtifactsWithoutAShareType(t *testing.T) {
	host := &recordingShareHost{}
	useShareHost(t, host)

	shell := newSaveShell(&saveTransferTestBackend{})
	shell.offerArtifact(artifactResult{kind: "Screenshot", path: "aram.png"})
	if len(host.paths) != 0 {
		t.Fatalf("host was handed %v for an artifact with no share type", host.paths)
	}
}
