package frontend

import (
	"context"
	"testing"
	"time"
)

func TestGenericEmulatorCommandsRemainPresent(t *testing.T) {
	menus := defaultMenus()
	wantMenus := []string{"File", "Emulation", "View", "Tools", "Help"}
	if len(menus) != len(wantMenus) {
		t.Fatalf("menu count = %d, want %d", len(menus), len(wantMenus))
	}
	for index, want := range wantMenus {
		if menus[index].Label != want {
			t.Fatalf("menu[%d] = %q, want %q", index, menus[index].Label, want)
		}
	}
	required := map[string]bool{
		"file.open":               false,
		"file.open_firmware":      false,
		"file.recent":             false,
		"emu.save_state":          false,
		"emu.state_slot":          false,
		"emu.speed":               false,
		"emu.rewind":              false,
		"emu.configure":           false,
		"tools.cheats":            false,
		"tools.memory":            false,
		"tools.patches":           false,
		"tools.debugger":          false,
		"tools.controller":        false,
		"tools.audio":             false,
		"tools.logs":              false,
		"tools.export_debug":      false,
		"tools.open_debug_folder": false,
		"tools.compatibility":     false,
		"help.issue":              false,
		"help.issue_history":      false,
		"view.fullscreen":         false,
		"view.rotation":           false,
		"view.layout":             false,
		"view.filter":             false,
		"view.screenshot":         false,
	}
	for _, menu := range menus {
		for _, command := range menu.Commands {
			if _, ok := required[command.ID]; ok {
				required[command.ID] = true
			}
		}
	}
	for command, found := range required {
		if !found {
			t.Errorf("required frontend command %q is missing", command)
		}
	}
}

func TestEveryPersistentCommandHasAnImplementationBoundary(t *testing.T) {
	shell := &Shell{
		backend:  NullBackend{},
		settings: defaultSettings(),
	}
	for _, menu := range defaultMenus() {
		for _, command := range menu.Commands {
			if command.ID == "" {
				t.Errorf("%s command has no stable ID", menu.Label)
			}
			if command.Action == nil && command.Backend == "" {
				t.Errorf("%s has neither frontend action nor backend operation", command.ID)
			}
			if command.DisplayLabel(shell) == "" {
				t.Errorf("%s has no display label", command.ID)
			}
		}
	}
}

func TestUnavailableBackendCommandExplainsWhy(t *testing.T) {
	shell := &Shell{
		backend:  NullBackend{},
		input:    &InputInfo{DisplayName: "synthetic.dat"},
		settings: defaultSettings(),
	}
	shell.menus = defaultMenus()
	command, found := shell.findCommand("emu.start")
	if !found {
		t.Fatal("emu.start not found")
	}
	availability := command.Availability(shell)
	if availability.Supported {
		t.Fatal("null backend unexpectedly supports start")
	}
	if availability.Reason == "" {
		t.Fatal("unavailable command has no explanation")
	}
}

func TestToolCommandOpensCapabilityPanel(t *testing.T) {
	shell := &Shell{
		backend:  NullBackend{},
		settings: defaultSettings(),
	}
	shell.menus = defaultMenus()
	shell.dispatchCommand("tools.debugger")
	if shell.panel == nil || shell.panel.Tool != ToolDebugger {
		t.Fatalf("debugger panel = %#v", shell.panel)
	}
	if len(shell.panel.Lines) == 0 {
		t.Fatal("debugger panel does not explain the unavailable backend capability")
	}
}

type interactiveToolBackend struct {
	NullBackend
	requests chan ToolRequest
}

func (*interactiveToolBackend) ToolSnapshot(
	context.Context,
	ToolKind,
) (ToolSnapshot, error) {
	return ToolSnapshot{
		Title:  "Memory Search",
		Lines:  []string{"Enter a value to begin a checked backend search."},
		Fields: []ToolField{{ID: "value", Label: "Value", Placeholder: "0x1234"}},
		Actions: []ToolAction{{
			ID:      "search",
			Label:   "Search",
			Enabled: true,
		}},
	}, nil
}

func (backend *interactiveToolBackend) ExecuteToolAction(
	_ context.Context,
	request ToolRequest,
) (ToolSnapshot, error) {
	backend.requests <- request
	return ToolSnapshot{
		Title: "Memory Search",
		Lines: []string{"1 checked result"},
		Actions: []ToolAction{{
			ID:      "clear",
			Label:   "Clear",
			Enabled: true,
		}},
	}, nil
}

func TestInteractiveToolSendsFieldsThroughBackendBoundary(t *testing.T) {
	backend := &interactiveToolBackend{requests: make(chan ToolRequest, 1)}
	shell := NewShell(backend, nil, "")
	shell.openToolPanel(ToolMemory)
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))

	if len(shell.panel.Fields) != 1 || len(shell.panel.Actions) != 1 {
		t.Fatalf("interactive tool panel = %#v", shell.panel)
	}
	shell.executeToolAction("search", map[string]string{"value": "0x1234"})
	request := <-backend.requests
	if request.Kind != ToolMemory ||
		request.Action != "search" ||
		request.Fields["value"] != "0x1234" {
		t.Fatalf("tool request = %#v", request)
	}
	shell.consumeToolResult(waitToolResult(t, shell.toolResults))
	if shell.panel.Busy || len(shell.panel.Lines) != 1 || shell.panel.Lines[0] != "1 checked result" {
		t.Fatalf("completed tool panel = %#v", shell.panel)
	}
}

func waitToolResult(t *testing.T, results <-chan toolResult) toolResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for tool result")
		return toolResult{}
	}
}
