package frontend

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	runtimedebug "runtime/debug"
	"slices"
	"strings"
	"time"
)

const (
	debugBundleSchemaVersion = 1
	debugArtifactSizeLimit   = 8 << 20
	debugBundleSizeLimit     = 32 << 20
	debugCollectionTimeout   = 10 * time.Second
)

type debugBundleSnapshot struct {
	CreatedAt     time.Time
	Input         *InputInfo
	Backend       string
	BackendState  BackendState
	FrontendState FrontendState
	Problem       *FrontendProblem
	Settings      debugSettingsReport
	Build         debugBuildReport
	FrontendLogs  []string
	Redactions    []string
}

type debugBundleManifest struct {
	SchemaVersion          int                `json:"schema_version"`
	CreatedAt              time.Time          `json:"created_at"`
	Host                   debugHostReport    `json:"host"`
	Build                  debugBuildReport   `json:"build"`
	Session                debugSessionReport `json:"session"`
	Files                  []debugFileReport  `json:"files"`
	BackendCollectionError string             `json:"backend_collection_error,omitempty"`
}

type debugHostReport struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

type debugBuildReport struct {
	Main       debugModuleReport   `json:"main"`
	Components []debugModuleReport `json:"components,omitempty"`
	VCS        string              `json:"vcs,omitempty"`
	Revision   string              `json:"revision,omitempty"`
	Time       string              `json:"time,omitempty"`
	Modified   bool                `json:"modified,omitempty"`
}

type debugModuleReport struct {
	Path        string             `json:"path"`
	Version     string             `json:"version,omitempty"`
	Sum         string             `json:"sum,omitempty"`
	Replacement *debugModuleReport `json:"replacement,omitempty"`
}

type debugSessionReport struct {
	Input         *debugInputReport   `json:"input,omitempty"`
	Backend       string              `json:"backend"`
	BackendState  BackendState        `json:"backend_state"`
	FrontendState FrontendState       `json:"frontend_state"`
	Problem       *debugProblemReport `json:"problem,omitempty"`
	Settings      debugSettingsReport `json:"settings"`
}

