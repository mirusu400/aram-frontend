package frontend

import "image/color"

// retroPaletteSpec carries the semantic colors one retro theme overrides on
// top of the base light/dark palette. Status colors (Success/Warning/Fault)
// and the scrim stay on the neutral base values so system feedback reads the
// same in every skin.
type retroPaletteSpec struct {
	canvas        string
	canvasRaised  string
	surface       string
	surfaceHover  string
	border        string
	borderStrong  string
	text          string
	textMuted     string
	onAccent      string
	accent        string
	accentHover   string
	accentPressed string
	accentSoft    string
}

// The values are lifted from the pack's palette source (retrothemes were
// generated from these exact colors), so text and custom-drawn chrome always
// match the sprites.
var retroPaletteSpecs = map[string]retroPaletteSpec{
	"chrome-blue-light": {
		canvas: "#F4F6FA", canvasRaised: "#C6CEDA", surface: "#E7EBF1",
		surfaceHover: "#D4DCE8", border: "#5A6B80", borderStrong: "#2C3A4E",
		text: "#1B2430", textMuted: "#67748A", onAccent: "#FFFFFF",
		accent: "#2F7FD1", accentHover: "#5AA1E8", accentPressed: "#1B5FA8",
		accentSoft: "#8FC4F5",
	},
	"chrome-blue-dark": {
		canvas: "#12161B", canvasRaised: "#171C23", surface: "#242A33",
		surfaceHover: "#3E4652", border: "#252C36", borderStrong: "#545D6A",
		text: "#E6EAF0", textMuted: "#8C97A6", onAccent: "#FFFFFF",
		accent: "#256CB4", accentHover: "#3D8DDA", accentPressed: "#17507F",
		accentSoft: "#0E3A61",
	},
	"candy-orange-light": {
		canvas: "#FFFBF3", canvasRaised: "#EFD8BC", surface: "#FFF3E2",
		surfaceHover: "#FFE9CB", border: "#9C7A54", borderStrong: "#5A3A1C",
		text: "#3E2711", textMuted: "#8E6B45", onAccent: "#FFFFFF",
		accent: "#F4741B", accentHover: "#FF9E42", accentPressed: "#CC530A",
		accentSoft: "#FFCE96",
	},
	"candy-orange-dark": {
		canvas: "#181008", canvasRaised: "#1D160E", surface: "#2E2318",
		surfaceHover: "#4E3C2E", border: "#43301F", borderStrong: "#8A6E57",
		text: "#F6E6D2", textMuted: "#B2937A", onAccent: "#2A1403",
		accent: "#D06E12", accentHover: "#F5942E", accentPressed: "#9C4E07",
		accentSoft: "#6E3604",
	},
	"mono-lcd-light": {
		canvas: "#CFDCB2", canvasRaised: "#A9BA88", surface: "#C2D1A2",
		surfaceHover: "#B8C89A", border: "#5E6B40", borderStrong: "#2A3418",
		text: "#212A12", textMuted: "#5E6B40", onAccent: "#CFDCB2",
		accent: "#2A3418", accentHover: "#43502A", accentPressed: "#212A12",
		accentSoft: "#9CAD7B",
	},
	"mono-lcd-dark": {
		canvas: "#081019", canvasRaised: "#0A1420", surface: "#101E2C",
		surfaceHover: "#11202E", border: "#2E5C78", borderStrong: "#5FB4E0",
		text: "#9FDCFA", textMuted: "#4E88A8", onAccent: "#06131E",
		accent: "#4FA8DC", accentHover: "#6FC4F2", accentPressed: "#3B87B4",
		accentSoft: "#1C3D54",
	},
	"glass-touch-light": {
		canvas: "#F7FAFD", canvasRaised: "#C8D3DF", surface: "#EAF0F6",
		surfaceHover: "#DAE3EE", border: "#AFBAC6", borderStrong: "#6E7C8C",
		text: "#16212E", textMuted: "#6B7A8C", onAccent: "#FFFFFF",
		accent: "#1E97E8", accentHover: "#63C0F8", accentPressed: "#0B6FB4",
		accentSoft: "#AEE2FF",
	},
	"glass-touch-dark": {
		canvas: "#080C10", canvasRaised: "#0B1016", surface: "#161D25",
		surfaceHover: "#252E36", border: "#1E2831", borderStrong: "#5A6874",
		text: "#E8F2F8", textMuted: "#7C8B99", onAccent: "#032430",
		accent: "#16A3E0", accentHover: "#4FC6F5", accentPressed: "#0A76A8",
		accentSoft: "#123B52",
	},
	"neon-edge-light": {
		canvas: "#F6F7FA", canvasRaised: "#E7EAEF", surface: "#FFFFFF",
		surfaceHover: "#F6F7F9", border: "#E9A0BE", borderStrong: "#D6165F",
		text: "#1A1A20", textMuted: "#7A7A88", onAccent: "#FFFFFF",
		accent: "#E5106B", accentHover: "#FF4E97", accentPressed: "#B60B53",
		accentSoft: "#FF9CC6",
	},
	"neon-edge-dark": {
		canvas: "#040406", canvasRaised: "#070709", surface: "#0C0C12",
		surfaceHover: "#191921", border: "#6E1440", borderStrong: "#FF2E8A",
		text: "#F4E9EF", textMuted: "#9A7A8C", onAccent: "#1A0009",
		accent: "#FF1478", accentHover: "#FF4E9E", accentPressed: "#C40D5C",
		accentSoft: "#2A0A1C",
	},
}

