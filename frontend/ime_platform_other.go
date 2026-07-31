//go:build !windows

package frontend

import "image"

// Everywhere except Windows the exp/textinput session is safe to use, so it
// drives composition and the frontend renders the preedit inline. See
// ime_platform_windows.go for why Windows needs its own path.
const platformIMEUsesEbitenTextInput = true

// platformIMEAttached is only meaningful on Windows, where the input context
// has to be reattached by hand.
func platformIMEAttached() bool { return true }

func focusPlatformIME(image.Rectangle) {}

func blurPlatformIME() {}

func movePlatformIMECaret(image.Rectangle) {}
