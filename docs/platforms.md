# Platform architecture

## Shared layer

Ebitengine supplies the framebuffer surface and shared keyboard, gamepad,
touch, and audio abstractions. `frontend.Backend` supplies emulator state.
`frontend.Picker` supplies host-specific file/document selection.

The interface language defaults to the device language on first run and is a
saved setting from then on. Windows reads `GetUserDefaultLocaleName` and macOS
reads `AppleLanguages`, both in `locale_*.go`. Android exposes neither to Go,
so its Activity passes the device tag through `Mobile.configureLocale` into
`frontend.SetHostLocale` before the shell loads settings; a locale declared
that way takes precedence over the environment.

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
frontend. Its document picker must include `application/zip` so direct WIPI
ZIP packages reach the same backend open request as desktop inputs. Desktop
Zenity code is excluded by build tags.

The generated binding exposes `mobile.SetHost`. The native Activity implements
`RequestDocument`, then completes the asynchronous request with
`OpenDocument`, `OpenFirmware`, or `DocumentSelectionCanceled`. `AudioFocus`
and `Pause`/`Resume` feed the same automatic lifecycle-pause contract.

Text entry crosses the same bridge. Ebitengine raises no soft keyboard for its
own surface and `exp/textinput` has no mobile backend, so a tap on a form
field - the issue report form above all - would otherwise be unanswerable on a
handset. The frontend calls `RequestTextInput(requestID, label, hint, text)`,
the Activity presents a native single-line editor where the system IME handles
Hangul composition and the clipboard, and it answers exactly once with
`SubmitTextInput` or `CancelTextInput`. Line breaks in the answer are folded to
spaces because the field renders one line, matching desktop. A build with no
host attached, which is every desktop build, keeps editing the field in place.

The shared mobile layout provides large touch targets for persistent
File/Emulation/View/Tools/Help navigation plus D-pad, OK, Back, Menu, and soft
keys. Turning on `Virtual keypad` adds the numeric cluster — 1-9, 0, star, and
hash — to the deck, which grows to hold four more rows; a handset has no
keyboard, so those keys are otherwise unreachable. Every deck button, keypad
keys included, is a placement slot: `Touch button layout` drags them into
custom positions stored normalized in `touch_layout`, so a saved arrangement
survives rotation and resizes.

That editor owns the whole trade between picture and controls. Steppers set
how much height the deck takes (`touch_deck_ratio`, 20-65% of the screen; the
guest display keeps the rest) and how large the buttons are
(`touch_control_scale`). Dragging a button into the tray puts it away
(`touch_hidden`) and dragging it back out restores it where it is dropped, so
a title that needs only a D-pad can clear everything else off the screen.
Every one of these is drafted while the editor is open and only written on
save, which is what makes `Cancel` mean cancel. Native hosts can dispatch the same stable command IDs and forward
inactive/active lifecycle changes. The lifecycle bridge resumes only a machine
that it automatically paused; it never overrides a manual user pause.

CI binds `./mobile` into `build/aram.aar`; this prevents Android support from
becoming an uncompiled roadmap claim.

## iOS

The same shared game is bound into an XCFramework. UIKit owns the document
picker, application lifecycle, security-scoped resources, signing, and store
packaging.

CI builds `build/ARAM.xcframework` on macOS so the exported host bridge is
checked for both mobile platforms.

## Backend portability

Frontend availability does not imply that every CPU backend is available.
The UI reports the selected backend and provides a portable-interpreter
fallback when the integration layer offers one.
