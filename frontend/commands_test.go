package frontend

import "testing"

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
		"file.open":           false,
		"file.open_firmware":  false,
		"file.recent":         false,
		"emu.save_state":      false,
		"emu.state_slot":      false,
		"emu.speed":           false,
		"emu.rewind":          false,
		"tools.cheats":        false,
		"tools.memory":        false,
		"tools.patches":       false,
		"tools.debugger":      false,
		"tools.controller":    false,
		"tools.audio":         false,
		"tools.logs":          false,
		"tools.compatibility": false,
		"view.fullscreen":     false,
		"view.rotation":       false,
		"view.layout":         false,
		"view.filter":         false,
		"view.screenshot":     false,
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
