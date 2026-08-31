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
- Speed (0.5x, 1x, 1.5x, 2x, 2.5x, 3x, 4x)
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
- Display filter: Original, Crisp Fit, Feature Phone TFT, Feature Phone STN,
  Smooth Pixel, and CRT TV
- Adjustable display-filter strength
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
- Export Debug Bundle (`Ctrl+Shift+D`)
- Open Debug Bundle Folder

The menu model is shared. Mobile may present the same commands as a drawer or
overlay rather than a desktop top bar, but the feature IDs remain identical.
`Open Recent` uses an in-application list with a visible vertical scrollbar.
Entries lead with the filename instead of a shared directory prefix, and the
selected entry shows its wrapped full path before it is opened.
`Open Debug Bundle Folder` creates the shared debug artifact directory when it
does not exist and opens it with Explorer, Finder, or `xdg-open`.
Checked backends can expose `ToolField` and `ToolAction` descriptors in a
snapshot. The frontend renders those fields and actions and sends a
`ToolRequest` through `ToolActionBackend`; guest memory is never accessed
directly by the frontend.

## Windows input

The default keyboard profile maps arrows, Enter, Backspace for cancel/back,
Home/End for call/end-call, Q/E soft keys, Page Up/Down handset volume keys,
Space, digits, comma/star, and period/hash into backend-neutral controls. The
optional desktop virtual keypad exposes those phone actions as CALL, END,
CANCEL, VOL-, and VOL+ buttons alongside the navigation and number keys. A
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
phone-number controls to mouse or touch input. Direction keys carry an arrow
rather than a localized word, on the keypad and on the touch deck alike.

The status bar ends with a machine-state signal glyph and, where the host
reports one, the battery charge. Neither is decorative: a platform that
exposes no battery draws no meter.

On touch layouts, the menu bar, toolbar, and status bar disappear when a
title enters the running state so the guest screen can fill everything above
the on-screen control deck at its largest aspect-preserving size; the
integer-scaling preference applies only to the windowed workspace. A faint
hamburger toggle floating at the top-right restores the chrome, and a HIDE
button at the right end of the toolbar strips it again. Open panels, the
touch layout editor, and focus mode always present their own full surface.

A panel that captures host input also pauses the running title: `Configure
ARAM`, `Help > Report an Issue...`, and every other modal panel hold the guest
still while they are open, and it resumes where it stopped once the panel is
closed. A panel that keeps host input on the guest - a cheat toggled during
play - leaves the title running.

## Updates

`Help > Check for Updates...` opens the `Updates` category in `Configure ARAM`.
Its read-only Current version row and `Help > About ARAM` identify the running
build. Stable archives show their release tag, Nightly archives show their
source revision, and local builds fall back to Go VCS build metadata.
It exposes downloads for:

- the integrated `aram-emu` product;
- the optional `aram-core` headless developer tools;
- the standalone `aram-frontend`.

When `ProductUpdateInstaller` is available, the ARAM product row is an
**Install & Restart** action rather than a download-only action. A successful
manual product update installs the selected channel immediately and passes the
current input path to the relaunched runtime. Developer tools and standalone
frontend archives remain download-only.

On Android, the product row is an **Install** action: the verified
`aram-android-universal.apk` is saved below the app-private update folder
named by the native host, and the host's installer returns
`ErrProductInstallDeferred` after handing the package to the system package
installer. The shell keeps running and reports the hand-off; Android replaces
the app only once the user confirms there. Developer tools and standalone
frontend archives publish no Android build, so those rows stay unavailable.

The first integrated launch opens a Welcome modal with Stable, Nightly, and
Decide later actions. The selection is persisted in `settings.json`; Decide
later closes the modal for the current session and presents it again next
launch. The standalone Null-backend frontend preview skips this Welcome.
When the backend implements `ProductUpdateInstaller`, selecting a channel
automatically downloads the matching integrated `aram-emu` archive. The host
installs and relaunches that verified build, which already contains compatible
`aram-core` and `aram-frontend` revisions. A missing Stable release falls back
to the bundled build; a Nightly or network failure keeps the Welcome open for
retry or Decide later.

The channel selector switches between the latest non-prerelease GitHub Release
(`Stable`) and the rolling release tagged `nightly` (`Nightly`). Each repository
publishes the exact Windows amd64, Linux amd64, and macOS arm64 archive names
expected by the frontend. The download path is
`Downloads/ARAM/<repository>/<channel>/<version>`.

Successful core and frontend Nightlies are also the inputs to the integrated
product pipeline. `aram-emu` records their exact revisions, rebuilds the whole
statically linked product, and publishes a new product Nightly. Downloading a
standalone core or frontend archive remains a developer action and never
changes the code inside a running integrated ARAM app.

