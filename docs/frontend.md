# Frontend command contract

## File

- Open File (`Ctrl+O`)
- Open Firmware Directory
- Open Recent
- Close Title
- Exit

Desktop file dialogs, mobile document pickers, file association, drag-and-drop,
and command-line paths converge on one backend open request.

## Emulation

- Start (`F5`)
- Pause/Resume (`F6`)
- Stop (`F8`)
- Reset (`Ctrl+R`)
- Frame Advance (`F7`)
- Fast Forward
- Load/Save State (`F9`/`F10`)
- State Slot (0-9)
- Speed (0.5x, 1x, 2x, 4x)
- Rewind

## View

- Fullscreen (`F11`, desktop)
- Integer Scaling
- Preserve Aspect Ratio
- Fit Window (`Ctrl+0`)
- Rotation
- Screen Layout
- Nearest/linear filter
- Native-resolution Screenshot (`Ctrl+Shift+S`)

## Tools

- Cheat Manager
- Memory Search
- Patch Manager
- Debugger
- Controller Settings
- Audio Settings
- Title Properties
- Compatibility Report
- Logs

The menu model is shared. Mobile may present the same commands as a drawer or
overlay rather than a desktop top bar, but the feature IDs remain identical.

## Windows input

The default keyboard profile maps arrows, Enter, Backspace, Q/E soft keys,
Space, digits, comma/star, and period/hash into backend-neutral controls. A
WASD direction profile and standard-layout gamepads are also supported.
Keyboard and gamepad state are merged so releasing one host input does not
release a control that is still held by another.

All desktop entry paths converge on `OpenRequest`. Ebitengine drop handles are
copied to an application-private cache before opening, retain the original
display name and extension, and are removed when the input closes.

## Observable state and errors

The shell exposes empty, selecting, inspecting, loading, ready, running,
paused, stopped, backend-unavailable, guest-faulted, malformed-input, and
unsupported-profile states. A structured failure keeps the input name,
detected format, selected profile, backend, and reason visible instead of
collapsing to an empty viewport.

## Generated artifacts

Screenshots are encoded from the guest-native frame before frontend scaling,
rotation, or filtering. Screenshots, compatibility reports, and exported logs
are stored below the platform user configuration directory in the `ARAM`
folder. Reports contain hashes and metadata only, never input bytes.
