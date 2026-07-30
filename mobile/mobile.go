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

// Host is implemented by the generated Android/iOS application layer. The
// native host owns SAF/UIDocumentPicker presentation and later calls
// OpenDocument, OpenFirmware, or DocumentSelectionCanceled.
type Host interface {
	RequestDocument(firmware bool)
}

func init() {
	mobile.SetGame(game)
}

func SetHost(host Host) {
	frontend.SetNativePickerHost(host)
}

// OpenDocument is called by the Android/iOS host after its native document
// picker grants access and supplies a backend-readable path or cache handle.
func OpenDocument(path, displayName string) {
	game.OpenExternalDocument(path, displayName, false)
}

func OpenFirmware(path, displayName string) {
	game.OpenExternalDocument(path, displayName, true)
}

func DocumentSelectionCanceled() {
	game.CancelExternalDocumentSelection()
}

// Command invokes a stable frontend command ID from the native host.
func Command(commandID string) {
	game.DispatchExternalCommand(commandID)
}

// Pause and Resume are native lifecycle hooks. Manual user pauses are not
// resumed automatically.
func Pause() {
	game.SetHostActive(false)
}

func Resume() {
	game.SetHostActive(true)
}

// AudioFocus mirrors native audio-focus/interruption callbacks. Losing focus
// uses the same automatic pause contract as lifecycle deactivation.
func AudioFocus(active bool) {
	game.SetHostActive(active)
}

// Dummy forces gomobile/ebitenmobile to bind the package.
func Dummy() {}
