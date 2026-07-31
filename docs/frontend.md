# Frontend command contract

## File

- Open File (`Ctrl+O`)
- Open Firmware Directory
- Open Recent
- Close Title
- Exit

Desktop file dialogs, mobile document pickers, file association, drag-and-drop,
and command-line paths converge on one backend open request.
The desktop picker accepts direct WIPI `.zip`/`.ZIP` packages in addition to
DAT, JAR, and firmware inputs. ZIP archives remain intact and are inspected by
the backend loader rather than unpacked by the presentation layer. Native
mobile hosts should include `application/zip` in their document picker.

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
- Configure frontend settings

## View

- Fullscreen (`F11`, desktop)
- Integer Scaling
- Preserve Aspect Ratio
- Fit Window (`Ctrl+0`)
- Rotation
- Screen Layout
- Nearest/linear filter
- Native-resolution Screenshot (`Ctrl+Shift+S`)

The internal canvas follows the actual window size. Application chrome keeps
fixed readable typography while the guest viewport consumes the remaining
space. Compact windows hide secondary toolbar/status metadata, center and
clamp modal windows, and scroll long settings pages instead of scaling the
entire 960x720 interface.

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
`Open Recent` uses an in-application list with a visible vertical scrollbar.
Entries lead with the filename instead of a shared directory prefix, and the
selected entry shows its wrapped full path before it is opened.
Checked backends can expose `ToolField` and `ToolAction` descriptors in a
snapshot. The frontend renders those fields and actions and sends a
`ToolRequest` through `ToolActionBackend`; guest memory is never accessed
directly by the frontend.

## Windows input

The default keyboard profile maps arrows, Enter, Backspace, Q/E soft keys,
Space, digits, comma/star, and period/hash into backend-neutral controls. A
WASD direction preset is also supported. Every keyboard action can be
remapped by clicking its binding row and pressing the desired physical key;
Escape cancels capture. Assigning an already-used key swaps the two actions
instead of creating an ambiguous duplicate. Standard-layout gamepads use the
same press-to-capture interaction and can be enabled independently, swap the
south/east confirm and back buttons, and map the left analog stick to
directions with a configurable dead zone.
Keyboard and gamepad state are merged so releasing one host input does not
release a control that is still held by another.
These options are persisted from `Tools > Controller Settings` or the
`Controls` category in `Configure ARAM`.

The `Bindings` category switches between keyboard and gamepad devices and
remaps every available normalized action from actual input. Profiles can
remain global or be stored per title using the input SHA-256 identity.
Connection support and live mapped input are visible in the Controls page.
Additional SDL-compatible mappings can be placed at
`ARAM/gamecontrollerdb.txt` beside `settings.json` and reloaded without
restarting. The optional virtual keypad reserves a responsive rail to the
right of the guest display and exposes direction, soft, action, menu, and
phone-number controls to mouse or touch input.

## Updates

`Help > Check for Updates...` opens the `Updates` category in `Configure ARAM`.
It exposes downloads for:

- the integrated `aram-emu` product;
- the optional `aram-core` headless developer tools;
- the standalone `aram-frontend`.

The first integrated launch opens a Welcome modal with Stable, Nightly, and
Decide later actions. The selection is persisted in `settings.json`; Decide
later closes the modal for the current session and presents it again next
launch. The standalone Null-backend frontend preview skips this Welcome.
Channel selection does not download anything automatically because the
matching `aram-core` runtime is already compiled into each `aram-emu` build.

The channel selector switches between the latest non-prerelease GitHub Release
(`Stable`) and the rolling release tagged `nightly` (`Nightly`). Each repository
publishes the exact Windows amd64, Linux amd64, and macOS arm64 archive names
expected by the frontend. The download path is
`Downloads/ARAM/<repository>/<channel>/<version>`.

Downloads use a temporary file, enforce a size limit, validate a GitHub
SHA-256 digest when one is present, and only then rename the archive into its
final path. Existing downloads are preserved with a numbered filename. The
running executable and loaded title are never replaced or restarted.

## Audio

Mute, volume, requested latency, and output device are configured from the
shared Audio settings category and applied immediately through `AudioBackend`.
Backends that implement `AudioDeviceBackend` provide selectable device IDs;
an empty ID always means the host system default.

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
