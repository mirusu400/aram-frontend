package frontend

// experimentSettingsRowModels builds the Experiments section: opt-in
// behaviours that trade one thing for another and are worth exposing before
// they are proven enough for a main section.
func experimentSettingsRowModels(shell *Shell) []settingsRowModel {
	return []settingsRowModel{
		{
			label:       "Display sync",
			description: "Show exactly one guest frame per display refresh instead of pacing by the clock, which removes the periodic double frame of a 62.5 Hz title on a 60 Hz display. The title runs up to five percent slower or faster to fit and its audio follows. Needs vsync on and a display near the title's frame rate; otherwise clock pacing is used.",
			value:       onOff(shell.settings.DisplaySync),
			action:      shell.toggleDisplaySync,
		},
		{
			label:       "Vsync",
			description: "Wait for the display before showing each frame. Off draws as fast as the host can for lower input latency, at the cost of tearing and much higher power use, and turns off display sync.",
			value:       onOff(!shell.settings.VsyncDisabled),
			action:      shell.toggleVsync,
		},
		{
			label:       "UI priority",
			description: "Give the interface CPU priority over the guest, so menus and input stay smooth while a heavy title runs a little slower.",
			value:       onOff(shell.settings.UIPriority),
			action:      shell.toggleUIPriority,
		},
		{
			label:       "CPU profiling",
			description: "Continuously sample the frontend's CPU use so a debug bundle can include a profile. Adds a small runtime cost.",
			value:       onOff(shell.settings.CPUProfile),
			action:      shell.toggleCPUProfile,
		},
		{
			label:       "Widescreen",
			description: "Widen the guest screen. A camera-scrolled title (some RPGs) shows more of the world; other titles leave margins or misplace fixed art. Tuned for 320-tall titles. Takes effect the next time a title is opened.",
			dropdown: &settingsDropdownModel{
				count: len(widescreenChoices()),
				label: func(i int) string {
					choices := widescreenChoices()
					if i < 0 || i >= len(choices) {
						return ""
					}
					return shell.widescreenLabel(choices[i])
				},
				value: func() int {
					return shell.widescreenChoiceIndex(widescreenChoices())
				},
				apply: func(i int) {
					choices := widescreenChoices()
					if i >= 0 && i < len(choices) {
						shell.setGuestWidthOverride(choices[i])
					}
				},
			},
		},
	}
}
