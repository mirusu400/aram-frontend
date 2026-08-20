package frontend

// themeFamilyModern selects the built-in vector-drawn design system; the retro
// families select a sprite skin from the embedded pack in retrothemes/.
const themeFamilyModern = "modern"

// retroFamilies lists the sprite skins in presentation order. Each family has
// a light and a dark variant resolved through the existing ThemeMode setting.
func retroFamilies() []string {
	return []string{"chrome-blue", "candy-orange", "mono-lcd", "glass-touch", "neon-edge"}
}

func isRetroFamily(family string) bool {
	for _, f := range retroFamilies() {
		if f == family {
			return true
		}
	}
	return false
}

// themeFamilyChoices lists every selectable skin, the modern default first.
func themeFamilyChoices() []string {
	return append([]string{themeFamilyModern}, retroFamilies()...)
}

func themeFamilyIndex(family string) int {
	for i, f := range themeFamilyChoices() {
		if f == family {
			return i
		}
	}
	return 0
}

// themeFamilyLabel returns the English display label localization keys use.
func themeFamilyLabel(family string) string {
	switch family {
	case "chrome-blue":
		return "Chrome Blue"
	case "candy-orange":
		return "Candy Orange"
	case "mono-lcd":
		return "Mono LCD"
	case "glass-touch":
		return "Glass Touch"
	case "neon-edge":
		return "Neon Edge"
	default:
		return "Modern"
	}
}

// retroThemeID folds the skin family and the light/dark mode into the theme
// directory name used by the embedded pack.
func retroThemeID(family, mode string) string {
	if mode == "dark" {
		return family + "-dark"
	}
	return family + "-light"
}
