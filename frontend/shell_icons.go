package frontend

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// shell_icons.go (L2) owns the launcher's per-title icons: it fetches them from
// the backend off the UI goroutine, caches them to disk, and hands finished
// ebiten images to the UI. The UI asks with requestHomeIcon / homeIcon.

// iconResult carries a decoded icon (or a nil img for "no icon") from a fetch
// goroutine back to the Update loop, where the ebiten image is created.
type iconResult struct {
	path string
	img  image.Image
}

// homeIcon returns the loaded icon for a title, or nil when none is available
// (not fetched yet, no backend, or the format carries no icon).
func (s *Shell) homeIcon(path string) *ebiten.Image {
	return s.iconImages[path]
}

// requestHomeIcon ensures an icon fetch is in flight (or done) for path. It is
// idempotent: a loaded, missing, or pending path is a no-op. Called by the UI
// while building the visible rows, so only shown titles are fetched.
func (s *Shell) requestHomeIcon(path string) {
	if path == "" {
		return
	}
	if s.iconImages[path] != nil || s.iconMissing[path] || s.iconPending[path] {
		return
	}
	source, ok := s.backend.(IconBackend)
	if !ok {
		// No backend can supply icons (e.g. the standalone preview): fall back
		// to the color tile and never retry.
		s.iconMissing[path] = true
		return
	}
	s.iconPending[path] = true
	go func() {
		s.iconSem <- struct{}{}
		defer func() { <-s.iconSem }()
		img := loadOrFetchIcon(source, path)
		s.iconResults <- iconResult{path: path, img: img}
	}()
}

// consumeIconResult applies a finished fetch on the Update goroutine, where an
// ebiten image may be created. It is called from consumeResults.
func (s *Shell) consumeIconResult(result iconResult) {
	delete(s.iconPending, result.path)
	if result.img != nil {
		s.iconImages[result.path] = ebiten.NewImageFromImage(result.img)
	} else {
		s.iconMissing[result.path] = true
	}
}

// loadOrFetchIcon returns a title's icon from the disk cache, or fetches it from
// the backend (caching the result). It runs on a fetch goroutine and returns a
// plain image.Image; the ebiten image is built on the Update goroutine.
func loadOrFetchIcon(source IconBackend, path string) image.Image {
	key := iconCacheKey(path)
	if data, ok := readIconCache(key); ok {
		if img, err := decodeIconPNG(data); err == nil {
			return img
		}
	}
	data, err := source.Icon(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	_ = writeIconCache(key, data)
	img, err := decodeIconPNG(data)
	if err != nil {
		return nil
	}
	return img
}