Downloads use a temporary file, enforce a size limit, validate a GitHub
SHA-256 digest when one is present, and only then rename the archive into its
final path. Existing downloads are preserved with a numbered filename. The
desktop product host extracts first-run updates into a content-addressed
runtime directory and relaunches it; manual developer-tool and standalone
frontend downloads never replace the running executable.

An installed product archive is deleted once the host has extracted it, along
with the repository, channel, and version folders the download created, so a
product the app installs itself does not also accumulate a copy of every build
under `Downloads`. A folder still holding another download is left alone, and an
archive a failed install left behind stays so it can be installed by hand.
Developer-tool and standalone frontend downloads are never deleted.

## Audio

Mute, volume, requested latency, and output device are configured from the
shared Audio settings category and applied immediately through `AudioBackend`.
Backends that implement `AudioDeviceBackend` provide selectable device IDs;
an empty ID always means the host system default.

All desktop entry paths converge on `OpenRequest`. Ebitengine drop handles are
copied to an application-private cache before opening, retain the original
display name and extension, and are removed when the input closes.

## Guest presentation

`VideoBackend` publishes an immutable guest-native frame with a sequence that
only changes when the pixels change. The shell reads it only after frame work
and timeline commands complete, then keys the upload by sequence, generation,
and guest time. The extra timeline anchors recover from a reused sequence
instead of leaving an old picture frozen while guest logic continues. The
persistent texture is rebuilt only when the guest changes resolution. A
backend that hands over tightly packed RGBA is uploaded without an intermediate
copy.

Feature Phone TFT is the default display filter. Crisp Fit uses sharp bilinear
sampling: source-pixel centers remain flat while fractional-size boundaries
blend across one output pixel. The TFT preset adds RGB565 colour, subtle LCD
cell seams, uneven backlighting, and an exponential response history driven by
display time, so moving pixels leave a real previous-frame trail and then
settle even when the guest frame becomes static. Feature Phone STN uses a
longer 75 ms response half-life, reduced saturation and contrast, a stronger
green-yellow cast, and visible cell crosstalk to reproduce older passive-matrix
panels. Smooth Pixel runs an independently implemented, edge-aware xBRZ-style
2x pass before fitting the image; isolated pixels and thin glyph strokes are
protected so small Hangul remains legible. Original keeps the separate
nearest/linear texture setting. CRT TV keeps luma detail while low-pass
filtering NTSC I/Q colour horizontally, then adds guest-row scanlines and a
3x2 RGB shadow mask at final display resolution. All presets affect
presentation only; native-resolution screenshots remain untouched.

The TFT, STN, Smooth Pixel, and CRT TV presets expose a 0-100% strength. The
default 100% preserves their authored appearance; lower values blend their
panel, smoothing, persistence, or CRT characteristics toward the fitted source.
All guest-presentation settings are saved automatically per identified title
using its input SHA-256. Changing them with no title open edits the global
defaults inherited by titles without an override.

Guest audio is drained from `AudioStreamBackend` on every tick, including ticks
where a frame batch is still executing. A title heavy enough to keep a batch
pending across ticks is exactly the one whose host queue would otherwise
underrun.

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

Debug bundles are ZIP files below `ARAM/debug`. Each bundle contains a
versioned JSON manifest, a redacted frontend event log, host/build metadata,
input hash and profile, the current guest-native frame as `screenshot.png`
when available, and bounded files returned by `DebugExportBackend`. The
screenshot is copied before asynchronous bundle collection so it represents
the frame visible when export was requested. Backend collection failures
become manifest warnings instead of discarding the frontend evidence.
Attachment names and sizes are validated before writing. The bundle excludes
input bytes, guest memory, save data, and other proprietary media.

`Help > Report Issue` opens an in-application report form for the situation,
game title, carrier, and expected `aram-frontend`, `aram-emu`, or `aram-core`
repository. `Submit Report` explicitly warns that it creates a public GitHub
issue, captures the same redacted debug bundle and current screenshot when
available, and streams them to
`https://aram-report-relay.mirusu400.workers.dev`. The relay creates the
selected repository issue and returns a report-scoped capability; the frontend
stores that capability in the user-private settings file so the user can add
follow-up comments without exposing a GitHub credential. The most recent 20
reports are available from `Help > Submitted Reports...`; selecting one
restores its GitHub link and follow-up comment form even after the original
panel or application was closed. Each capability is limited to its report and
is not included in exported debug bundles.

The finished issue opens in the browser. Its Worker-served attachment links
expire after 30 days. If upload or relay authentication fails, the bundle
remains local and the frontend opens its folder plus a prefilled GitHub draft
for manual attachment and submission. Idempotency keys are reused when that
automatic upload is retried, preventing an uncertain network response from
creating duplicate issues.
