package frontend

import (
	"context"
	"sync"

	"golang.design/x/clipboard"
)

var (
	clipboardInitOnce sync.Once
	clipboardInitErr  error
)

func writeClipboardText(value string) error {
	clipboardInitOnce.Do(func() {
		clipboardInitErr = clipboard.Init()
	})
	if clipboardInitErr != nil {
		return clipboardInitErr
	}
	_, err := clipboard.Write(
		context.Background(),
		clipboard.FmtText,
		[]byte(value),
	)
	return err
}
