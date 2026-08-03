package frontend

import (
	"context"
	"strings"
	"time"

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

func (s *Shell) applyAudioSettings() {
	_ = s.settings.save()
	settings := s.currentAudioSettings()
	if backend, ok := s.backend.(AudioBackend); ok {
		if err := backend.ConfigureAudio(settings); err != nil {
			s.setStatus(s.tr("Audio settings: ") + err.Error())
			return
		}
	}
	if s.audioOutput != nil {
		s.audioOutput.configure(settings)
	}
	s.setStatus(s.trf(
		"Audio: muted=%s volume=%d latency=%dms device=%s",
		s.tr(onOff(s.settings.Muted)),
		s.settings.Volume,
		s.settings.AudioLatencyMS,
		s.tr(s.audioDeviceLabel()),
	))
}

func (s *Shell) currentAudioSettings() AudioSettings {
	return AudioSettings{
		Muted:    s.settings.Muted,
		Volume:   s.settings.Volume,
		Latency:  time.Duration(s.settings.AudioLatencyMS) * time.Millisecond,
		DeviceID: s.settings.AudioDeviceID,
	}
}

func (s *Shell) toggleMuted() {
	s.settings.Muted = !s.settings.Muted
	s.applyAudioSettings()
}

func (s *Shell) cycleVolume() {
	s.settings.Volume += 5
	if s.settings.Volume > 100 {
		s.settings.Volume = 0
	}
	s.applyAudioSettings()
}

func (s *Shell) cycleAudioLatency() {
	s.settings.AudioLatencyMS += 10
	if s.settings.AudioLatencyMS > 250 {
		s.settings.AudioLatencyMS = 20
	}
	s.applyAudioSettings()
}

func (s *Shell) cycleAudioDevice() {
	devices := s.audioDevices()
	current := -1
	for index, device := range devices {
		if device.ID == s.settings.AudioDeviceID {
			current = index
			break
		}
	}
	s.settings.AudioDeviceID = devices[(current+1)%len(devices)].ID
	s.applyAudioSettings()
}

func (s *Shell) audioDevices() []AudioDevice {
	devices := []AudioDevice{{Name: "System default"}}
	backend, ok := s.backend.(AudioDeviceBackend)
	if !ok {
		return devices
	}
	seen := map[string]bool{"": true}
	for _, device := range backend.AudioDevices() {
		if device.ID == "" || seen[device.ID] {
			continue
		}
		if device.Name == "" {
			device.Name = device.ID
		}
		seen[device.ID] = true
		devices = append(devices, device)
	}
	return devices
}

func (s *Shell) audioDeviceLabel() string {
	for _, device := range s.audioDevices() {
		if device.ID == s.settings.AudioDeviceID {
			return device.Name
		}
	}
	return "System default"
}

func (s *Shell) cycleKeyboardProfile() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		if profile.KeyboardProfile == "default" {
			profile.KeyboardProfile = "wasd"
		} else {
			profile.KeyboardProfile = "default"
		}
		profile.KeyboardBindings = nil
	}, "Keyboard profile updated")
}

func (s *Shell) toggleVirtualKeypad() {
	s.settings.ShowVirtualKeypad = !s.settings.ShowVirtualKeypad
	s.saveControllerSettings(s.trf(
		"Virtual keypad: %s",
		s.tr(onOff(s.settings.ShowVirtualKeypad)),
	))
}

func (s *Shell) toggleGamepadEnabled() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadEnabled = !profile.GamepadEnabled
	}, "Gamepad input updated")
}

func (s *Shell) cycleGamepadLayout() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		if profile.GamepadLayout == "standard" {
			profile.GamepadLayout = "swapped"
		} else {
			profile.GamepadLayout = "standard"
		}
		delete(profile.GamepadBindings, "ok")
		delete(profile.GamepadBindings, "back")
	}, "Gamepad confirm/back layout updated")
}

func (s *Shell) toggleGamepadAnalog() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadAnalog = !profile.GamepadAnalog
	}, "Analog directions updated")
}

func (s *Shell) cycleGamepadDeadzone() {
	s.updateControllerProfile(func(profile *ControllerProfile) {
		profile.GamepadDeadzone += 5
		if profile.GamepadDeadzone > 50 {
			profile.GamepadDeadzone = 15
		}
	}, "Gamepad dead zone updated")
}

func (s *Shell) resetControllerBindings() {
	s.bindingCapture = nil
	key := s.controllerProfileKey()
	if key != "" {
		delete(s.settings.TitleControllers, key)
	} else {
		s.settings.setGlobalControllerProfile(defaultSettings().globalControllerProfile())
	}
	s.saveControllerSettings("Controller bindings reset")
}

func (s *Shell) reloadGamepadMappings() {
	applied, err := loadCustomGamepadMappings()
	if err != nil {
		s.setStatus(s.tr("Controller database: ") + err.Error())
		return
	}
	s.gamepadMappingsLoaded = applied
	if !applied {
		path, pathErr := customGamepadMappingsPath()
		if pathErr != nil {
			s.setStatus(s.tr("Controller database: ") + pathErr.Error())
			return
		}
		s.setStatus(s.trf(
			"Controller database: no mapping file at %s",
			path,
		))
		return
	}
	s.setStatus(s.tr("Custom controller database reloaded"))
}

