package frontend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Panel struct {
	Kind  string
	Tool  ToolKind
	Title string
	Lines []string
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
			"This panel is ready, but the current backend does not expose",
			"the checked " + string(kind) + " service.",
			"",
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
		s.panel.Lines = []string{
			"Backend tool request failed:",
			"",
			result.err.Error(),
		}
		s.setStatus(toolTitle(result.kind) + ": " + result.err.Error())
		return
	}
	if result.snapshot.Title != "" {
		s.panel.Title = result.snapshot.Title
	}
	s.panel.Lines = append([]string(nil), result.snapshot.Lines...)
	s.setStatus(toolTitle(result.kind) + " refreshed")
}

func (s *Shell) openControllerPanel() {
	s.panel = &Panel{
		Kind:  "controller",
		Title: "Controller Settings",
	}
}

func (s *Shell) openAudioPanel() {
	s.panel = &Panel{
		Kind:  "audio",
		Title: "Audio Settings",
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
	case "audio":
		s.handleAudioPanelShortcuts()
	case "controller":
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			if s.settings.KeyboardProfile == "default" {
				s.settings.KeyboardProfile = "wasd"
			} else {
				s.settings.KeyboardProfile = "default"
			}
			_ = s.settings.save()
			s.setStatus("Keyboard profile: " + s.settings.KeyboardProfile)
		}
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

func (s *Shell) handleAudioPanelShortcuts() {
	changed := false
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyM):
		s.settings.Muted = !s.settings.Muted
		changed = true
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		s.settings.Volume = min(100, s.settings.Volume+5)
		changed = true
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		s.settings.Volume = max(0, s.settings.Volume-5)
		changed = true
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		s.settings.AudioLatencyMS = min(250, s.settings.AudioLatencyMS+10)
		changed = true
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		s.settings.AudioLatencyMS = max(20, s.settings.AudioLatencyMS-10)
		changed = true
	}
	if !changed {
		return
	}
	_ = s.settings.save()
	if backend, ok := s.backend.(AudioBackend); ok {
		if err := backend.ConfigureAudio(AudioSettings{
			Muted:   s.settings.Muted,
			Volume:  s.settings.Volume,
			Latency: time.Duration(s.settings.AudioLatencyMS) * time.Millisecond,
		}); err != nil {
			s.setStatus("Audio settings: " + err.Error())
			return
		}
	}
	s.setStatus(fmt.Sprintf(
		"Audio: muted=%t volume=%d latency=%dms",
		s.settings.Muted,
		s.settings.Volume,
		s.settings.AudioLatencyMS,
	))
}

func (s *Shell) drawPanel(screen *ebiten.Image) {
	x, y, width, height := 110, 70, 740, 580
	ebitenutil.DrawRect(screen, 0, 0, logicalWidth, logicalHeight, colorOverlay)
	ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), float64(width+4), float64(height+4), borderColor)
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(width), float64(height), panelColor)
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(width), 38, accentColor)
	ebitenutil.DebugPrintAt(screen, s.panel.Title, x+16, y+15)

	lines := s.panelLines()
	lines = wrapPanelLines(lines, 84, 29)
	ebitenutil.DebugPrintAt(screen, strings.Join(lines, "\n"), x+22, y+58)

	footer := s.panelFooter()
	if footer != "" {
		ebitenutil.DebugPrintAt(screen, footer, x+22, y+height-30)
	}
	ebitenutil.DrawRect(screen, 760, 612, 90, 36, menuActiveColor)
	ebitenutil.DebugPrintAt(screen, "Close", 785, 626)
}

