package frontend

import (
	"context"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Panel struct {
	Kind            string
	Tool            ToolKind
	Title           string
	Lines           []string
	Fields          []ToolField
	Actions         []ToolAction
	FieldValues     map[string]string
	Busy            bool
	AllowGuestInput bool
}

// guestInputAllowed reports whether host controls still reach the guest. Any
// open panel captures input unless it opted out, so a cheat can be toggled
// without the game losing the keypress that advances it.
func (s *Shell) guestInputAllowed() bool {
	return s.panel == nil || s.panel.AllowGuestInput
}

type toolResult struct {
	kind     ToolKind
	snapshot ToolSnapshot
	err      error
}

func (s *Shell) openToolPanel(kind ToolKind) {
	if kind == ToolLogs {
		s.panel = &Panel{Kind: "logs", Tool: kind, Title: "Logs"}
		return
	}
	title := toolTitle(kind)
	s.panel = &Panel{
		Kind:  "tool",
		Tool:  kind,
		Title: title,
		Lines: []string{"Loading backend capability..."},
	}
	backend, ok := s.backend.(ToolBackend)
	if !ok {
		s.panel.Lines = []string{
			"The current backend does not expose this panel.",
			s.trf(
				"The checked %s service is unavailable.",
				s.tr(toolTitle(kind)),
			),
			"Guest memory is never read directly by the frontend.",
			"Connect an integration backend that implements ToolBackend.",
		}
		return
	}
	go func() {
		snapshot, err := backend.ToolSnapshot(context.Background(), kind)
		s.toolResults <- toolResult{kind: kind, snapshot: snapshot, err: err}
	}()
}

func (s *Shell) consumeToolResult(result toolResult) {
	if s.panel == nil || s.panel.Tool != result.kind {
		return
	}
	if result.err != nil {
		s.panel.Busy = false
		s.panel.Lines = []string{
			"Backend tool request failed:",
			"",
			result.err.Error(),
		}
		s.setStatus(s.trf(
			"%s: %s",
			s.tr(toolTitle(result.kind)),
			result.err.Error(),
		))
		return
	}
	if result.snapshot.Title != "" {
		s.panel.Title = result.snapshot.Title
	}
	s.panel.Lines = append([]string(nil), result.snapshot.Lines...)
	s.panel.Fields = append([]ToolField(nil), result.snapshot.Fields...)
	s.panel.Actions = append([]ToolAction(nil), result.snapshot.Actions...)
	s.panel.AllowGuestInput = result.snapshot.AllowGuestInput
	s.panel.FieldValues = make(map[string]string, len(result.snapshot.Fields))
	for _, field := range result.snapshot.Fields {
		s.panel.FieldValues[field.ID] = field.Value
	}
	s.panel.Busy = false
	s.setStatus(s.trf(
		"%s refreshed",
		s.tr(toolTitle(result.kind)),
	))
}

func (s *Shell) executeToolAction(action string, fields map[string]string) {
	if s.panel == nil || s.panel.Kind != "tool" || s.panel.Busy {
		return
	}
	backend, ok := s.backend.(ToolActionBackend)
	if !ok {
		s.setStatus(s.trf(
			"%s: backend actions are unavailable",
			s.tr(toolTitle(s.panel.Tool)),
		))
		return
	}
	request := ToolRequest{
		Kind:   s.panel.Tool,
		Action: action,
		Fields: cloneStringMap(fields),
	}
	s.panel.Busy = true
	s.setStatus(s.trf(
		"%s: %s...",
		s.tr(toolTitle(s.panel.Tool)),
		s.tr(action),
	))
	go func() {
		snapshot, err := backend.ExecuteToolAction(context.Background(), request)
		s.toolResults <- toolResult{kind: request.Kind, snapshot: snapshot, err: err}
	}()
}

func (s *Shell) openControllerPanel() {
	s.openSettingsSection("Controls")
}

func (s *Shell) openAudioPanel() {
	s.openSettingsSection("Audio")
}

func (s *Shell) openUpdatesPanel() {
	s.openSettingsSection("Updates")
}

func (s *Shell) openSettingsPanel() {
	s.openSettingsSection("General")
}

func (s *Shell) openSettingsSection(section string) {
	s.settingsSection = section
	if s.interfaceUI != nil {
		s.interfaceUI.settingsSection = section
		s.interfaceUI.panelSignature = ""
	}
	s.panel = &Panel{
		Kind:  "settings",
		Title: "Configure ARAM",
	}
}

func (s *Shell) openPropertiesPanel() {
	s.panel = &Panel{
		Kind:  "properties",
		Title: "Title Properties",
	}
}

func (s *Shell) openCompatibilityPanel() {
	s.panel = &Panel{
		Kind:  "compatibility",
		Tool:  ToolCompatibility,
		Title: "Compatibility Report",
	}
}

func (s *Shell) handlePanelShortcuts(control bool) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.panel = nil
		return
	}
	switch s.panel.Kind {
	case "compatibility":
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			s.saveCompatibilityReport()
		}
	case "tool":
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			kind := s.panel.Tool
			s.openToolPanel(kind)
		}
	case "logs":
		if control && inpututil.IsKeyJustPressed(ebiten.KeyS) {
			s.saveLog()
		}
	}
}
