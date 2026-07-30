# Codex project guide

## Scope

`aram-frontend` is the presentation and product-interaction repository. It
depends only on the small `Backend` contract defined here; it must not parse
WIPI containers, implement WIPI APIs, execute guest instructions, or inspect
CPU-specific state directly.

Sibling repositories:

- `aram-core`: headless execution, loaders, profiles, state, debugger backend;
- `aram-emu`: ecosystem roadmap, integration, packaging, release criteria;
- `aram-test`: black-box corpus orchestration, compatibility deltas, and
  failure triage;
- `anycall_magichole`: reverse-engineering evidence and reference runtime.

## Platform split

- Shared Ebitengine screen, menu model, overlays, hotkeys, touch layout, and
  frontend state stay in `frontend`.
- Windows/Linux/macOS use the desktop command and native dialog picker.
- Android/iOS use `ebitenmobile bind` and a native host application.
- Native mobile code owns Storage Access Framework/document pickers,
  permissions, lifecycle, share intents, and store packaging.
- The shared UI accepts a selected document from the native bridge; it never
  assumes every document is a normal filesystem path.
- Platform files use explicit Go build constraints. Do not add runtime
  `GOOS` conditionals where a build-tag boundary is clearer.

## Non-negotiable emulator UX

Keep persistent File, Emulation, View, Tools, and Help navigation. Preserve:

- file/firmware open, recent inputs, drag-and-drop, intents/file association;
- start, pause, reset, stop, frame advance, fast-forward, rewind;
- save/load states and state slots;
- fullscreen, integer scaling, aspect, rotation, layout, filters, screenshots;
- keyboard/gamepad/touch mapping and per-title profiles;
- audio settings, mute, latency, and device selection;
- cheats, memory search, patches, debugger, logs, and compatibility reports;
- explicit empty, loading, ready, running, paused, stopped, and fault states.

Commands may be disabled while a backend feature is unavailable, but they must
not disappear to make a single-title demo look simpler.

## Verification

```powershell
gofmt -w .
go test ./...
go vet ./...
```

Linux GUI tests run under Xvfb. Mobile bindings follow the official
`ebitenmobile bind` flow and are packaged by Android Studio/Xcode rather than
pretending a desktop executable is a mobile app.

## Automatic Git handoff

Finishing a successful file-changing task includes committing and pushing the
task's changes. Do this automatically without waiting for a separate request.

1. Run the verification appropriate to the files and risk of the task.
2. Review `git status` and the final diff before staging.
3. Stage only paths or hunks changed for the current task. Use explicit
   pathspecs; never use `git add -A`, `git add .`, or otherwise sweep unrelated
   work into the commit.
4. Create a concise conventional commit that describes the completed task.
5. Push the new commit to the current branch's configured upstream. If the
   branch has no upstream and `origin` is the intended repository, establish it
   with `git push -u origin HEAD`.

Existing user changes are never part of the automatic handoff unless they were
explicitly placed in the current task. Do not commit generated artifacts,
secrets, proprietary evidence, or private test data. If a file mixes task
changes with unrelated user edits, stage only the task hunks when that can be
done safely; otherwise leave it uncommitted and report the conflict.

Do not commit or push failed, incomplete, or unverified work as if it were
finished. Never amend, rebase, force-push, or bypass hooks unless the user
explicitly requests it. An explicit user request not to commit or push takes
precedence. For work spanning multiple repositories, apply this policy
separately in each repository and report each pushed branch and commit hash.
