# ARAM Frontend

[![ci](https://github.com/mirusu400/aram-frontend/actions/workflows/ci.yml/badge.svg)](https://github.com/mirusu400/aram-frontend/actions/workflows/ci.yml)

Cross-platform product frontend for **ARAM — Archived Runtime for ARM
Mobiles**.

This repository owns:

- the shared emulator screen, menus, status, settings, input, and overlays;
- desktop application behavior for Windows, Linux, and macOS;
- Android/iOS Ebitengine mobile bindings and native document-picker bridges;
- generic emulator workflows such as open/recent, pause, state, rewind,
  controller configuration, cheats, debugger, and compatibility reporting.

The presentation layer uses Ebitengine with EbitenUI and an ARAM-specific
design system. Semantic palette, typography, spacing, radius, and component
tokens live in `frontend/design_system.go`; shell composition lives in
`frontend/shell_ui.go`. See [`docs/design-system.md`](docs/design-system.md)
before adding or restyling UI.

It does not parse WIPI containers or emulate ARM. An injected `Backend`
implementation connects it to
[`aram-core`](https://github.com/mirusu400/aram-core). The integration and
release plan live in [`aram-emu`](https://github.com/mirusu400/aram-emu).

## Desktop

Run the integrated emulator from the sibling `aram-emu` repository:

```powershell
cd ..\aram-emu
go run ./cmd/aram
```

The command in this repository is only a frontend preview. It intentionally
uses a null backend, so it can exercise menus, layout, and file selection but
cannot execute a game:

```powershell
go run ./cmd/aram-frontend
go test ./...
```

On Windows the shared shell supports native file and firmware selection,
command-line inputs, drag-and-drop, the full emulator command model, guest
frame rendering, keyboard/gamepad input, native-resolution screenshots, and
frontend tool panels. Backend-dependent operations stay visible and explain
why they are unavailable. File/Open includes direct WIPI ZIP packages alongside
DAT and JAR packages; the selected archive is passed intact to `aram-core` for
format inspection and safe loading.

Desktop resizing uses the available window as the internal canvas: the guest
viewport expands while menus, text, centered dialogs, and scrollable settings
remain readable. Controller bindings use press-to-capture keyboard/gamepad
remapping with global and per-title profiles. An optional responsive virtual
phone keypad can be shown beside the guest display.

## Updates

`Help > Check for Updates...` (or `Emulation > Configure ARAM > Updates`)
downloads public archives for the integrated `aram-emu` product, the
optional `aram-core` developer tools, and the standalone `aram-frontend`.

On the first integrated `aram-emu` launch, a responsive Welcome dialog asks
for the Stable or Nightly channel and persists that choice. `aram-core` is
already compiled into the ARAM product, so no separate core download is
required for gameplay. Choosing a channel does not silently download or
replace executables; updates remain explicit from the Updates page.

- **Stable** selects the latest published GitHub Release.
- **Nightly** selects the rolling `nightly` prerelease produced from the latest
  successful `main` build.

Downloads are stored below `Downloads/ARAM` and never replace the running
application. GitHub-provided SHA-256 digests are checked before an archive is
made visible. Windows amd64, Linux amd64, and macOS arm64 use the matching CI
archive; unsupported host combinations are disabled in the UI.

## Mobile

Ebitengine mobile applications are generated as native libraries:

```text
ebitenmobile bind -target android -javapkg io.github.mirusu400.aram \
  -o build/aram.aar ./mobile
```

Android Studio supplies the Activity, Storage Access Framework document
picker, lifecycle, and packaging. iOS uses the equivalent generated
XCFramework and native document picker.

The generated mobile API accepts a native `Host` callback for document-picker
requests and exposes completion, cancellation, lifecycle, and audio-focus
entry points.

Every push and pull request tests, vets, and builds the standalone frontend on
Windows x64, Linux x64, and macOS arm64. Compressed desktop artifacts, the
Android AAR, and the iOS XCFramework are retained for 14 days. The standalone
desktop artifacts use the null backend; the runnable integrated emulator is
published by `aram-emu`.

After every successful push to `main`, CI moves the `nightly` tag to that
commit and updates the rolling Nightly prerelease with the exact three desktop
archives produced by the run plus `SHA256SUMS.txt`. The Android AAR and iOS
XCFramework remain first-class portability gates, so a failed mobile binding
prevents a broken frontend Nightly from being published.

Publishing any GitHub Release other than `nightly` builds that tag and attaches
the same desktop archives to the release for the Stable channel.

The native Android host project and release signing remain integration
responsibilities rather than frontend-library concerns.
