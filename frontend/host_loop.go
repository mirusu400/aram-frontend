package frontend

import "github.com/hajimehoshi/ebiten/v2"

// hostLoopVsyncOffTPS is the Update rate used while vsync is off. Draw then
// runs unthrottled, so ticks cannot follow the display; a fixed rate above
// sixty keeps input polling and guest scheduling finer than the default
// without letting Update spin as fast as Draw does.
const hostLoopVsyncOffTPS = 120

// applyHostLoop configures how often ebiten calls Update and whether Draw
// waits for the display.
//
// With vsync on, Update is tied to Draw (one tick per refresh). That is what
// lets the guest be scheduled per display frame rather than per fixed 60 Hz
// tick, and it halves the worst-case input-to-guest delay on a fast display.
// With vsync off, Draw runs as fast as the host can manage and Update falls
// back to a fixed rate.
//
// Both calls are plain atomics before the game loop starts, so this is safe to
// run from NewShell and from a settings toggle alike.
func applyHostLoop(vsyncDisabled bool) {
	if vsyncDisabled {
		ebiten.SetVsyncEnabled(false)
		ebiten.SetTPS(hostLoopVsyncOffTPS)
		return
	}
	ebiten.SetVsyncEnabled(true)
	ebiten.SetTPS(ebiten.SyncWithFPS)
}
