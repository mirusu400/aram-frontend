package frontend

import (
	"fmt"
	"strings"

	"github.com/ebitenui/ebitenui/widget"
)

type settingsRowModel struct {
	label       string
	description string
	value       string
	action      func()
	disabled    bool
	slider      *settingsSliderModel
	dropdown    *settingsDropdownModel
}

// settingsSliderModel drives a slider-backed settings row. Slider positions
// run min..max in whole steps; value/format/apply translate between the
// position and the underlying setting.
type settingsSliderModel struct {
	min    int
	max    int
	value  func() int
	format func(int) string
	apply  func(int)
}

type settingsSliderBinding struct {
	slider *widget.Slider
	label  *widget.Text
	model  *settingsSliderModel
}

// settingsDropdownModel drives a dropdown-backed settings row with entries
// indexed 0..count-1.
type settingsDropdownModel struct {
	count int
	label func(int) string
	value func() int
	apply func(int)
}

type settingsDropdownBinding struct {
	dropdown *widget.ListComboButton
	model    *settingsDropdownModel
}

func (u *shellUI) settingsRows(shell *Shell) []*widget.Container {
	rows := u.settingsRowModels(shell)
	// One width for the whole section keeps the action column aligned down
	// the page instead of stepping in and out row by row.
	actionWidth := u.settingsActionWidth(shell, rows)
	result := make([]*widget.Container, 0, len(rows))
	for _, row := range rows {
		result = append(result, u.buildSettingsRow(row, actionWidth))
	}
	return result
}