func (s *Shell) panelLines() []string {
	if s.panel == nil {
		return nil
	}
	switch s.panel.Kind {
	case "logs":
		start := max(0, len(s.logs)-28)
		if start == len(s.logs) {
			return []string{"No frontend log entries."}
		}
		return append([]string(nil), s.logs[start:]...)
	case "controller":
		lines := []string{
			"Keyboard profile: " + s.settings.KeyboardProfile,
			fmt.Sprintf("Connected gamepads: %d", len(ebiten.AppendGamepadIDs(nil))),
			"",
			"Normalized control bindings:",
		}
		for _, binding := range keyboardBindings(s.settings.KeyboardProfile) {
			lines = append(lines, fmt.Sprintf("  %-12s  %s", binding.Control, binding.Label))
		}
		lines = append(lines,
			"",
			"Standard gamepad layout: D-pad, A/B, shoulders, and Menu.",
			"Keyboard and gamepad state are merged before events reach the backend.",
		)
		return lines
	case "audio":
		backendStatus := "The current backend does not expose live audio configuration."
		if _, ok := s.backend.(AudioBackend); ok {
			backendStatus = "Changes are applied to the connected backend immediately."
		}
		return []string{
			fmt.Sprintf("Muted: %t", s.settings.Muted),
			fmt.Sprintf("Volume: %d%%", s.settings.Volume),
			fmt.Sprintf("Requested latency: %d ms", s.settings.AudioLatencyMS),
			"",
			backendStatus,
		}
	case "properties":
		if s.input == nil {
			return []string{"No input is selected."}
		}
		return []string{
			"Name: " + s.input.DisplayName,
			"Format: " + emptyFallback(s.input.Format, "unknown"),
			fmt.Sprintf("Size: %d bytes", s.input.Size),
			"SHA-256: " + emptyFallback(s.input.SHA256, "not supplied"),
			"Profile: " + emptyFallback(s.input.ProfileID, "unselected"),
			"Path/handle: " + emptyFallback(s.selectedPath, "native document"),
			"Backend: " + s.backendName(),
			"Frontend state: " + string(s.state),
			"Core state: " + string(s.backend.State()),
		}
	case "compatibility":
		if s.input == nil {
			return []string{"No input is selected."}
		}
		lines := []string{
			"Input: " + s.input.DisplayName,
			"SHA-256: " + emptyFallback(s.input.SHA256, "not supplied"),
			"Format: " + emptyFallback(s.input.Format, "unknown"),
			"Profile: " + emptyFallback(s.input.ProfileID, "unselected"),
			"Backend: " + s.backendName(),
			"Frontend state: " + string(s.state),
			"Core state: " + string(s.backend.State()),
			"",
			"The report contains metadata only; no game or firmware bytes are copied.",
		}
		if s.problem != nil {
			lines = append(lines,
				"",
				"Last problem: "+string(s.problem.State),
				s.problem.Reason,
			)
		}
		return lines
	default:
		return append([]string(nil), s.panel.Lines...)
	}
}

func (s *Shell) panelFooter() string {
	if s.panel == nil {
		return ""
	}
	switch s.panel.Kind {
	case "audio":
		return "M: mute  Up/Down: volume  Left/Right: latency  Esc: close"
	case "controller":
		return "P: switch keyboard profile  Esc: close"
	case "compatibility":
		return "S: save report  Esc: close"
	case "tool":
		return "R: refresh from backend  Esc: close"
	case "logs":
		return "Ctrl+S: save log  Esc: close"
	default:
		return "Esc: close"
	}
}

func wrapPanelLines(lines []string, width, limit int) []string {
	var wrapped []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for len([]rune(line)) > width {
			lineRunes := []rune(line)
			breakAt := width
			for index := width; index > 0; index-- {
				if lineRunes[index] == ' ' || lineRunes[index] == '\t' {
					breakAt = index
					break
				}
			}
			wrapped = append(wrapped, string(lineRunes[:breakAt]))
			line = strings.TrimSpace(string(lineRunes[breakAt:]))
		}
		wrapped = append(wrapped, line)
		if len(wrapped) >= limit {
			return wrapped[:limit]
		}
	}
	if len(wrapped) > limit {
		return wrapped[:limit]
	}
	return wrapped
}

func toolTitle(kind ToolKind) string {
	switch kind {
	case ToolCheats:
		return "Cheat Manager"
	case ToolMemory:
		return "Memory Search"
	case ToolPatches:
		return "Patch Manager"
	case ToolDebugger:
		return "Debugger"
	case ToolLogs:
		return "Logs"
	case ToolCompatibility:
		return "Compatibility Report"
	default:
		return strings.Title(string(kind))
	}
}
