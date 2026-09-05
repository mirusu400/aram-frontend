package frontend

import "strings"

func (s *Shell) panelLines() []string {
	if s.panel == nil {
		return nil
	}
	switch s.panel.Kind {
	case "logs":
		start := max(0, len(s.logs)-28)
		if start == len(s.logs) {
			return []string{s.tr("No frontend log entries.")}
		}
		return append([]string(nil), s.logs[start:]...)
	case "properties":
		if s.input == nil {
			return []string{s.tr("No input is selected.")}
		}
		pathLabel := s.selectedPath
		if pathLabel != "" && pathLabel == s.temporaryPath {
			// A temporary copy's path is a private cache filename
			// ("drop-<random>.ext"), meaningless to the user — show the
			// name it was opened under instead.
			pathLabel = s.trf("%s (temporary copy)", s.input.DisplayName)
		}
		return []string{
			s.trf("Name: %s", s.input.DisplayName),
			s.trf(
				"Format: %s",
				emptyFallback(s.input.Format, s.tr("unknown")),
			),
			s.trf("Size: %d bytes", s.input.Size),
			s.trf(
				"SHA-256: %s",
				emptyFallback(s.input.SHA256, s.tr("not supplied")),
			),
			s.trf(
				"Profile: %s",
				emptyFallback(s.input.ProfileID, s.tr("unselected")),
			),
			s.trf(
				"Path/handle: %s",
				emptyFallback(pathLabel, s.tr("native document")),
			),
			s.trf("Backend: %s", s.backendName()),
			s.trf(
				"Frontend state: %s",
				s.tr(stateValueLabel(string(s.state))),
			),
			s.trf(
				"Core state: %s",
				s.tr(stateValueLabel(string(s.backend.State()))),
			),
		}
	case "compatibility":
		if s.input == nil {
			return []string{s.tr("No input is selected.")}
		}
		lines := []string{
			s.trf("Input: %s", s.input.DisplayName),
			s.trf(
				"SHA-256: %s",
				emptyFallback(s.input.SHA256, s.tr("not supplied")),
			),
			s.trf(
				"Format: %s",
				emptyFallback(s.input.Format, s.tr("unknown")),
			),
			s.trf(
				"Profile: %s",
				emptyFallback(s.input.ProfileID, s.tr("unselected")),
			),
			s.trf("Backend: %s", s.backendName()),
			s.trf(
				"Frontend state: %s",
				s.tr(stateValueLabel(string(s.state))),
			),
			s.trf(
				"Core state: %s",
				s.tr(stateValueLabel(string(s.backend.State()))),
			),
			"",
			s.tr("The report contains metadata only; no game or firmware bytes are copied."),
		}
		if s.problem != nil {
			lines = append(lines,
				"",
				s.trf(
					"Last problem: %s",
					s.tr(stateValueLabel(string(s.problem.State))),
				),
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
	case "welcome":
		return "Choose Stable or Nightly; aram-core runtime is already included."
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
