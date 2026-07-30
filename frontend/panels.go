package frontend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Panel struct {
	Kind        string
	Tool        ToolKind
	Title       string
	Lines       []string
	Fields      []ToolField
	Actions     []ToolAction
	FieldValues map[string]string
	Busy        bool
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
		s.panel.Busy = false
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
	s.panel.Fields = append([]ToolField(nil), result.snapshot.Fields...)
	s.panel.Actions = append([]ToolAction(nil), result.snapshot.Actions...)
	s.panel.FieldValues = make(map[string]string, len(result.snapshot.Fields))
	for _, field := range result.snapshot.Fields {
		s.panel.FieldValues[field.ID] = field.Value
	}
	s.panel.Busy = false
	s.setStatus(toolTitle(result.kind) + " refreshed")
}

func (s *Shell) executeToolAction(action string, fields map[string]string) {
	if s.panel == nil || s.panel.Kind != "tool" || s.panel.Busy {
		return
	}
	backend, ok := s.backend.(ToolActionBackend)
	if !ok {
		s.setStatus(toolTitle(s.panel.Tool) + ": backend actions are unavailable")
		return
	}
	request := ToolRequest{
		Kind:   s.panel.Tool,
		Action: action,
		Fields: cloneStringMap(fields),
	}
	s.panel.Busy = true
	s.setStatus(toolTitle(s.panel.Tool) + ": " + action + "...")
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
			s.setStatus("Audio settings: " + err.Error())
			return
		}
	}
	if s.audioOutput != nil {
		s.audioOutput.configure(settings)
	}
	s.setStatus(fmt.Sprintf(
		"Audio: muted=%t volume=%d latency=%dms device=%s",
		s.settings.Muted,
		s.settings.Volume,
		s.settings.AudioLatencyMS,
		s.audioDeviceLabel(),
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
	s.saveControllerSettings("Virtual keypad: " + onOff(s.settings.ShowVirtualKeypad))
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
		s.setStatus("Controller database: " + err.Error())
		return
	}
	s.gamepadMappingsLoaded = applied
	if !applied {
		path, pathErr := customGamepadMappingsPath()
		if pathErr != nil {
			s.setStatus("Controller database: " + pathErr.Error())
			return
		}
		s.setStatus("Controller database: no mapping file at " + path)
		return
	}
	s.setStatus("Custom controller database reloaded")
}

func (s *Shell) togglePerTitleControls() {
	if s.settings.PerTitleControls {
		s.settings.PerTitleControls = false
		s.saveControllerSettings("Controller profile scope: global")
		return
	}
	key := s.titleControllerKey()
	if key == "" {
		s.setStatus("Controller profile: open an identified title first")
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
		s.setStatus("Controller settings: " + err.Error())
		return
	}
	s.setStatus(status)
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
	s.setStatus("Appearance: " + strings.Title(s.settings.ThemeMode))
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
