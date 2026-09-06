package frontend

import (
	"encoding/json"
	"strings"
	"time"
)

type artifactResult struct {
	kind    string
	path    string
	warning string
	err     error
}

type dropResult struct {
	path        string
	data        []byte
	displayName string
	// temporary marks path as a private cache copy the shell removes once the
	// input is released, as opposed to the dropped file's own location.
	temporary bool
	err       error
}

type compatibilityReport struct {
	CreatedAt     time.Time        `json:"created_at"`
	Input         *InputInfo       `json:"input,omitempty"`
	InputPath     string           `json:"input_path,omitempty"`
	Backend       string           `json:"backend"`
	BackendState  BackendState     `json:"backend_state"`
	FrontendState FrontendState    `json:"frontend_state"`
	Problem       *FrontendProblem `json:"problem,omitempty"`
	StateSlot     int              `json:"state_slot"`
	Speed         float64          `json:"speed"`
}

func (s *Shell) saveScreenshot() {
	frame := s.currentFrame()
	if frame.Image == nil {
		s.setStatus(s.tr("Screenshot: no guest-native frame is available"))
		return
	}
	snapshot := cloneImage(frame.Image)
	go func() {
		path, err := writePNGArtifact("screenshots", "aram", snapshot)
		s.artifactResults <- artifactResult{kind: "Screenshot", path: path, err: err}
	}()
}

func (s *Shell) saveCompatibilityReport() {
	if s.input == nil {
		s.setStatus(s.tr("Compatibility report: no input is selected"))
		return
	}
	report := compatibilityReport{
		CreatedAt:     time.Now().UTC(),
		Input:         s.input,
		InputPath:     s.selectedPath,
		Backend:       s.backendName(),
		BackendState:  s.backend.State(),
		FrontendState: s.state,
		Problem:       s.problem,
		StateSlot:     s.settings.StateSlot,
		Speed:         s.settings.Speed,
	}
	go func() {
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
		var path string
		if err == nil {
			path, err = writeTextArtifact("reports", "compatibility", ".json", data)
		}
		s.artifactResults <- artifactResult{kind: "Compatibility report", path: path, err: err}
	}()
}

func (s *Shell) saveLog() {
	data := []byte(strings.Join(s.logs, "\n") + "\n")
	go func() {
		path, err := writeTextArtifact("logs", "frontend", ".log", data)
		s.artifactResults <- artifactResult{kind: "Log", path: path, err: err}
	}()
}

func (s *Shell) openDebugBundleFolder() {
	path, err := artifactDirectory("debug")
	if err == nil {
		err = openArtifactFolder(path)
	}
	if err != nil {
		message := s.tr("Debug bundle folder: ") + err.Error()
		s.appendLog(message)
		s.setStatus(message)
		return
	}
	message := s.trf("Debug bundle folder opened: %s", path)
	s.appendLog(message)
	s.setStatus(message)
}
