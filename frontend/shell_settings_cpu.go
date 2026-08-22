package frontend

import "strings"

// cpuDropdownChoices returns the selectable CPU backend identifiers in dropdown
// order. It asks the backend which cores it offers (only the precise
// interpreter until a fast/native core registers), always including the current
// selection so a saved choice stays visible.
func (s *Shell) cpuDropdownChoices() []string {
	// "fastest" is not a core the backend registers; it is a request for the
	// best one this build has, so it is offered first and always available.
	choices := []string{"fastest", "precise"}
	if selector, ok := s.backend.(CPUBackendSelector); ok {
		if available := selector.AvailableCPUBackends(); len(available) > 0 {
			choices = append([]string{"fastest"}, available...)
		}
	}
	current := s.settings.CPUChoice
	if current == "" {
		current = "precise"
	}
	found := false
	for _, c := range choices {
		if c == current {
			found = true
			break
		}
	}
	if !found {
		choices = append(append([]string(nil), choices...), current)
	}
	return choices
}

// cpuChoiceLabel returns the localized display label for a CPU backend name.
func (s *Shell) cpuChoiceLabel(name string) string {
	switch name {
	case "fastest":
		return s.tr("Fastest available")
	case "", "precise", "portable-interpreter":
		return s.tr("Precise (interpreter)")
	case "jit":
		return s.tr("JIT recompiler (experimental)")
	default:
		if title := strings.TrimSpace(name); title != "" {
			return s.trf("Core: %s", title)
		}
		return s.tr("Precise (interpreter)")
	}
}

func (s *Shell) currentCPUSettings() CPUSettings {
	name := s.settings.CPUChoice
	if name == "" {
		name = "fastest"
	}
	return CPUSettings{Name: name}
}

// setCPU selects a CPU backend by identifier and applies it.
func (s *Shell) setCPU(name string) {
	if name == "" {
		name = "fastest"
	}
	s.settings.CPUChoice = name
	s.applyCPUSettings()
}

// applyCPUSettings persists the selection and pushes it to the backend. The new
// core takes effect the next time a title is opened.
func (s *Shell) applyCPUSettings() {
	_ = s.settings.save()
	if selector, ok := s.backend.(CPUBackendSelector); ok {
		if err := selector.ConfigureCPU(s.currentCPUSettings()); err != nil {
			s.setStatus(s.tr("CPU core: ") + err.Error())
			return
		}
	}
	s.setStatus(s.trf(
		"CPU core: %s (restart title to apply)",
		s.cpuChoiceLabel(s.settings.CPUChoice),
	))
}
