package frontend

type Command struct {
	ID           string
	Label        string
	Shortcut     string
	Backend      BackendCommand
	Enabled      func(*Shell) bool
	Action       func(*Shell)
	DynamicLabel func(*Shell) string
}

func (c Command) DisplayLabel(shell *Shell) string {
	if c.DynamicLabel != nil {
		return c.DynamicLabel(shell)
	}
	return shell.tr(c.Label)
}

func (c Command) Availability(shell *Shell) Capability {
	if c.Enabled != nil && !c.Enabled(shell) {
		return Capability{Reason: shell.tr("This command is not available in the current frontend state")}
	}
	if c.Backend != "" {
		if shell.input == nil {
			return Capability{Reason: shell.tr("Open a title or firmware input first")}
		}
		if capabilities, ok := shell.backend.(CapabilityBackend); ok {
			capability := capabilities.Capability(c.Backend)
			capability.Reason = shell.tr(capability.Reason)
			return capability
		}
		if !shell.backend.Supports(c.Backend) {
			return Capability{Reason: shell.tr("The selected backend does not support this command")}
		}
		return Capability{Supported: true}
	}
	if c.Action == nil {
		return Capability{Reason: shell.tr("This frontend command is not implemented")}
	}
	return Capability{Supported: true}
}

func (c Command) IsEnabled(shell *Shell) bool {
	return c.Availability(shell).Supported
}

type Menu struct {
	Label    string
	Commands []Command
}

func defaultMenus() []Menu {
	hasInput := func(shell *Shell) bool { return shell.input != nil }
	hasRecent := func(shell *Shell) bool { return len(shell.settings.RecentFiles) > 0 }
	hasFrame := func(shell *Shell) bool { return shell.currentFrame().Image != nil }
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
				{ID: "emu.frame", Label: "Frame Advance", Shortcut: "F7", Backend: CommandFrame},
				{ID: "emu.stop", Label: "Stop", Shortcut: "F8", Backend: CommandStop},
				{ID: "emu.reset", Label: "Reset", Shortcut: "Ctrl+R", Backend: CommandReset},
				{ID: "emu.fast_forward", Label: "Fast Forward", Backend: CommandFastForward},
				{ID: "emu.load_state", Label: "Load State", Shortcut: "F9", Backend: CommandLoadState},
				{ID: "emu.save_state", Label: "Save State", Shortcut: "F10", Backend: CommandSaveState},
				{
					ID:     "emu.state_slot",
					Action: (*Shell).cycleStateSlot,
					DynamicLabel: func(shell *Shell) string {
						return shell.trf("State Slot: %d", shell.settings.StateSlot)
					},
				},
				{
					ID:     "emu.speed",
					Action: (*Shell).cycleSpeed,
					DynamicLabel: func(shell *Shell) string {
						return shell.trf("Speed: %gx", shell.settings.Speed)
					},
				},
				{ID: "emu.rewind", Label: "Rewind", Backend: CommandRewind},
				{ID: "emu.configure", Label: "Configure...", Action: (*Shell).openSettingsPanel},
			},
		},
		{
			Label: "View",
			Commands: []Command{
				{ID: "view.fullscreen", Label: "Toggle Fullscreen", Shortcut: "F11", Action: (*Shell).toggleFullscreen},
				{
					ID:      "view.focus",
					Label:   "Focus Mode",
					Enabled: func(*Shell) bool { return platformUsesTouchLayout() },
					Action:  (*Shell).toggleFocusMode,
				},
				{ID: "view.integer", Label: "Integer Scaling", Action: (*Shell).toggleIntegerScaling},
				{ID: "view.aspect", Label: "Preserve Aspect Ratio", Action: (*Shell).toggleAspectRatio},
				{ID: "view.fit", Label: "Fit Window", Shortcut: "Ctrl+0", Action: (*Shell).fitWindow},
				{
					ID:     "view.rotation",
					Action: (*Shell).cycleRotation,
					DynamicLabel: func(shell *Shell) string {
						return shell.trf("Rotation: %d°", shell.settings.Rotation)
					},
				},
				{
					ID:     "view.layout",
					Action: (*Shell).cycleScreenLayout,
					DynamicLabel: func(shell *Shell) string {
						return shell.trf(
							"Screen Layout: %s",
							shell.tr(settingValueLabel(shell.settings.ScreenLayout)),
						)
					},
				},
				{
					ID:     "view.filter",
					Action: (*Shell).cycleFilter,
					DynamicLabel: func(shell *Shell) string {
						return shell.trf(
							"Filter: %s",
							shell.tr(settingValueLabel(shell.settings.Filter)),
						)
					},
				},
				{ID: "view.screenshot", Label: "Screenshot", Shortcut: "Ctrl+Shift+S", Enabled: hasFrame, Action: (*Shell).saveScreenshot},
			},
		},
		{
			Label: "Tools",
			Commands: []Command{
				{ID: "tools.cheats", Label: "Cheat Manager", Action: func(shell *Shell) { shell.openToolPanel(ToolCheats) }},
				{ID: "tools.memory", Label: "Memory Search", Action: func(shell *Shell) { shell.openToolPanel(ToolMemory) }},
				{ID: "tools.patches", Label: "Patch Manager", Action: func(shell *Shell) { shell.openToolPanel(ToolPatches) }},
				{ID: "tools.debugger", Label: "Debugger", Action: func(shell *Shell) { shell.openToolPanel(ToolDebugger) }},
				{ID: "tools.controller", Label: "Controller Settings", Action: (*Shell).openControllerPanel},
				{ID: "tools.audio", Label: "Audio Settings", Action: (*Shell).openAudioPanel},
				{ID: "tools.properties", Label: "Title Properties", Enabled: hasInput, Action: (*Shell).openPropertiesPanel},
				{ID: "tools.compatibility", Label: "Compatibility Report", Enabled: hasInput, Action: (*Shell).openCompatibilityPanel},
				{ID: "tools.logs", Label: "Logs", Action: func(shell *Shell) { shell.openToolPanel(ToolLogs) }},
				{ID: "tools.export_debug", Label: "Export Debug Bundle...", Shortcut: "Ctrl+Shift+D", Action: (*Shell).saveDebugBundle},
				{ID: "tools.open_debug_folder", Label: "Open Debug Bundle Folder", Action: (*Shell).openDebugBundleFolder},
			},
		},
		{
			Label: "Help",
			Commands: []Command{
				{ID: "help.updates", Label: "Check for Updates...", Action: (*Shell).openUpdatesPanel},
				{ID: "help.documentation", Label: "Documentation", Action: (*Shell).openDocumentation},
				{ID: "help.issue", Label: "Report Issue", Action: (*Shell).openIssueTracker},
				{ID: "help.issue_history", Label: "Submitted Reports...", Action: (*Shell).openIssueReportHistory},
				{ID: "help.about", Label: "About ARAM", Action: (*Shell).showAbout},
			},
		},
	}
}
