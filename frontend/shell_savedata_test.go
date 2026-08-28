package frontend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// saveTransferTestBackend is a NullBackend that also offers the save backup
// contract, so shell export/import can be driven without a real emulator.
type saveTransferTestBackend struct {
	NullBackend
	exportData []byte
	exportErr  error
	imported   []byte
	importErr  error
}

func (b *saveTransferTestBackend) ExportSaveData() ([]byte, error) {
	return b.exportData, b.exportErr
}

func (b *saveTransferTestBackend) ImportSaveData(data []byte) error {
	b.imported = append([]byte(nil), data...)
	return b.importErr
}

func newSaveShell(backend Backend) *Shell {
	settings := defaultSettings()
	settings.Language = string(LanguageEnglish)
	return &Shell{
		backend:            backend,
		settings:           settings,
		artifactResults:    make(chan artifactResult, 4),
		saveRestoreResults: make(chan saveRestoreResult, 2),
	}
}

func TestExportSaveDataWritesBackupFile(t *testing.T) {
	config := t.TempDir()
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	backend := &saveTransferTestBackend{exportData: []byte("BACKUP-BYTES")}
	shell := newSaveShell(backend)
	shell.input = &InputInfo{DisplayName: "에픽크로니클PE.zip", SHA256: "abcdef0123456789"}

	shell.exportSaveData()
	result := <-shell.artifactResults
	if result.err != nil {
		t.Fatalf("export reported error: %v", result.err)
	}
	if filepath.Base(filepath.Dir(result.path)) != "save-backups" {
		t.Fatalf("backup not written under save-backups: %q", result.path)
	}
	if filepath.Ext(result.path) != ".aramsave" {
		t.Fatalf("backup extension = %q, want .aramsave", filepath.Ext(result.path))
	}
	if !strings.Contains(filepath.Base(result.path), "abcdef01") {
		t.Fatalf("backup name %q does not carry the title identity", result.path)
	}
	written, err := os.ReadFile(result.path)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(written) != "BACKUP-BYTES" {
		t.Fatalf("backup contents = %q, want %q", written, "BACKUP-BYTES")
	}
}

func TestExportSaveDataWithoutTitleReportsStatus(t *testing.T) {
	shell := newSaveShell(&saveTransferTestBackend{})
	shell.exportSaveData()
	if !strings.Contains(shell.status, "no title") {
		t.Fatalf("status = %q, want a no-title message", shell.status)
	}
}

func TestExportSaveDataUnsupportedBackendReportsStatus(t *testing.T) {
	shell := newSaveShell(NullBackend{})
	shell.input = &InputInfo{DisplayName: "title", SHA256: "00"}
	shell.exportSaveData()
	if !strings.Contains(shell.status, "does not support") {
		t.Fatalf("status = %q, want an unsupported-backend message", shell.status)
	}
}

func TestImportSaveDataFromPathRestoresBackup(t *testing.T) {
	backend := &saveTransferTestBackend{}
	shell := newSaveShell(backend)
	shell.input = &InputInfo{DisplayName: "title", SHA256: "00"}

	path := filepath.Join(t.TempDir(), "save.aramsave")
	if err := os.WriteFile(path, []byte("RESTORE-BYTES"), 0o600); err != nil {
		t.Fatal(err)
	}

	shell.importSaveDataFromPath(path)
	result := <-shell.saveRestoreResults
	if result.err != nil {
		t.Fatalf("restore reported error: %v", result.err)
	}
	if string(backend.imported) != "RESTORE-BYTES" {
		t.Fatalf("backend received %q, want %q", backend.imported, "RESTORE-BYTES")
	}
	if result.name != "save.aramsave" {
		t.Fatalf("restore name = %q", result.name)
	}
}

func TestImportSaveDataFromMissingFileReportsStatus(t *testing.T) {
	shell := newSaveShell(&saveTransferTestBackend{})
	shell.input = &InputInfo{DisplayName: "title", SHA256: "00"}
	shell.importSaveDataFromPath(filepath.Join(t.TempDir(), "absent.aramsave"))
	if !strings.Contains(shell.status, "Restore save") {
		t.Fatalf("status = %q, want a restore error", shell.status)
	}
}

func TestConsumePickerResultRoutesSaveImport(t *testing.T) {
	backend := &saveTransferTestBackend{}
	shell := newSaveShell(backend)
	shell.input = &InputInfo{DisplayName: "title", SHA256: "00"}
	shell.dialogOpen = true

	path := filepath.Join(t.TempDir(), "slot.aramsave")
	if err := os.WriteFile(path, []byte("PICKED"), 0o600); err != nil {
		t.Fatal(err)
	}
	shell.consumePickerResult(pickerResult{operation: operationImportSave, path: path})
	if shell.dialogOpen {
		t.Fatal("dialog stayed open after a save import selection")
	}
	result := <-shell.saveRestoreResults
	if result.err != nil {
		t.Fatalf("routed restore errored: %v", result.err)
	}
	if string(backend.imported) != "PICKED" {
		t.Fatalf("routed restore delivered %q", backend.imported)
	}
}

func TestSaveBackupBaseNamePreservesUnicodeAndAddsIdentity(t *testing.T) {
	name := saveBackupBaseName("에픽크로니클PE.zip", "0123456789abcdef")
	if !strings.HasPrefix(name, "에픽크로니클PE") {
		t.Fatalf("base name dropped the Korean title: %q", name)
	}
	if !strings.HasSuffix(name, "-01234567") {
		t.Fatalf("base name lost the short identity: %q", name)
	}
	if strings.ContainsAny(name, `\/:*?"<>| `) {
		t.Fatalf("base name kept a reserved character: %q", name)
	}
}