// retroPalette resolves a theme's semantic colors, keeping the base palette's
// status and overlay roles untouched.
func retroPalette(theme string, base ARAMPalette) ARAMPalette {
	spec, ok := retroPaletteSpecs[theme]
	if !ok {
		return base
	}
	p := base
	p.Canvas = hexNRGBA(spec.canvas)
	p.CanvasRaised = hexNRGBA(spec.canvasRaised)
	p.Surface = hexNRGBA(spec.surface)
	p.SurfaceRaised = hexNRGBA(spec.surface)
	p.SurfaceHover = hexNRGBA(spec.surfaceHover)
	p.Border = hexNRGBA(spec.border)
	p.BorderStrong = hexNRGBA(spec.borderStrong)
	p.Text = hexNRGBA(spec.text)
	p.TextMuted = hexNRGBA(spec.textMuted)
	p.TextDisabled = mixNRGBA(p.TextMuted, p.Surface, 0.45)
	p.OnAccent = hexNRGBA(spec.onAccent)
	p.Accent = hexNRGBA(spec.accent)
	p.AccentHover = hexNRGBA(spec.accentHover)
	p.AccentPressed = hexNRGBA(spec.accentPressed)
	p.AccentSoft = hexNRGBA(spec.accentSoft)
	return p
}

func hexNRGBA(s string) color.NRGBA {
	hexDigit := func(b byte) uint8 {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		panic("invalid hex color: " + s)
	}
	if len(s) != 7 || s[0] != '#' {
		panic("invalid hex color: " + s)
	}
	return color.NRGBA{
		R: hexDigit(s[1])<<4 | hexDigit(s[2]),
		G: hexDigit(s[3])<<4 | hexDigit(s[4]),
		B: hexDigit(s[5])<<4 | hexDigit(s[6]),
		A: 0xff,
	}
}

// mixNRGBA blends a toward b by t (0..1) in straight NRGBA space.
func mixNRGBA(a, b color.NRGBA, t float64) color.NRGBA {
	lerp := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5)
	}
	return color.NRGBA{
		R: lerp(a.R, b.R),
		G: lerp(a.G, b.G),
		B: lerp(a.B, b.B),
		A: lerp(a.A, b.A),
	}
}
