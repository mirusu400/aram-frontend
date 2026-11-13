package frontend

import (
	"errors"
	"testing"
)

type recordingURLHost struct {
	urls []string
}

func (host *recordingURLHost) OpenURL(rawURL string) error {
	host.urls = append(host.urls, rawURL)
	return nil
}

func TestNativeURLBridge(t *testing.T) {
	host := &recordingURLHost{}
	SetNativeURLHost(host)
	t.Cleanup(func() { SetNativeURLHost(nil) })
	if err := openNativeURL("https://example.invalid/play"); err != nil {
		t.Fatal(err)
	}
	if len(host.urls) != 1 || host.urls[0] != "https://example.invalid/play" {
		t.Fatalf("opened URLs = %v", host.urls)
	}
	SetNativeURLHost(nil)
	if err := openNativeURL("https://example.invalid/play"); !errors.Is(err, ErrNativeURLUnavailable) {
		t.Fatalf("detached host error = %v", err)
	}
}
