package frontend

import (
	"os"
	"path/filepath"
	"strings"
)

// saveRestoreResult reports the outcome of an asynchronous save restore so the
// shell can show a status on its own update loop rather than from a goroutine.
type saveRestoreResult struct {
	name string
	err  error
}

// exportSaveData writes the loaded title's save (its writable storage) to a
// portable backup file under the ARAM save-backups folder, so the save survives
// loss of the local state directory and can be kept anywhere.
func (s *Shell) exportSaveData() {
	if s.input == nil {
		s.setStatus(s.tr("Save backup: no title is loaded"))
		return
	}
	transfer, ok := s.backend.(SaveTransferBackend)
	if !ok {
		s.setStatus(s.tr("Save backup: this input does not support save backup"))
		return
	}
	prefix := saveBackupBaseName(s.input.DisplayName, s.input.SHA256)
	s.setStatus(s.tr("Backing up save..."))
	go func() {
		var path string
		data, err := transfer.ExportSaveData()
		if err == nil {
			path, err = writeTextArtifact("save-backups", prefix, ".aramsave", data)
		}
		s.artifactResults <- artifactResult{kind: "Save backup", path: path, err: err}
	}()
}

// openSaveBackupFolder reveals the folder the backups are written to, so a user
// can copy a backup out to cloud storage or a USB drive - the copy that actually
// survives losing the local state directory.
func (s *Shell) openSaveBackupFolder() {
	path, err := artifactDirectory("save-backups")
	if err == nil {
		err = openArtifactFolder(path)
	}
	if err != nil {
		message := s.tr("Save backup folder: ") + err.Error()
		s.appendLog(message)
		s.setStatus(message)
		return
	}
	message := s.trf("Save backup folder opened: %s", path)
	s.appendLog(message)
	s.setStatus(message)
}

// importSaveData prompts for a save backup file and restores it into the loaded
// title. The picker runs off the update loop, exactly like opening an input.
func (s *Shell) importSaveData() {
	if s.dialogOpen || s.loading {
		return
	}
	if s.input == nil {
		s.setStatus(s.tr("Restore save: no title is loaded"))
		return
	}
	if _, ok := s.backend.(SaveTransferBackend); !ok {
		s.setStatus(s.tr("Restore save: this input does not support save restore"))
		return
	}
	s.preDialogState = s.state
	s.state = FrontendSelecting
	s.dialogOpen = true
	s.setStatus(s.tr("Waiting for save backup selection..."))
	go func() {
		path, err := s.picker.OpenSaveBackupFile()
		s.pickerResults <- pickerResult{operation: operationImportSave, path: path, err: err}
	}()
}

// importSaveDataFromPath reads a chosen backup file and applies it to the loaded
// title. Reading is quick; the backend restore is run off the update loop
// because it serializes with guest frame stepping.
func (s *Shell) importSaveDataFromPath(path string) {
	transfer, ok := s.backend.(SaveTransferBackend)
	if !ok {
		s.setStatus(s.tr("Restore save: this input does not support save restore"))
		return
	}
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		s.setStatus(s.tr("Restore save: ") + err.Error())
		return
	}
	s.setStatus(s.trf("Restoring save from %s...", name))
	go func() {
		s.saveRestoreResults <- saveRestoreResult{name: name, err: transfer.ImportSaveData(data)}
	}()
}

// saveBackupBaseName builds a filesystem-safe backup name from a title's
// display name and a short slice of its identity, so backups for different
// titles never collide.
func saveBackupBaseName(displayName, hash string) string {
	base := sanitizeSaveBackupName(displayName)
	if base == "" {
		base = "aram"
	}
	if len(hash) > 8 {
		hash = hash[:8]
	}
	if hash != "" {
		base = base + "-" + hash
	}
	return base
}

// sanitizeSaveBackupName strips only the characters a filename cannot carry
// (path separators, reserved punctuation, control characters, spaces), so a
// Korean title keeps its own letters in the backup name instead of a row of
// underscores.
func sanitizeSaveBackupName(name string) string {
	name = strings.TrimSpace(name)
	if ext := filepath.Ext(name); ext != "" && len(ext) <= 5 {
		name = strings.TrimSuffix(name, ext)
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, name)
}
