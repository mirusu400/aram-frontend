package frontend

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// libraryScanLimit caps how many titles the Home "Installed" tab collects, so a
// scan pointed at a huge tree (a whole drive) stops instead of running for
// minutes and flooding the list.
const libraryScanLimit = 2000

// LibraryEntry is one openable title discovered under a library folder. Name is
// the display label (base filename without its extension); Path is the cleaned
// absolute path passed to the open flow.
type LibraryEntry struct {
	Path string
	Name string
}

// errScanLimit aborts a WalkDir once libraryScanLimit entries are collected.
var errScanLimit = errors.New("library scan limit reached")

// scanLibraryFolders walks each root recursively and returns the titles whose
// filename matches one of patterns (case-insensitively). Results are
// de-duplicated by path across overlapping roots, sorted by name then path, and
// capped at limit. Unreadable directories are skipped rather than failing the
// whole scan.
func scanLibraryFolders(roots, patterns []string, limit int) []LibraryEntry {
	if limit <= 0 {
		limit = libraryScanLimit
	}
	lowered := loweredPatterns(patterns)
	seen := make(map[string]struct{})
	entries := make([]LibraryEntry, 0, 64)
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		walk := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// A folder we cannot read (permissions, a stale root) must not
				// sink the rest of the scan.
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !matchesLoweredPattern(d.Name(), lowered) {
				return nil
			}
			clean := cleanLibraryPath(path)
			if clean == "" {
				return nil
			}
			key := strings.ToLower(clean)
			if _, ok := seen[key]; ok {
				return nil
			}
			seen[key] = struct{}{}
			entries = append(entries, LibraryEntry{Path: clean, Name: libraryEntryName(clean)})
			if len(entries) >= limit {
				return errScanLimit
			}
			return nil
		}
		if err := filepath.WalkDir(root, walk); err != nil && errors.Is(err, errScanLimit) {
			break
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		li, lj := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if li != lj {
			return li < lj
		}
		return entries[i].Path < entries[j].Path
	})
	return entries
}

// libraryEntryName is the base filename without its extension, the label shown
// on a Home row.
func libraryEntryName(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if strings.TrimSpace(base) == "" {
		return filepath.Base(path)
	}
	return base
}

func loweredPatterns(patterns []string) []string {
	lowered := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		lowered = append(lowered, pattern)
	}
	return lowered
}

func matchesLoweredPattern(name string, loweredPatterns []string) bool {
	name = strings.ToLower(name)
	for _, pattern := range loweredPatterns {
		if ok, err := filepath.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}
