//go:build js && wasm

package frontend

import (
	"errors"
	"io"
	"io/fs"
)

// readFirstDroppedFile reads the first regular file from a browser
// drag-and-drop into memory. The web/wasm build has no writable filesystem and
// no $HOME/$XDG_CACHE_HOME, so os.UserCacheDir is unavailable; the dropped
// bytes instead travel through OpenRequest.Data, exactly like the web picker's
// selection (see drop_default.go for the desktop cache-copy counterpart).
func readFirstDroppedFile(files fs.FS, results chan<- dropResult) {
	var result dropResult
	errStop := errors.New("dropped file read")
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

		data, err := io.ReadAll(source)
		if err != nil {
			return err
		}
		result.data = data
		result.displayName = entry.Name()
		return errStop
	})
	if errors.Is(err, errStop) {
		err = nil
	}
	if err == nil && result.data == nil {
		err = errors.New("the drop did not contain a regular file")
	}
	result.err = err
	results <- result
}
