package frontend

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
)

// icon_cache.go is the L1 disk cache + PNG decode for launcher game icons. It
// touches the filesystem and image codecs only; it holds no Shell reference.

// iconCacheDir is where extracted game icons are stored, beside settings.json.
func iconCacheDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ARAM", "iconcache"), nil
}

// iconCacheKey identifies a cached icon by the source path plus its size and
// modification time, so replacing a file re-extracts its icon.
func iconCacheKey(path string) string {
	hash := sha256.New()
	hash.Write([]byte(path))
	if info, err := os.Stat(path); err == nil {
		hash.Write([]byte("|" + strconv.FormatInt(info.Size(), 10) +
			"|" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// readIconCache returns the cached PNG bytes for key, if present.
func readIconCache(key string) ([]byte, bool) {
	dir, err := iconCacheDir()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(dir, key+".png"))
	if err != nil {
		return nil, false
	}
	return data, true
}

// writeIconCache stores PNG bytes for key. A cache failure is non-fatal.
func writeIconCache(key string, data []byte) error {
	dir, err := iconCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key+".png"), data, 0o644)
}

// decodeIconPNG decodes PNG bytes to an image. The backend normalizes every
// icon to PNG, so PNG is the only format handled here.
func decodeIconPNG(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}
