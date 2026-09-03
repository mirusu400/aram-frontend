package frontend

// Experimental widescreen: an optional override of the guest framebuffer width.
// A camera-scrolled title (e.g. an RPG that lays its world out from the
// runtime-reported screen size) fills the extra area with more world; a title
// that hardcodes its layout leaves margins. Height stays at the device-native
// size. The choice takes effect the next time a title is opened.

// nativeGuestHeight is the handset height the widescreen override pairs with the
// chosen width. It matches the common 240x320 portrait handset (제노니아 and most
// LGT/KTF titles); the override widens only, so the height is held here. Titles
// authored for a different native height are out of scope for this experiment.
const nativeGuestHeight = 320

// widescreenChoices lists the offered guest widths in dropdown order. Zero means
// the device-native width (off).
func widescreenChoices() []int {
	return []int{0, 320, 384, 480, 640}
}

// widescreenLabel returns the localized display label for a guest-width choice.
func (s *Shell) widescreenLabel(width int) string {
	if width <= 0 {
		return s.tr("Off (native width)")
	}
	return s.trf("%d px wide", width)
}

// widescreenChoiceIndex returns the dropdown index of the current setting,
// keeping an out-of-list saved value visible by appending it.
func (s *Shell) widescreenChoiceIndex(choices []int) int {
	for i, w := range choices {
		if w == s.settings.GuestWidthOverride {
			return i
		}
	}
	return 0
}

// currentDisplaySettings maps the saved width override to a DisplaySettings the
// backend understands. Height mirrors the native handset height so only the
// width widens.
func (s *Shell) currentDisplaySettings() DisplaySettings {
	width := s.settings.GuestWidthOverride
	if width <= 0 {
		return DisplaySettings{}
	}
	return DisplaySettings{Width: width, Height: nativeGuestHeight}
}

// setGuestWidthOverride selects a guest framebuffer width and applies it.
func (s *Shell) setGuestWidthOverride(width int) {
	if width < 0 {
		width = 0
	}
	s.settings.GuestWidthOverride = width
	s.applyDisplaySettings()
}

// applyDisplaySettings persists the selection and pushes it to the backend. The
// new geometry takes effect the next time a title is opened.
func (s *Shell) applyDisplaySettings() {
	_ = s.settings.save()
	if configurator, ok := s.backend.(DisplayConfigurator); ok {
		if err := configurator.ConfigureDisplay(s.currentDisplaySettings()); err != nil {
			s.setStatus(s.tr("Widescreen: ") + err.Error())
			return
		}
	}
	s.setStatus(s.trf(
		"Widescreen: %s (restart title to apply)",
		s.widescreenLabel(s.settings.GuestWidthOverride),
	))
}
