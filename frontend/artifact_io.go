package frontend

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var openArtifactFolder = openPlatformFolder

func cloneImage(source image.Image) *image.RGBA {
	bounds := source.Bounds()
	result := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(result, result.Bounds(), source, bounds.Min, draw.Src)
	return result
}

func writePNGArtifact(directory, prefix string, source image.Image) (string, error) {
	root, err := artifactDirectory(directory)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, timestampedName(prefix, ".png"))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func writeTextArtifact(directory, prefix, extension string, data []byte) (string, error) {
	root, err := artifactDirectory(directory)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, timestampedName(prefix, extension))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func artifactDirectory(name string) (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "ARAM", name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func timestampedName(prefix, extension string) string {
	return fmt.Sprintf("%s-%s%s", prefix, time.Now().Format("20060102-150405.000"), extension)
}

func copyFirstDroppedFile(files fs.FS, results chan<- dropResult) {
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

func removeTemporaryDrop(path string) {
	if path == "" {
		return
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return
	}
	expectedRoot := filepath.Clean(filepath.Join(cacheRoot, "ARAM", "drops"))
	cleaned := filepath.Clean(path)
	relative, err := filepath.Rel(expectedRoot, cleaned)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return
	}
	_ = os.Remove(cleaned)
}
