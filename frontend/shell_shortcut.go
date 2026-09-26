package frontend

// pinHomeShortcut sends the selected launcher row to the mobile host. Icon
// extraction may read an archive, so it runs away from Ebitengine's UI loop.
func (s *Shell) pinHomeShortcut(path string) {
	host := currentNativeShortcutHost()
	if path == "" || host == nil {
		return
	}
	title := s.rememberedDisplayName(path)
	for _, entry := range s.homeTabEntries(s.homeTab) {
		if entry.Path == path && entry.Name != "" {
			title = entry.Name
			break
		}
	}
	if title == "" {
		title = libraryEntryName(path)
	}
	noIconStatus := s.tr("Game icon unavailable; cannot create Home screen shortcut")
	s.setStatus(s.trf("Adding %s to Home screen...", title))
	go func() {
		var icon []byte
		if source, ok := s.backend.(IconBackend); ok {
			icon = loadShortcutIconPNG(source, path)
		}
		if len(icon) == 0 {
			s.ReportExternalOpenStatus(noIconStatus)
			return
		}
		if err := host.PinGameShortcut(path, title, icon); err != nil {
			s.ReportExternalOpenStatus("Shortcut: " + err.Error())
		}
	}()
}
