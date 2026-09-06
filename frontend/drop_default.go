//go:build !js || !wasm

package frontend

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// readFirstDroppedFile reports the first regular file from a desktop
// drag-and-drop. Ebitengine's dropped-file system knows the real path of each
// file (see realDroppedPath), and that path is what gets opened, so the title
// lands in the recent list like one chosen through the file dialog. When the
// real path is unavailable the file is copied into a private cache directory
// and opened as a temporary input the shell removes once it is released. The
// web counterpart in drop_web.go returns the bytes in memory instead, because
// the browser build has no writable cache directory.
func readFirstDroppedFile(files fs.FS, results chan<- dropResult) {
	var result dropResult
	errStop := errors.New("dropped file copied")
	err := fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		source, err := files.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()

		if real := realDroppedPath(source); real != "" {
			result.path = real
			result.displayName = entry.Name()
			return errStop
		}
		result.temporary = true

		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		cacheRoot = filepath.Join(cacheRoot, "ARAM", "drops")
		if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
			return err
		}
		extension := filepath.Ext(entry.Name())
		target, err := os.CreateTemp(cacheRoot, "drop-*"+extension)
		if err != nil {
			return err
		}
		targetPath := target.Name()
		if _, err := io.Copy(target, source); err != nil {
			_ = target.Close()
			_ = os.Remove(targetPath)
			return err
		}
		if err := target.Close(); err != nil {
			_ = os.Remove(targetPath)
			return err
		}
		result.path = targetPath
		result.displayName = entry.Name()
		return errStop
	})
	if errors.Is(err, errStop) {
		err = nil
	}
	if err == nil && result.path == "" {
		err = errors.New("the drop did not contain a regular file")
	}
	result.err = err
	results <- result
}

// realDroppedPath returns the on-disk path behind a dropped file when the
// file system exposes one, or "" when it does not. Ebitengine's desktop
// dropped-file system wraps real files and reports their absolute path
// through AbsPath; the path is used only when it still names a regular file.
func realDroppedPath(source fs.File) string {
	located, ok := source.(interface{ AbsPath() string })
	if !ok {
		return ""
	}
	path := located.AbsPath()
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}
