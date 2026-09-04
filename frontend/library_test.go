package frontend

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates parents and an empty file, failing the test on error.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanLibraryFoldersFindsNestedMatchesIgnoresOthers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "alpha.dat"))
	writeFile(t, filepath.Join(root, "sub", "bravo.jar"))
	writeFile(t, filepath.Join(root, "sub", "deep", "charlie.MBN")) // case-insensitive
	writeFile(t, filepath.Join(root, "notes.txt"))                  // ignored
	writeFile(t, filepath.Join(root, "sub", "readme.md"))           // ignored

	entries := scanLibraryFolders([]string{root}, supportedInputPatterns(), 0)
	if len(entries) != 3 {
		t.Fatalf("scan found %d entries, want 3: %+v", len(entries), entries)
	}
	// Sorted by name, so alpha, bravo, charlie.
	wantNames := []string{"alpha", "bravo", "charlie"}
	for index, want := range wantNames {
		if entries[index].Name != want {
			t.Fatalf("entry %d name = %q, want %q", index, entries[index].Name, want)
		}
	}
}

func TestScanLibraryFoldersDeduplicatesOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "game.dat"))
	sub := filepath.Join(root, "inner")
	writeFile(t, filepath.Join(sub, "child.jar"))

	// root and its own subfolder overlap; child.jar must appear once.
	entries := scanLibraryFolders([]string{root, sub}, supportedInputPatterns(), 0)
	if len(entries) != 2 {
		t.Fatalf("overlapping roots produced %d entries, want 2: %+v", len(entries), entries)
	}
	seen := map[string]int{}
	for _, entry := range entries {
		seen[entry.Path]++
	}
	for path, count := range seen {
		if count != 1 {
			t.Fatalf("path %q listed %d times", path, count)
		}
	}
}

func TestScanLibraryFoldersHonorsLimit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.dat", "b.dat", "c.dat", "d.dat"} {
		writeFile(t, filepath.Join(root, name))
	}
	entries := scanLibraryFolders([]string{root}, supportedInputPatterns(), 2)
	if len(entries) != 2 {
		t.Fatalf("limit produced %d entries, want 2", len(entries))
	}
}

func TestScanLibraryFoldersSkipsMissingRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "here.dat"))
	missing := filepath.Join(root, "does-not-exist")

	entries := scanLibraryFolders([]string{missing, root, ""}, supportedInputPatterns(), 0)
	if len(entries) != 1 || entries[0].Name != "here" {
		t.Fatalf("scan across a missing root = %+v", entries)
	}
}
