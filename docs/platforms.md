# Platform architecture

## Shared layer

Ebitengine supplies the framebuffer surface and shared keyboard, gamepad,
touch, and audio abstractions. `frontend.Backend` supplies emulator state.
`frontend.Picker` supplies host-specific file/document selection.

## Desktop

| Platform | Window/render | File selection | Packaging |
|---|---|---|---|
| Windows | Ebitengine | native Windows dialog | `.exe`/installer |
| Linux | Ebitengine | Zenity-compatible dialog | AppImage/Flatpak later |
| macOS | Ebitengine | native macOS dialog | signed `.app` later |

Desktop retains a conventional top menu and keyboard shortcuts.

## Android

The Go package is bound through `ebitenmobile bind` into an AAR. An Android
Activity owns:

- Storage Access Framework open-document and open-document-tree contracts;
- persisted URI permissions;
- pause/resume and audio-focus lifecycle;
- gamepad and touch-overlay settings;
- scoped storage and share intents.

The native layer passes a document handle or copied cache file to the shared
frontend. Desktop Zenity code is excluded by build tags.

CI binds `./mobile` into `build/aram.aar`; this prevents Android support from
becoming an uncompiled roadmap claim.

## iOS

The same shared game is bound into an XCFramework. UIKit owns the document
picker, application lifecycle, security-scoped resources, signing, and store
packaging.

## Backend portability

Frontend availability does not imply that every CPU backend is available.
The UI reports the selected backend and provides a portable-interpreter
fallback when the integration layer offers one.