// settingsRowModels describes the active section's rows. It is separate from
// the widget construction so the panel can measure a section before laying it
// out, and so tests can check that geometry without a running UI.
func (u *shellUI) settingsRowModels(shell *Shell) []settingsRowModel {
	var rows []settingsRowModel
	profile := shell.controllerProfile()
	display := shell.displayProfile()
	switch u.settingsSection {
	case "Appearance":
		rows = []settingsRowModel{
			{
				label:       "Application theme",
				description: "Switch between the neutral light and dark palettes.",
				value:       strings.Title(shell.settings.ThemeMode),
				action:      shell.cycleThemeMode,
			},
			{
				label:       "UI skin",
				description: "Sprite skin for the shell chrome. Retro skins follow the light/dark theme above.",
				dropdown: &settingsDropdownModel{
					count: len(themeFamilyChoices()),
					label: func(i int) string {
						choices := themeFamilyChoices()
						if i < 0 || i >= len(choices) {
							return ""
						}
						return shell.tr(themeFamilyLabel(choices[i]))
					},
					value: func() int {
						return themeFamilyIndex(shell.settings.ThemeFamily)
					},
					apply: func(i int) {
						choices := themeFamilyChoices()
						if i >= 0 && i < len(choices) {
							shell.setThemeFamily(choices[i])
						}
					},
				},
			},
			{
				label:       "Handset font",
				description: "Bitmap font used to render in-game text. Galmuri is crisp; Dunggeunmo is softer.",
				dropdown: &settingsDropdownModel{
					count: len(shell.fontDropdownChoices()),
					label: func(i int) string {
						choices := shell.fontDropdownChoices()
						if i < 0 || i >= len(choices) {
							return ""
						}
						return shell.fontChoiceLabel(choices[i])
					},
					value: func() int {
						return fontChoiceIndex(shell.fontDropdownChoices(), shell.settings.FontChoice)
					},
					apply: func(i int) {
						choices := shell.fontDropdownChoices()
						if i >= 0 && i < len(choices) {
							shell.setFont(choices[i])
						}
					},
				},
			},
			{
				label:       "Load custom font…",
				description: "Use your own BDF or TrueType/OpenType font for in-game text.",
				value:       "",
				action:      shell.loadCustomFont,
			},
			{
				label:       "Control shape",
				description: "Buttons, menus, cards, and dialogs use square corners.",
				value:       "Square",
			},
		}
	case "Graphics":
		rows = []settingsRowModel{
			{
				label:       "Display profile",
				description: "Changes are saved for the loaded title; without one they become global defaults.",
				value:       shell.displayProfileScopeLabel(),
			},
			{
				label:       "Integer scaling",
				description: "Use whole-number scale factors when possible.",
				value:       onOff(display.IntegerScaling),
				action:      func() { shell.dispatchCommand("view.integer") },
			},
			{
				label:       "Preserve aspect ratio",
				description: "Prevent the guest image from stretching.",
				value:       onOff(display.PreserveAspect),
				action:      func() { shell.dispatchCommand("view.aspect") },
			},
			{
				label:       "Rotation",
				description: "Rotate the guest display clockwise.",
				value:       fmt.Sprintf("%d°", display.Rotation),
				action:      func() { shell.dispatchCommand("view.rotation") },
			},
			{
				label:       "Texture filter",
				description: "Choose nearest or linear sampling.",
				value:       strings.Title(display.Filter),
				action:      func() { shell.dispatchCommand("view.filter") },
				disabled:    display.DisplayEffect != displayEffectOff,
			},
			{
				label:       "Display filter",
				description: "Choose original pixels, feature-phone panels, xBRZ-style smoothing, or an NTSC CRT TV.",
				dropdown: &settingsDropdownModel{
					count: len(displayEffectChoices()),
					label: func(i int) string {
						choices := displayEffectChoices()
						if i < 0 || i >= len(choices) {
							return ""
						}
						return shell.tr(displayEffectValueLabel(choices[i]))
					},
					value: func() int {
						return displayEffectIndex(shell.displayProfile().DisplayEffect)
					},
					apply: func(i int) {
						choices := displayEffectChoices()
						if i >= 0 && i < len(choices) {
							shell.setDisplayEffect(choices[i])
						}
					},
				},
			},
			{
				label:       "Filter strength",
				description: "Adjust the selected display filter from subtle to full.",
				disabled:    !displayEffectSupportsStrength(display.DisplayEffect),
				slider: &settingsSliderModel{
					min:    displayEffectStrengthMin / 10,
					max:    displayEffectStrengthMax / 10,
					value:  func() int { return shell.displayProfile().DisplayEffectStrength / 10 },
					format: func(v int) string { return fmt.Sprintf("%d%%", v*10) },
					apply:  func(v int) { shell.setDisplayEffectStrength(v * 10) },
				},
			},
			{
				label:       "Screen layout",
				description: "Center or stretch the guest display.",
				value:       strings.Title(display.ScreenLayout),
				action:      func() { shell.dispatchCommand("view.layout") },
			},
		}
	case "Audio":
		rows = []settingsRowModel{
			{
				label:       "Mute output",
				description: "Silence frontend audio without stopping emulation.",
				value:       onOff(shell.settings.Muted),
				action:      shell.toggleMuted,
			},
			{
				label:       "Effect / music mixing",
				description: "Mixed layers effects over continuous music; Faithful matches the handset.",
				value:       shell.audioMixModeLabel(),
				action:      shell.toggleAudioMixMode,
			},
			{
				label:       "Soften audio",
				description: "Gentle low-pass that eases the harsh FM synth top end. Playback only.",
				value:       onOff(shell.settings.AudioSoften),
				action:      shell.toggleAudioSoften,
			},
			{
				label:       "Volume",
				description: "Output volume in five-percent steps.",
				slider: &settingsSliderModel{
					min:    0,
					max:    40,
					value:  func() int { return (shell.settings.Volume + 2) / 5 },
					format: func(v int) string { return fmt.Sprintf("%d%%", v*5) },
					apply:  func(v int) { shell.setVolume(v * 5) },
				},
			},
			{
				label:       "Requested latency",
				description: "Audio buffer target in ten-millisecond steps.",
				slider: &settingsSliderModel{
					min:    2,
					max:    25,
					value:  func() int { return (shell.settings.AudioLatencyMS + 5) / 10 },
					format: func(v int) string { return fmt.Sprintf("%d ms", v*10) },
					apply:  func(v int) { shell.setAudioLatency(v * 10) },
				},
			},
			{
				label:       "Output device",
				description: "Choose a backend-provided device or the system default.",
				value:       shorten(shell.audioDeviceLabel(), 20),
				action:      shell.cycleAudioDevice,
			},
			{
				label:       "Buffer health",
				description: "Current host fill and cumulative underrun / overrun events.",
				value:       shell.audioQueueTelemetryLabel(),
			},
		}
	case "Controls":
		rows = []settingsRowModel{
			{
				label:       "Profile scope",
				description: "Keep global controls or save overrides for the loaded title.",
				value:       shell.controllerProfileScopeLabel(),
				action:      shell.togglePerTitleControls,
			},
			{
				label:       "Keyboard profile",
				description: "Apply the arrow-key or WASD preset, replacing custom keyboard bindings.",
				value:       keyboardProfileLabel(profile.KeyboardProfile),
				action:      shell.cycleKeyboardProfile,
			},
			{
				label:       "Virtual keypad",
				description: virtualKeypadDescription(),
				value:       onOff(shell.settings.ShowVirtualKeypad),
				action:      shell.toggleVirtualKeypad,
			},
			{
				label:       "Gamepad input",
				description: "Accept input from standard-layout controllers.",
				value:       onOff(profile.GamepadEnabled),
				action:      shell.toggleGamepadEnabled,
			},
			{
				label:       "Confirm / back layout",
				description: "Choose the south or east face button for confirm.",
				value:       gamepadLayoutLabel(profile.GamepadLayout),
				action:      shell.cycleGamepadLayout,
			},
			{
				label:       "Analog directions",
				description: "Map the left stick to normalized directions.",
				value:       onOff(profile.GamepadAnalog),
				action:      shell.toggleGamepadAnalog,
			},
			{
				label:       "Stick dead zone",
				description: "Ignore small left-stick movement.",
				value:       fmt.Sprintf("%d%%", profile.GamepadDeadzone),
				action:      shell.cycleGamepadDeadzone,
			},
			{
				label:       "Vibration",
				description: "Rumble a connected gamepad and vibrate the phone when a title requests it.",
				value:       onOff(shell.settings.VibrationEnabled),
				action:      shell.toggleVibration,
			},
			{
				label:       "Connected gamepads",
				description: "Detected devices and standard-layout support.",
				value:       gamepadConnectionLabel(shell.language()),
			},
			{
				label:       "Controller database",
				description: "Reload ARAM/gamecontrollerdb.txt for unsupported devices.",
				value: func() string {
					if shell.gamepadMappingsLoaded {
						return "Custom"
					}
					return "Built-in"
				}(),
				action: shell.reloadGamepadMappings,
			},
			{
				label:       "Live input test",
				description: "Press controller buttons to verify the active mapping.",
				value:       shorten(shell.gamepadActivityLabel(), 22),
			},
			{
				label:       "Button bindings",
				description: "Capture keyboard keys or physical gamepad buttons.",
				value:       "Edit",
				action: func() {
					u.selectSettingsSection(shell, "Bindings")
				},
			},
		}
		if platformUsesTouchLayout() {
			rows = append(rows,
				settingsRowModel{
					label:       "Touch button size",
					description: "Scale of the on-screen touch controls.",
					slider: &settingsSliderModel{
						min:    touchControlScaleMin / 10,
						max:    touchControlScaleMax / 10,
						value:  func() int { return shell.touchControlScalePercent() / 10 },
						format: func(v int) string { return fmt.Sprintf("%d%%", v*10) },
						apply:  func(v int) { shell.setTouchControlScale(v * 10) },
					},
				},
				settingsRowModel{
					label:       "Touch button layout",
					description: "Drag the on-screen buttons into custom positions.",
					value:       touchLayoutValueLabel(shell),
					action:      shell.beginTouchLayoutEdit,
				},
			)
		}
	case "Bindings":
		if u.bindingDevice != bindingDeviceKeyboard &&
			u.bindingDevice != bindingDeviceGamepad {
			u.bindingDevice = bindingDeviceKeyboard
		}
		rows = append(rows, settingsRowModel{
			label:       "Binding device",
			description: "Choose which physical input type to configure.",
			value:       strings.Title(string(u.bindingDevice)),
			action: func() {
				if shell.bindingCapture != nil {
					shell.cancelBindingCapture("Binding capture canceled")
				}
				if u.bindingDevice == bindingDeviceKeyboard {
					u.bindingDevice = bindingDeviceGamepad
				} else {
					u.bindingDevice = bindingDeviceKeyboard
				}
				u.panelSignature = ""
			},
		})
		controls := gamepadControlOrder
		if u.bindingDevice == bindingDeviceKeyboard {
			controls = keyboardControlOrder
		}
		for _, control := range controls {
			controlID := control
			if u.bindingDevice == bindingDeviceKeyboard {
				value := shorten(shell.keyboardBindingLabel(controlID), 20)
				description := "Click, then press the keyboard key to assign. Esc cancels."
				if captureMatches(shell.bindingCapture, bindingDeviceKeyboard, controlID) {
					value = "Press a key..."
					description = "Listening for keyboard input. Esc cancels."
				}
				rows = append(rows, settingsRowModel{
					label:       controlDisplayName(controlID),
					description: description,
					value:       value,
					action:      func() { shell.beginKeyboardBindingCapture(controlID) },
				})
				continue
			}
			value := shorten(shell.gamepadBindingLabel(controlID), 20)
			description := "Click, then press the physical gamepad button to assign. Esc cancels."
			if captureMatches(shell.bindingCapture, bindingDeviceGamepad, controlID) {
				value = "Press a button..."
				description = "Listening for a standard-layout gamepad button. Esc cancels."
			}
			rows = append(rows, settingsRowModel{
				label:       controlDisplayName(controlID),
				description: description,
				value:       value,
				action:      func() { shell.beginGamepadBindingCapture(controlID) },
			})
		}
		rows = append(rows, settingsRowModel{
			label:       "Reset all bindings",
			description: "Restore keyboard and gamepad mappings for the active profile.",
			value:       "Reset all",
			action:      shell.resetControllerBindings,
		})
	case "Experiments":
		rows = []settingsRowModel{
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
		}
	case "Updates":
		rows = updateSettingsRowModels(shell)
	default:
		rows = []settingsRowModel{
			{
				label:       "Language",
				description: "Choose the language used by menus, settings, and frontend messages.",
				value:       languageLabel(shell.language(), shell.language()),
				action:      shell.cycleLanguage,
			},
			{
				label:       "Emulation speed",
				description: "Guest execution speed relative to the original handset.",
				slider: &settingsSliderModel{
					min:   0,
					max:   len(speedPresets) - 1,
					value: func() int { return speedPresetIndex(shell.settings.Speed) },
					format: func(v int) string {
						return fmt.Sprintf("%gx", speedPresets[clampInt(v, 0, len(speedPresets)-1)])
					},
					apply: func(v int) {
						shell.setSpeed(speedPresets[clampInt(v, 0, len(speedPresets)-1)])
					},
				},
			},
			{
				label:       "Save-state slot",
				description: "Slot used by load and save state commands.",
				dropdown: &settingsDropdownModel{
					count: 10,
					label: func(v int) string { return shell.trf("Slot %d", v) },
					value: func() int { return shell.settings.StateSlot },
					apply: shell.setStateSlot,
				},
			},
			{
				label:       "CPU core",
				description: "Backend that runs the guest. Precise is the accurate interpreter; a faster core appears when available. Takes effect the next time a title is opened.",
				dropdown: &settingsDropdownModel{
					count: len(shell.cpuDropdownChoices()),
					label: func(i int) string {
						choices := shell.cpuDropdownChoices()
						if i < 0 || i >= len(choices) {
							return ""
						}
						return shell.cpuChoiceLabel(choices[i])
					},
					value: func() int {
						return fontChoiceIndex(shell.cpuDropdownChoices(), shell.settings.CPUChoice)
					},
					apply: func(i int) {
						choices := shell.cpuDropdownChoices()
						if i >= 0 && i < len(choices) {
							shell.setCPU(choices[i])
						}
					},
				},
			},
			{
				label:       "Backend",
				description: "Integration currently connected to the frontend.",
				value:       shorten(shell.backendName(), 24),
			},
		}
	}

	return rows
}

