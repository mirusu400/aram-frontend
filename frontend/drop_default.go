//go:build !js || !wasm

package frontend

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// readFirstDroppedFile copies the first regular file from a desktop
// drag-and-drop into a private cache directory and reports its path. The shell
// then opens that path and removes the copy once the input is released. The
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
