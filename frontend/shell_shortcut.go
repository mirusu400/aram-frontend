package frontend

import (
	"bytes"
	"image/png"
)

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
	s.setStatus(s.trf("Adding %s to Home screen...", title))
	go func() {
		var icon []byte
		if source, ok := s.backend.(IconBackend); ok {
			if img := loadOrFetchIcon(source, path); img != nil {
				var buffer bytes.Buffer
				if err := png.Encode(&buffer, img); err == nil && buffer.Len() <= 512*1024 {
					icon = buffer.Bytes()
				}
			}
		}
		if err := host.PinGameShortcut(path, title, icon); err != nil {
			s.ReportExternalOpenStatus("Shortcut: " + err.Error())
		}
	}()
}