type debugInputReport struct {
	DisplayName string `json:"display_name"`
	Format      string `json:"format"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ProfileID   string `json:"profile_id"`
}

type debugProblemReport struct {
	State       FrontendState `json:"state"`
	Input       string        `json:"input,omitempty"`
	Format      string        `json:"format,omitempty"`
	Profile     string        `json:"profile,omitempty"`
	Backend     string        `json:"backend,omitempty"`
	Reason      string        `json:"reason"`
	Recoverable bool          `json:"recoverable"`
}

type debugSettingsReport struct {
	Language       string  `json:"language"`
	Speed          float64 `json:"speed"`
	StateSlot      int     `json:"state_slot"`
	Theme          string  `json:"theme"`
	IntegerScaling bool    `json:"integer_scaling"`
	PreserveAspect bool    `json:"preserve_aspect"`
	Rotation       int     `json:"rotation"`
	ScreenLayout   string  `json:"screen_layout"`
	Filter         string  `json:"filter"`
	Muted          bool    `json:"muted"`
	Volume         int     `json:"volume"`
	AudioLatencyMS int     `json:"audio_latency_ms"`
}

type debugFileReport struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
	SHA256    string `json:"sha256"`
}

type debugBundleFile struct {
	name      string
	source    string
	mediaType string
	data      []byte
}

func (s *Shell) saveDebugBundle() {
	s.setStatus(s.tr("Collecting debug bundle..."))
	snapshot := s.captureDebugBundleSnapshot(time.Now().UTC())
	backend := s.backend
	go func() {
		var (
			artifacts  []DebugArtifact
			collectErr error
		)
		if exporter, ok := backend.(DebugExportBackend); ok {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				debugCollectionTimeout,
			)
			artifacts, collectErr = exporter.DebugArtifacts(ctx)
			cancel()
		}
		path, warning, err := writeDebugBundle(snapshot, artifacts, collectErr)
		s.artifactResults <- artifactResult{
			kind:    "Debug bundle",
			path:    path,
			warning: warning,
			err:     err,
		}
	}()
}

func (s *Shell) captureDebugBundleSnapshot(createdAt time.Time) debugBundleSnapshot {
	var input *InputInfo
	if s.input != nil {
		value := *s.input
		input = &value
	}
	var problem *FrontendProblem
	if s.problem != nil {
		value := *s.problem
		problem = &value
	}

	redactions := debugRedactionRoots(s.selectedPath)
	logs := append([]string(nil), s.logs...)
	for index := range logs {
		logs[index] = redactDebugText(logs[index], redactions)
	}
	if problem != nil {
		problem.Reason = redactDebugText(problem.Reason, redactions)
	}
	build := currentDebugBuildReport()
	redactDebugBuildReport(&build, redactions)

	return debugBundleSnapshot{
		CreatedAt:     createdAt,
		Input:         input,
		Backend:       s.backendName(),
		BackendState:  s.backend.State(),
		FrontendState: s.state,
		Problem:       problem,
		Settings: debugSettingsReport{
			Language:       s.settings.Language,
			Speed:          s.settings.Speed,
			StateSlot:      s.settings.StateSlot,
			Theme:          s.settings.ThemeMode,
			IntegerScaling: s.settings.IntegerScaling,
			PreserveAspect: s.settings.PreserveAspect,
			Rotation:       s.settings.Rotation,
			ScreenLayout:   s.settings.ScreenLayout,
			Filter:         s.settings.Filter,
			Muted:          s.settings.Muted,
			Volume:         s.settings.Volume,
			AudioLatencyMS: s.settings.AudioLatencyMS,
		},
		Build:        build,
		FrontendLogs: logs,
		Redactions:   redactions,
	}
}

func writeDebugBundle(
	snapshot debugBundleSnapshot,
	backendArtifacts []DebugArtifact,
	backendErr error,
) (string, string, error) {
	frontendLog := []byte(strings.Join(snapshot.FrontendLogs, "\n"))
	if len(frontendLog) != 0 {
		frontendLog = append(frontendLog, '\n')
	}
	files := []debugBundleFile{{
		name:      "frontend.log",
		source:    "frontend",
		mediaType: "text/plain; charset=utf-8",
		data:      frontendLog,
	}}
	warnings := make([]string, 0, 2)
	if backendErr != nil {
		warnings = append(
			warnings,
			redactDebugText(backendErr.Error(), snapshot.Redactions),
		)
	}
	validBackend, validationWarnings := validateDebugArtifacts(backendArtifacts)
	for index := range validationWarnings {
		validationWarnings[index] = redactDebugText(
			validationWarnings[index],
			snapshot.Redactions,
		)
	}
	warnings = append(warnings, validationWarnings...)
	files = append(files, validBackend...)

	manifest := debugBundleManifest{
		SchemaVersion: debugBundleSchemaVersion,
		CreatedAt:     snapshot.CreatedAt,
		Host: debugHostReport{
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
		},
		Build: snapshot.Build,
		Session: debugSessionReport{
			Input:         debugInput(snapshot.Input),
			Backend:       snapshot.Backend,
			BackendState:  snapshot.BackendState,
			FrontendState: snapshot.FrontendState,
			Problem:       debugProblem(snapshot.Problem),
			Settings:      snapshot.Settings,
		},
		Files: make([]debugFileReport, 0, len(files)),
	}
	if len(warnings) != 0 {
		manifest.BackendCollectionError = strings.Join(warnings, "; ")
	}
	for _, file := range files {
		sum := sha256.Sum256(file.data)
		manifest.Files = append(manifest.Files, debugFileReport{
			Name:      file.name,
			Source:    file.source,
			MediaType: file.mediaType,
			Size:      len(file.data),
			SHA256:    hex.EncodeToString(sum[:]),
		})
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode debug manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	files = append([]debugBundleFile{{
		name:      "manifest.json",
		source:    "frontend",
		mediaType: "application/json",
		data:      manifestData,
	}}, files...)

	path, err := writeDebugZIP(snapshot.CreatedAt, files)
	if err != nil {
		return "", "", err
	}
	return path, manifest.BackendCollectionError, nil
}

func validateDebugArtifacts(artifacts []DebugArtifact) ([]debugBundleFile, []string) {
	files := make([]debugBundleFile, 0, len(artifacts))
	warnings := make([]string, 0)
	names := map[string]bool{
		"manifest.json": true,
		"frontend.log":  true,
	}
	total := 0
	for _, artifact := range artifacts {
		name := strings.TrimSpace(artifact.Name)
		switch {
		case !validDebugArtifactName(name):
			warnings = append(
				warnings,
				fmt.Sprintf("ignored invalid debug artifact name %q", name),
			)
			continue
		case names[name]:
			warnings = append(
				warnings,
				fmt.Sprintf("ignored duplicate debug artifact %q", name),
			)
			continue
		case len(artifact.Data) > debugArtifactSizeLimit:
			warnings = append(
				warnings,
				fmt.Sprintf("ignored oversized debug artifact %q", name),
			)
			continue
		case total > debugBundleSizeLimit-len(artifact.Data):
			warnings = append(
				warnings,
				fmt.Sprintf(
					"ignored debug artifact %q: bundle size limit exceeded",
					name,
				),
			)
			continue
		}
		mediaType := strings.TrimSpace(artifact.MediaType)
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		names[name] = true
		total += len(artifact.Data)
		files = append(files, debugBundleFile{
			name:      name,
			source:    "backend",
			mediaType: mediaType,
			data:      artifact.Data,
		})
	}
	return files, warnings
}

func validDebugArtifactName(name string) bool {
	if name == "" ||
		name == "." ||
		name == ".." ||
		filepath.Base(name) != name ||
		strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	for _, character := range name {
		if character > 127 ||
			!(character >= 'a' && character <= 'z') &&
				!(character >= 'A' && character <= 'Z') &&
				!(character >= '0' && character <= '9') &&
				!strings.ContainsRune("._-", character) {
			return false
		}
	}
	return true
}

func writeDebugZIP(
	createdAt time.Time,
	files []debugBundleFile,
) (path string, returnedErr error) {
	root, err := artifactDirectory("debug")
	if err != nil {
		return "", err
	}
	path = filepath.Join(root, timestampedName("aram-debug", ".zip"))
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer func() {
		if returnedErr != nil {
			_ = output.Close()
			_ = os.Remove(path)
		}
	}()

	archive := zip.NewWriter(output)
	for _, file := range files {
		header := &zip.FileHeader{
			Name:     file.name,
			Method:   zip.Deflate,
			Modified: createdAt,
		}
		header.SetMode(0o600)
		entry, createErr := archive.CreateHeader(header)
		if createErr != nil {
			_ = archive.Close()
			return "", fmt.Errorf(
				"create debug bundle entry %q: %w",
				file.name,
				createErr,
			)
		}
		if _, writeErr := entry.Write(file.data); writeErr != nil {
			_ = archive.Close()
			return "", fmt.Errorf(
				"write debug bundle entry %q: %w",
				file.name,
				writeErr,
			)
		}
	}
	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("close debug bundle: %w", err)
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("close debug bundle file: %w", err)
	}
	return path, nil
}

func currentDebugBuildReport() debugBuildReport {
	info, ok := runtimedebug.ReadBuildInfo()
	if !ok {
		return debugBuildReport{}
	}
	report := debugBuildReport{Main: debugModule(info.Main)}
	for _, dependency := range info.Deps {
		if dependency == nil ||
			dependency.Path != "github.com/mirusu400/aram-core" &&
				dependency.Path != "github.com/mirusu400/aram-frontend" {
			continue
		}
		report.Components = append(
			report.Components,
			debugModule(*dependency),
		)
	}
	slices.SortFunc(
		report.Components,
		func(left, right debugModuleReport) int {
			return strings.Compare(left.Path, right.Path)
		},
	)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs":
			report.VCS = setting.Value
		case "vcs.revision":
			report.Revision = setting.Value
		case "vcs.time":
			report.Time = setting.Value
		case "vcs.modified":
			report.Modified = setting.Value == "true"
		}
	}
	return report
}

func debugModule(module runtimedebug.Module) debugModuleReport {
	report := debugModuleReport{
		Path:    module.Path,
		Version: module.Version,
		Sum:     module.Sum,
	}
	if module.Replace != nil {
		replacement := debugModule(*module.Replace)
		report.Replacement = &replacement
	}
	return report
}

func redactDebugBuildReport(report *debugBuildReport, roots []string) {
	redactDebugModule(&report.Main, roots)
	for index := range report.Components {
		redactDebugModule(&report.Components[index], roots)
	}
}

func redactDebugModule(module *debugModuleReport, roots []string) {
	if module == nil {
		return
	}
	module.Path = redactDebugText(module.Path, roots)
	redactDebugModule(module.Replacement, roots)
}

func debugInput(input *InputInfo) *debugInputReport {
	if input == nil {
		return nil
	}
	return &debugInputReport{
		DisplayName: input.DisplayName,
		Format:      input.Format,
		Size:        input.Size,
		SHA256:      input.SHA256,
		ProfileID:   input.ProfileID,
	}
}

func debugProblem(problem *FrontendProblem) *debugProblemReport {
	if problem == nil {
		return nil
	}
	return &debugProblemReport{
		State:       problem.State,
		Input:       problem.Input,
		Format:      problem.Format,
		Profile:     problem.Profile,
		Backend:     problem.Backend,
		Reason:      problem.Reason,
		Recoverable: problem.Recoverable,
	}
}

func debugRedactionRoots(selectedPath string) []string {
	roots := []string{selectedPath}
	if path, err := os.UserHomeDir(); err == nil {
		roots = append(roots, path)
	}
	if path, err := os.UserConfigDir(); err == nil {
		roots = append(roots, path)
	}
	if path, err := os.UserCacheDir(); err == nil {
		roots = append(roots, path)
	}
	slices.SortFunc(roots, func(left, right string) int {
		lengthOrder := len(right) - len(left)
		if lengthOrder != 0 {
			return lengthOrder
		}
		return strings.Compare(left, right)
	})
	return slices.Compact(roots)
}

func redactDebugText(value string, roots []string) string {
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		value = strings.ReplaceAll(value, root, "<redacted-path>")
	}
	return value
}
