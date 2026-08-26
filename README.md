# ARAM Frontend

[![ci](https://github.com/mirusu400/aram-frontend/actions/workflows/ci.yml/badge.svg)](https://github.com/mirusu400/aram-frontend/actions/workflows/ci.yml)

Cross-platform product frontend for **ARAM, Archived Runtime for ARM
Mobiles**.

This repository owns:

- the shared emulator screen, menus, status, settings, input, and overlays;
- desktop application behavior for Windows, Linux, and macOS;
- Android/iOS Ebitengine mobile bindings and native document-picker bridges;
- generic emulator workflows such as open/recent, pause, state, rewind,
  controller configuration, cheats, debugger, and compatibility reporting.

The Tools menu can export one attachable debug ZIP containing redacted
frontend event logs, the current guest-native screenshot when available, and
bounded diagnostics supplied by the active backend.

The presentation layer uses Ebitengine with EbitenUI and an ARAM-specific
design system. Semantic palette, typography, spacing, radius, and component
tokens live in `frontend/design_system.go`; shell composition lives in
`frontend/shell_ui.go`. See [`docs/design-system.md`](docs/design-system.md)
before adding or restyling UI.

Parsing WIPI containers and emulating ARM live in the core; an injected
`Backend` implementation connects this frontend to
[`aram-core`](https://github.com/mirusu400/aram-core). The integration and
release plan live in [`aram-emu`](https://github.com/mirusu400/aram-emu).

## Desktop

Run the integrated emulator from the sibling `aram-emu` repository:

```powershell
cd ..\aram-emu
go run ./cmd/aram
```

The command in this repository is only a frontend preview. It intentionally
uses a null backend, so it exercises menus, layout, and file selection while
the core handles actual execution:

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

`Tools > Export Debug Bundle...` (`Ctrl+Shift+D`) writes an `aram-debug-*.zip`
below the platform configuration directory. The bundle contains a JSON
manifest, redacted frontend logs, build/runtime metadata, input identity
metadata, the current guest-native screenshot when available, and checked
backend diagnostic files. It excludes the selected game and firmware bytes,
guest memory, save data, and other proprietary media.
`Tools > Open Debug Bundle Folder` creates that directory when necessary and
opens it in Explorer, Finder, or the desktop file manager.
`Help > Report Issue` collects the situation, game title, carrier, and expected
ARAM repository in-app. Submitting creates the same redacted debug ZIP,
uploads it and the current screenshot when available through the ARAM Report
Relay, creates the GitHub issue automatically, and opens the finished issue in
the browser. Screenshot transfer is enabled by default and can be turned off
in the report form; disabling it also removes the image from the uploaded ZIP.
The completed panel can add capability-authorized follow-up comments without a
GitHub login. The latest 20 successful submissions are saved locally and can
be reopened from `Help > Submitted Reports...` to visit the issue or add
another relay-authorized comment after closing the original panel or
restarting ARAM. If the relay is unavailable, ARAM preserves the bundle, opens
its folder, and falls back to a prefilled GitHub draft for manual submission.
Uploaded attachments are publicly linked from the issue and expire after 30
days; the selected game and firmware bytes stay excluded.

Desktop resizing uses the available window as the internal canvas: the guest
viewport expands while menus, text, centered dialogs, and scrollable settings
remain readable. Controller bindings use press-to-capture keyboard/gamepad
remapping with global and per-title profiles. An optional responsive virtual
phone keypad can be shown beside the guest display.

## Updates

`Help > Check for Updates...` (or `Emulation > Configure ARAM > Updates`)
downloads public archives for the integrated `aram-emu` product, the
optional `aram-core` developer tools, and the standalone `aram-frontend`.
The Updates category and `Help > About ARAM` show the version of the running
build: a release tag for Stable, the source revision for Nightly, or a
development revision for local builds.
In the integrated host, the ARAM product action installs the verified archive,
restarts automatically, and reopens the currently loaded input. Core tools and
the standalone frontend remain ordinary archive downloads.

On the first integrated `aram-emu` launch, a responsive Welcome dialog asks
for the Stable or Nightly channel. The integrated desktop host then downloads
the latest `aram-emu` archive for that channel, verifies its GitHub SHA-256
digest, installs it into a content-addressed runtime directory, and relaunches
ARAM. That archive already contains a mutually compatible `aram-core` and
`aram-frontend`; downloading their standalone developer artifacts would not
change the running Go executable. If no Stable release exists yet, ARAM keeps
using the bundled build. The standalone Null-backend frontend preview leaves
product installation to the integrated build.

- **Stable** selects the latest published GitHub Release.
- **Nightly** selects the rolling `nightly` prerelease produced from the latest
  successful `main` build.

Downloads are stored below `Downloads/ARAM`. First-run product installation
extracts the verified archive below the platform `ARAM/runtime/versions`
configuration directory instead of overwriting a running executable. The
original bootstrap delegates future launches to the selected runtime. Windows
amd64, Linux amd64, and macOS arm64 use the matching CI archive; unsupported
host combinations are disabled in the UI.

On Android, the ARAM product row downloads `aram-android-universal.apk` into
an app-private update folder named by the native host and hands it to the
system package installer, where the user confirms the update. The Welcome
modal only records the chosen channel there because the installed APK is
already the complete product. Developer archives have no Android build.

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
Windows x64, Linux x64, and macOS arm64. Branch and pull-request builds skip
GitHub Actions artifact uploads. Main and Stable release runs use the three
desktop archives only as a temporary cross-job handoff, delete those workflow
artifacts immediately after publishing, and keep the resulting files only on
the GitHub Release. The Android AAR and iOS XCFramework are build gates and are
not uploaded as workflow artifacts. The standalone desktop builds use the null
backend; the runnable integrated emulator is published by `aram-emu`.

After every successful push to `main`, CI moves the `nightly` tag to that
commit and updates the rolling Nightly prerelease with the exact three desktop
archives produced by the run plus `SHA256SUMS.txt`. The Android AAR and iOS
XCFramework remain first-class portability gates, so a failed mobile binding
prevents a broken frontend Nightly from being published.

Publishing any GitHub Release other than `nightly` builds that tag and attaches
the same desktop archives to the release for the Stable channel.

The native Android host project and release signing remain integration
responsibilities rather than frontend-library concerns.
