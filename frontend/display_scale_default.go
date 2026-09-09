//go:build !android && !ios

package frontend

// Desktop and browser scaling remain unchanged for now. Mobile is the path
// where Ebitengine always exposes the view size in DIP while presenting a
// higher-resolution native surface.
func platformRenderScale() float64 { return 1 }