func (s *Shell) togglePerTitleControls() {
	if s.settings.PerTitleControls {
		s.settings.PerTitleControls = false
		s.saveControllerSettings("Controller profile scope: global")
		return
	}
	key := s.titleControllerKey()
	if key == "" {
		s.setStatus(s.tr("Controller profile: open an identified title first"))
		return
	}
	s.settings.PerTitleControls = true
	if _, ok := s.settings.TitleControllers[key]; !ok {
		s.settings.TitleControllers[key] = s.settings.globalControllerProfile()
	}
	s.saveControllerSettings("Controller profile scope: this title")
}

func (s *Shell) controllerProfile() ControllerProfile {
	global := s.settings.globalControllerProfile()
	key := s.controllerProfileKey()
	if key == "" {
		return global
	}
	profile, ok := s.settings.TitleControllers[key]
	if !ok {
		return global
	}
	profile.normalize()
	return profile
}

func (s *Shell) updateControllerProfile(update func(*ControllerProfile), status string) {
	profile := s.controllerProfile()
	update(&profile)
	profile.normalize()
	if key := s.controllerProfileKey(); key != "" {
		if s.settings.TitleControllers == nil {
			s.settings.TitleControllers = make(map[string]ControllerProfile)
		}
		s.settings.TitleControllers[key] = profile
	} else {
		s.settings.setGlobalControllerProfile(profile)
	}
	s.saveControllerSettings(status)
}

func (s *Shell) titleControllerKey() string {
	if s.input == nil {
		return ""
	}
	if s.input.SHA256 != "" {
		return "sha256:" + strings.ToLower(s.input.SHA256)
	}
	if s.input.DisplayName != "" {
		return "title:" + strings.ToLower(s.input.DisplayName)
	}
	return ""
}

func (s *Shell) controllerProfileKey() string {
	if !s.settings.PerTitleControls {
		return ""
	}
	return s.titleControllerKey()
}

func (s *Shell) controllerProfileScopeLabel() string {
	if s.controllerProfileKey() != "" {
		return "This title"
	}
	return "Global"
}

func (s *Shell) gamepadBindingLabel(control string) string {
	for _, binding := range gamepadBindingsForProfile(s.controllerProfile()) {
		if binding.Control == control {
			return binding.Label
		}
	}
	return "Unassigned"
}

func (s *Shell) saveControllerSettings(status string) {
	if err := s.settings.save(); err != nil {
		s.setStatus(s.tr("Controller settings: ") + err.Error())
		return
	}
	s.setStatus(s.tr(status))
}

func gamepadLayoutLabel(layout string) string {
	switch layout {
	case "swapped":
		return "East confirm"
	case "custom":
		return "Custom"
	default:
		return "South confirm"
	}
}

func keyboardProfileLabel(profile string) string {
	switch profile {
	case "wasd":
		return "WASD"
	case "custom":
		return "Custom"
	default:
		return "Arrow keys"
	}
}

func controlDisplayName(control string) string {
	switch control {
	case "up":
		return "Up"
	case "down":
		return "Down"
	case "left":
		return "Left"
	case "right":
		return "Right"
	case "ok":
		return "Confirm"
	case "back":
		return "Back"
	case "soft-left":
		return "Soft key left"
	case "soft-right":
		return "Soft key right"
	case "menu":
		return "Menu"
	case "star":
		return "Star"
	case "hash":
		return "Hash"
	default:
		if strings.HasPrefix(control, "num") && len(control) == 4 {
			return "Number " + strings.TrimPrefix(control, "num")
		}
		return control
	}
}

func (s *Shell) cycleThemeMode() {
	if s.settings.ThemeMode == "light" {
		s.settings.ThemeMode = "dark"
	} else {
		s.settings.ThemeMode = "light"
	}
	_ = s.settings.save()
	s.setStatus(s.trf(
		"Appearance: %s",
		s.tr(settingValueLabel(s.settings.ThemeMode)),
	))
}

func (s *Shell) cycleLanguage() {
	previous := s.settings.Language
	if normalizeLanguage(previous) == LanguageKorean {
		s.settings.Language = string(LanguageEnglish)
	} else {
		s.settings.Language = string(LanguageKorean)
	}
	if err := s.settings.save(); err != nil {
		s.settings.Language = previous
		s.setStatus(s.tr("Language settings: ") + err.Error())
		return
	}
	s.setStatus(s.trf(
		"Language: %s",
		languageLabel(s.language(), s.language()),
	))
	if s.panel != nil {
		s.panel.Title = s.tr("Configure ARAM")
	}
	if localized, ok := s.picker.(languageAwarePicker); ok {
		localized.SetLanguage(s.language())
	}
	if s.input == nil {
		setPlatformWindowTitle(s.tr("ARAM - Archived Runtime for ARM Mobiles"))
	}
	s.interfaceUI = newShellUI(s, s.design)
}

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
				emptyFallback(s.selectedPath, s.tr("native document")),
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
