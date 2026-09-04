package frontend

import (
	"path/filepath"
	"strings"
)

// Home library + favorites settings helpers. These operate purely on the
// stored path lists (GameLibraryFolders, FavoriteFiles); the Shell owns saving
// and re-scanning.

// cleanPathList absolutizes, cleans, and case-insensitively de-duplicates a
// list of filesystem paths while preserving order, dropping blanks. It backs
// the Home library folders and favorites so a hand-edited settings.json never
// lists the same entry twice.
func cleanPathList(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = cleanLibraryPath(path)
		if path == "" {
			continue
		}
		key := strings.ToLower(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, path)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

// cleanLibraryPath normalizes a single path the way addRecent does: absolute
// when possible, then cleaned. A blank input stays blank.
func cleanLibraryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

// addLibraryFolder appends a scan root, cleaned and de-duplicated. It reports
// whether the set changed so the caller can skip a redundant save + rescan.
func (s *Settings) addLibraryFolder(path string) bool {
	path = cleanLibraryPath(path)
	if path == "" {
		return false
	}
	for _, existing := range s.GameLibraryFolders {
		if strings.EqualFold(existing, path) {
			return false
		}
	}
	s.GameLibraryFolders = append(s.GameLibraryFolders, path)
	return true
}

// removeLibraryFolder drops a scan root. It reports whether anything changed.
func (s *Settings) removeLibraryFolder(path string) bool {
	path = cleanLibraryPath(path)
	if path == "" {
		return false
	}
	kept := s.GameLibraryFolders[:0]
	removed := false
	for _, existing := range s.GameLibraryFolders {
		if strings.EqualFold(existing, path) {
			removed = true
			continue
		}
		kept = append(kept, existing)
	}
	if !removed {
		return false
	}
	if len(kept) == 0 {
		s.GameLibraryFolders = nil
	} else {
		s.GameLibraryFolders = kept
	}
	return true
}

// isFavorite reports whether path is starred.
func (s *Settings) isFavorite(path string) bool {
	path = cleanLibraryPath(path)
	if path == "" {
		return false
	}
	for _, existing := range s.FavoriteFiles {
		if strings.EqualFold(existing, path) {
			return true
		}
	}
	return false
}

// toggleFavorite adds path to the favorites list if absent, or removes it if
// present. It reports the resulting starred state.
func (s *Settings) toggleFavorite(path string) bool {
	cleaned := cleanLibraryPath(path)
	if cleaned == "" {
		return false
	}
	kept := s.FavoriteFiles[:0]
	removed := false
	for _, existing := range s.FavoriteFiles {
		if strings.EqualFold(existing, cleaned) {
			removed = true
			continue
		}
		kept = append(kept, existing)
	}
	if removed {
		if len(kept) == 0 {
			s.FavoriteFiles = nil
		} else {
			s.FavoriteFiles = kept
		}
		return false
	}
	s.FavoriteFiles = append(s.FavoriteFiles, cleaned)
	return true
}
