//go:build android || ios

package frontend

import "github.com/hajimehoshi/ebiten/v2"

func platformRenderScale() float64 {
	return normalizedRenderScale(ebiten.Monitor().DeviceScaleFactor())
}
