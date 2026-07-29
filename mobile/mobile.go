//go:build android || ios

package mobile

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/mirusu400/aram-frontend/frontend"
)

var game = frontend.NewShell(
	frontend.NullBackend{},
	frontend.NewPlatformPicker(),
	"",
)

func init() {
	mobile.SetGame(game)
}

// OpenDocument is called by the Android/iOS host after its native document
// picker grants access and supplies a backend-readable path or cache handle.
func OpenDocument(path, displayName string) {
	game.OpenExternalDocument(path, displayName, false)
}

func OpenFirmware(path, displayName string) {
	game.OpenExternalDocument(path, displayName, true)
}

// Dummy forces gomobile/ebitenmobile to bind the package.
func Dummy() {}
