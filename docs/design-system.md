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

Light is the default theme; the Appearance settings page switches to the
neutral dark palette at runtime and persists the choice.

## Sprite skins

Besides the vector-drawn `modern` family, the Appearance page offers five
retro sprite skins (`chrome-blue`, `candy-orange`, `mono-lcd`, `glass-touch`,
`neon-edge`) embedded under `frontend/retrothemes/` (original CC0 artwork).
Each family ships a light and a dark variant selected by the existing theme
mode, so the skin choice and the light/dark toggle stay independent settings
(`theme_family` and `theme_mode`). `newARAMDesignSystem` resolves both into
one design system: the retro path swaps `ARAMPalette` and `ARAMComponents`
for sprite-backed equivalents (`design_system_retro*.go`) while spacing
and the token contract stay unchanged. Switching either setting
rebuilds the design system and shell UI at runtime; no restart is required.

Sprite skins also swap the whole type ramp onto **Terrarum Sans Bitmap**, a
pixel font with full Hangul coverage (embedded under
`frontend/assets/terrarum/`, SIL OFL 1.1 — its license ships next to the
file). Sizes stay on the font's native pixel grid: 20px for captions, body,
and strong text, 40px for headings and display text. The face has a single
weight, matching how era handsets drew their shells. Dialog titles use the
dedicated `OnTitle` palette role so the title ink matches each skin's
gradient, and soft-key primaries use `OnWarm`.

Two rules keep a swapped type ramp vertically centered. A `MultiFace` reports
the largest ascent among its members and widgets center text by that line box,
so the Noto fallback runs at 80% of the nominal size and the combined metrics
stay the pixel font's own. `ARAMTypography.CenterNudge` then carries the pixel
or two a face's ink sits below its line-box center; EbitenUI widgets get it as
text padding on the theme (negative top, equal bottom, so preferred sizes are
unchanged) and custom-drawn labels get it through `centeredTextTop`, which
measures the face instead of assuming a text height.

Sprite skins go beyond the nine-slice chrome: the application toolbar swaps
its text actions for the pack's 16×16 pixel icons (inverted ink on pressed
faces, faded for disabled), primary actions wear the era's soft-key face with
the pack's `text_on_warm` ink, the menu highlight is the accent gradient
selection bar, and the guest viewport is framed by the pack's LCD bezel with
`lcd_bg`/`lcd_ink` driving the empty-state screen through the
`GuestSurface`/`GuestInk` palette roles.
Every retro tile is 17×17 with an 8px fixed border and a 1px stretchable
center, which keeps gradients from banding at any control size.

The `Configure ARAM` panel derives its geometry from the active ramp rather
than from the modern faces it was first drawn for
(`frontend/ui_settings_metrics.go`): the dialog scales with the body line
height, the action column is measured from the widest control label in the
section, and each row's label and description wrap into what is left. Rows are
stretched to the panel width, and EbitenUI's anchor layout sizes a row from
its first child, so unwrapped copy silently widens the scroll content and
carries the right-anchored control out of view — the state the pixel ramp
first exposed.

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
