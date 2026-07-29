package frontend

type Command struct {
	ID       string
	Label    string
	Shortcut string
	Backend  BackendCommand
	Enabled  func(*Shell) bool
	Action   func(*Shell)
}

func (c Command) IsEnabled(shell *Shell) bool {
	if c.Enabled != nil && !c.Enabled(shell) {
		return false
	}
	if c.Backend != "" {
		return shell.backend.Supports(c.Backend)
	}
	return c.Action != nil
}

type Menu struct {
	Label    string
	Commands []Command
}

func defaultMenus() []Menu {
	hasInput := func(shell *Shell) bool { return shell.input != nil }
	hasRecent := func(shell *Shell) bool { return len(shell.settings.RecentFiles) > 0 }
	return []Menu{
		{
			Label: "File",
			Commands: []Command{
				{ID: "file.open", Label: "Open File...", Shortcut: "Ctrl+O", Action: (*Shell).chooseFile},
				{ID: "file.open_firmware", Label: "Open Firmware Directory...", Action: (*Shell).chooseFirmwareDirectory},
				{ID: "file.recent", Label: "Open Recent...", Enabled: hasRecent, Action: (*Shell).chooseRecent},
				{ID: "file.close", Label: "Close Title", Enabled: hasInput, Action: (*Shell).closeInput},
				{ID: "file.exit", Label: "Exit", Action: func(shell *Shell) { shell.quitting = true }},
			},
		},
		{
			Label: "Emulation",
			Commands: []Command{
				{ID: "emu.start", Label: "Start", Shortcut: "F5", Backend: CommandStart},
				{ID: "emu.pause", Label: "Pause / Resume", Shortcut: "F6", Backend: CommandPauseResume},
				{ID: "emu.stop", Label: "Stop", Shortcut: "F8", Backend: CommandStop},
				{ID: "emu.reset", Label: "Reset", Shortcut: "Ctrl+R", Backend: CommandReset},
				{ID: "emu.frame", Label: "Frame Advance", Backend: CommandFrame},
				{ID: "emu.fast_forward", Label: "Fast Forward", Backend: CommandFastForward},
				{ID: "emu.load_state", Label: "Load State...", Shortcut: "F9", Backend: CommandLoadState},
				{ID: "emu.save_state", Label: "Save State...", Shortcut: "F10", Backend: CommandSaveState},
				{ID: "emu.rewind", Label: "Rewind", Backend: CommandRewind},
			},
		},
		{
			Label: "View",
			Commands: []Command{
				{ID: "view.fullscreen", Label: "Toggle Fullscreen", Shortcut: "F11", Action: (*Shell).toggleFullscreen},
				{ID: "view.integer", Label: "Integer Scaling", Action: (*Shell).toggleIntegerScaling},
				{ID: "view.aspect", Label: "Preserve Aspect Ratio", Action: (*Shell).toggleAspectRatio},
				{ID: "view.fit", Label: "Fit Window"},
				{ID: "view.rotation", Label: "Rotation"},
				{ID: "view.layout", Label: "Screen Layout"},
				{ID: "view.filter", Label: "Filter"},
				{ID: "view.screenshot", Label: "Screenshot"},
			},
		},
		{
			Label: "Tools",
			Commands: []Command{
				{ID: "tools.cheats", Label: "Cheat Manager"},
				{ID: "tools.memory", Label: "Memory Search"},
				{ID: "tools.patches", Label: "Patch Manager"},
				{ID: "tools.debugger", Label: "Debugger"},
				{ID: "tools.controller", Label: "Controller Settings"},
				{ID: "tools.audio", Label: "Audio Settings"},
				{ID: "tools.properties", Label: "Title Properties"},
				{ID: "tools.compatibility", Label: "Compatibility Report"},
				{ID: "tools.logs", Label: "Logs"},
			},
		},
		{
			Label: "Help",
			Commands: []Command{
				{ID: "help.documentation", Label: "Documentation"},
				{ID: "help.issue", Label: "Report Issue"},
				{ID: "help.about", Label: "About ARAM", Action: (*Shell).showAbout},
			},
		},
	}
}
