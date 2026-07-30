# ARAM Frontend

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

```powershell
go run ./cmd/aram-frontend
go test ./...
```

The standalone command uses a null backend, so menus and file selection work
while execution reports that an integration backend is not attached.

The runnable product command lives in the sibling `aram-emu` integration
repository. It attaches the portable `aram-core` application machine:

```powershell
cd ..\aram-emu
go run ./cmd/aram
```

On Windows the shared shell supports native file and firmware selection,
command-line inputs, drag-and-drop, the full emulator command model, guest
frame rendering, keyboard/gamepad input, native-resolution screenshots, and
frontend tool panels. Backend-dependent operations stay visible and explain
why they are unavailable.

Desktop resizing uses the available window as the internal canvas: the guest
viewport expands while menus, text, centered dialogs, and scrollable settings
remain readable. Controller bindings use press-to-capture keyboard/gamepad
remapping with global and per-title profiles. An optional responsive virtual
phone keypad can be shown beside the guest display.

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

The CI workflow builds the Android AAR as a first-class portability check.
The native Android host project and release signing remain integration
responsibilities rather than frontend-library concerns.
