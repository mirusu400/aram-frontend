package frontend

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	debugBundleSchemaVersion = 1
	debugArtifactSizeLimit   = 8 << 20
	debugBundleSizeLimit     = 32 << 20
	debugCollectionTimeout   = 10 * time.Second
	debugGoroutineDumpLimit  = 8 << 20
)

// captureGoroutineDump returns a stack dump of every goroutine, capped so a
// pathological process cannot produce an unbounded artifact. It is the key
// signal for a hang or deadlock, where the point-in-time state says nothing
// about which goroutine is stuck and why.
func captureGoroutineDump() string {
	buffer := make([]byte, 1<<20)
	for {
		written := runtime.Stack(buffer, true)
		if written < len(buffer) {
			return string(buffer[:written])
		}
		if len(buffer) >= debugGoroutineDumpLimit {
			return string(buffer)
		}
		buffer = make([]byte, 2*len(buffer))
	}
}

type debugBundleSnapshot struct {
	CreatedAt     time.Time
	Input         *InputInfo
	Backend       string
	BackendState  BackendState
	FrontendState FrontendState
	Problem       *FrontendProblem
	Settings      debugSettingsReport
	Pacing        debugPacingReport
	Build         debugBuildReport
	FrontendLogs  []string
	Audio         AudioQueueTelemetry
	AudioTrace    []byte
	CPUProfile    []byte
	CrashReport   []byte
	Redactions    []string
	Screenshot    *image.RGBA
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

type debugSessionReport struct {
	Input         *debugInputReport   `json:"input,omitempty"`
	Backend       string              `json:"backend"`
	BackendState  BackendState        `json:"backend_state"`
	FrontendState FrontendState       `json:"frontend_state"`
	Problem       *debugProblemReport `json:"problem,omitempty"`
	Settings      debugSettingsReport `json:"settings"`
	Pacing        debugPacingReport   `json:"pacing"`
	Audio         AudioQueueTelemetry `json:"audio"`
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
	Language              string  `json:"language"`
	CPU                   string  `json:"cpu"`
	Speed                 float64 `json:"speed"`
	StateSlot             int     `json:"state_slot"`
	Theme                 string  `json:"theme"`
	IntegerScaling        bool    `json:"integer_scaling"`
	PreserveAspect        bool    `json:"preserve_aspect"`
	Rotation              int     `json:"rotation"`
	ScreenLayout          string  `json:"screen_layout"`
	Filter                string  `json:"filter"`
	DisplayEffect         string  `json:"display_effect"`
	DisplayEffectStrength int     `json:"display_effect_strength"`
	Muted                 bool    `json:"muted"`
	Volume                int     `json:"volume"`
	AudioLatencyMS        int     `json:"audio_latency_ms"`
	AudioMixMode          bool    `json:"audio_mix_mode"`
	UIPriority            bool    `json:"ui_priority"`
}

// debugPacingReport answers the first question a "too slow" report raises: is
// the host keeping up with the guest at all, and by how much is it missing?
//
// The settings screen already shows the achieved ratio beside the requested
// one, but a bundle carried neither, so a report of a slow title could not be
// separated from a title that is slow by design without asking the reporter to
// read the number off their own screen (aram-core#127). RequestedSpeed is the
// ratio of guest time to real time the user asked for; MeasuredSpeed is the
// ratio the host actually delivered over the pacing window, and is zero before
// the first window closes. AchievedPercent is the second over the first.
type debugPacingReport struct {
	RequestedSpeed float64 `json:"requested_speed"`
	// EffectiveSpeed is the ratio the pacer is actually aiming for: the
	// display-sync rate while engaged, otherwise RequestedSpeed.
	EffectiveSpeed    float64 `json:"effective_speed"`
	MeasuredSpeed     float64 `json:"measured_speed"`
	AchievedPercent   float64 `json:"achieved_percent"`
	UIPriority        bool    `json:"ui_priority"`
	DisplaySync       bool    `json:"display_sync"`
	DisplaySyncActive bool    `json:"display_sync_active"`
	HostTickRate      float64 `json:"host_tick_rate"`
	VsyncDisabled     bool    `json:"vsync_disabled"`
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

func collectDebugBundle(
	snapshot debugBundleSnapshot,
	backend Backend,
) (string, string, error) {
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
	return writeDebugBundle(snapshot, artifacts, collectErr)
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
	files = append(files, debugBundleFile{
		name:      "goroutines.txt",
		source:    "frontend",
		mediaType: "text/plain; charset=utf-8",
		data:      []byte(redactDebugText(captureGoroutineDump(), snapshot.Redactions)),
	})
	if len(snapshot.AudioTrace) != 0 {
		files = append(files, debugBundleFile{
			name:      "audio-trace.log",
			source:    "frontend",
			mediaType: "text/plain; charset=utf-8",
			data:      snapshot.AudioTrace,
		})
	}
	if len(snapshot.CPUProfile) != 0 {
		files = append(files, debugBundleFile{
			name:      "cpu.pprof",
			source:    "frontend",
			mediaType: "application/octet-stream",
			data:      snapshot.CPUProfile,
		})
	}
	if len(snapshot.CrashReport) != 0 {
		files = append(files, debugBundleFile{
			name:      "crash.txt",
			source:    "frontend",
			mediaType: "text/plain; charset=utf-8",
			data:      snapshot.CrashReport,
		})
	}
	if snapshot.Screenshot != nil {
		var screenshot bytes.Buffer
		if err := png.Encode(&screenshot, snapshot.Screenshot); err != nil {
			return "", "", fmt.Errorf("encode debug screenshot: %w", err)
		}
		files = append(files, debugBundleFile{
			name:      "screenshot.png",
			source:    "frontend",
			mediaType: "image/png",
			data:      screenshot.Bytes(),
		})
	}
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
			Pacing:        snapshot.Pacing,
			Audio:         snapshot.Audio,
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
		"manifest.json":  true,
		"frontend.log":   true,
		"screenshot.png": true,
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