func updateSettingsRowModels(shell *Shell) []settingsRowModel {
	channel := normalizeUpdateChannel(shell.settings.UpdateChannel)
	downloadRoot, downloadRootErr := defaultUpdateDownloadRoot()
	downloadRootLabel := shorten(downloadRoot, 34)
	if downloadRootErr != nil {
		downloadRootLabel = "Unavailable"
	}
	return []settingsRowModel{
		{
			label:       "Current version",
			description: "Version of the running ARAM application.",
			value:       currentApplicationVersion(),
		},
		{
			label:       "Update channel",
			description: "Stable uses the latest official release; Nightly uses the latest main-branch build.",
			value:       updateChannelLabel(channel),
			action:      shell.cycleUpdateChannel,
		},
		{
			label: "ARAM product",
			description: shell.updateRowDescription(
				updateComponentProduct,
				"Integrated app, rebuilt from successful aram-core and aram-frontend Nightlies.",
			),
			value:    shell.updateActionLabel(updateComponentProduct),
			action:   func() { shell.downloadUpdate(updateComponentProduct) },
			disabled: !shell.updateActionAvailable(updateComponentProduct),
		},
		{
			label:       "Download folder",
			description: "Archives are verified and saved without replacing the running application.",
			value:       downloadRootLabel,
		},
	}
}

func touchLayoutValueLabel(shell *Shell) string {
	if len(shell.settings.TouchLayout) > 0 {
		return "Customized"
	}
	return "Default"
}

func onOff(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}

func visibility(visible bool) widget.Visibility {
	if visible {
		return widget.Visibility_Show
	}
	return widget.Visibility_Hide
}

// virtualKeypadDescription explains where the keypad goes, which differs by
// platform: a rail beside the guest display on desktop, extra rows of the
// on-screen deck on a touch layout.
func virtualKeypadDescription() string {
	if platformUsesTouchLayout() {
		return "Add the number, star, and hash keys to the on-screen controls."
	}
	return "Show a clickable phone keypad to the right of the guest display."
}
