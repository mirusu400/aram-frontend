package frontend

import "math"

// normalizedRenderScale keeps display metadata failures from collapsing the
// render target. Fractional Android densities are deliberately preserved: an
// integer approximation would make Ebitengine resample the whole final frame.
func normalizedRenderScale(scale float64) float64 {
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale < 1 {
		return 1
	}
	return scale
}

func scaledPixels(value int, scale float64) int {
	if value == 0 {
		return 0
	}
	result := int(math.Round(float64(value) * normalizedRenderScale(scale)))
	if result == 0 {
		if value < 0 {
			return -1
		}
		return 1
	}
	return result
}

func scaledScreenSize(width, height int, scale float64) (int, int) {
	scale = normalizedRenderScale(scale)
	return max(1, scaledPixels(width, scale)), max(1, scaledPixels(height, scale))
}
