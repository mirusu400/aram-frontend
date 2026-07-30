# ARAM design system

ARAM uses Ebitengine for the game loop, guest framebuffer, input, and mobile
binding. EbitenUI owns application chrome: menus, dropdowns, the application
toolbar, status, configuration, and modal panels.

## Visual language

The shell follows a restrained desktop-emulator layout:

- neutral light and dark palettes avoid tinting the application chrome;
- a small platform-style blue accent is reserved for selection and primary
  actions;
- a conventional emulator menu and action toolbar keep the guest display
  dominant;
- square cards, controls, dropdowns, and dialogs avoid decorative corner
  radii;
- regular, strong, heading, display, and caption faces provide a fixed
  information hierarchy.

## Token contract

`frontend/design_system.go` is the source of truth:

- `ARAMPalette` contains semantic colors such as `Canvas`, `Surface`,
  `TextMuted`, `Accent`, and `Fault`;
- `ARAMSpacing` and `ARAMRadius` define layout rhythm and shape;
- `ARAMTypography` owns embedded, platform-independent Go font faces;
- `ARAMComponents` owns reusable EbitenUI nine-slices and button states.

New UI should consume semantic roles rather than adding raw colors or
component-local spacing. Guest framebuffer scaling and filtering remain in
`frontend/render.go` and must not move into the widget layer.

`ARAMRadius` remains explicit but all production radius tokens are zero.
Light is the default theme; the Appearance settings page switches to the
neutral dark palette at runtime and persists the choice.

Persistent runtime metadata must not consume a sidebar beside the guest
display. The user-enabled virtual keypad is the sole right-rail exception and
reserves space rather than covering the guest image. Configuration belongs in
the category-based `Configure ARAM` modal, opened from
`Emulation > Configure...` or the application toolbar.

The shell uses the current Ebitengine layout dimensions rather than a fixed
internal canvas. Modal bounds are centered and clamped to the viewport, long
configuration categories scroll, and secondary metadata is hidden at compact
desktop widths. The optional desktop virtual keypad scales within its right
rail. On mobile, the touch deck reserves layout space below the guest viewport
instead of covering it.

## Interaction contract

Persistent command IDs and Backend capabilities remain independent from
presentation. Menu and quick-action buttons call the existing command
dispatcher, so desktop shortcuts, native mobile entry points, and injected
backends continue to use the same behavior.

Mobile keeps its persistent touch controls and navigation. Those controls
reuse ARAM component styles while EbitenUI handles the shared overlays and
menus.
