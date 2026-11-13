//go:build js && wasm

package frontend

import (
	"strings"
	"syscall/js"
	"testing"
)

func TestWebURLPopupBlockReturnsError(t *testing.T) {
	open := js.FuncOf(func(js.Value, []js.Value) any { return js.Null() })
	defer open.Release()
	setTestWindowOpen(t, open.Value)

	err := openPlatformURL("https://example.invalid/")
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("popup block error = %v", err)
	}
}

func TestWebURLOpenExceptionReturnsError(t *testing.T) {
	open := js.Global().Get("Function").New("throw new Error('open denied')")
	setTestWindowOpen(t, open)

	err := openPlatformURL("https://example.invalid/")
	if err == nil || !strings.Contains(err.Error(), "open denied") {
		t.Fatalf("window.open exception error = %v", err)
	}
}

func TestWebURLJavaScriptExceptionClosesBlankTabAndReturnsError(t *testing.T) {
	closed := false
	closeTab := js.FuncOf(func(js.Value, []js.Value) any {
		closed = true
		return nil
	})
	defer closeTab.Release()
	location := js.Global().Get("Object").New()
	location.Set("replace", js.Global().Get("Function").New("throw new Error('navigation denied')"))
	opened := js.Global().Get("Object").New()
	opened.Set("location", location)
	opened.Set("close", closeTab.Value)
	open := js.FuncOf(func(js.Value, []js.Value) any { return opened })
	defer open.Release()
	setTestWindowOpen(t, open.Value)

	err := openPlatformURL("https://example.invalid/")
	if err == nil || !strings.Contains(err.Error(), "navigation denied") {
		t.Fatalf("navigation exception error = %v", err)
	}
	if !closed {
		t.Fatal("navigation exception left the blank tab open")
	}
}

func TestWebURLCloseExceptionDoesNotMaskNavigationError(t *testing.T) {
	location := js.Global().Get("Object").New()
	location.Set("replace", js.Global().Get("Function").New("throw new Error('navigation denied')"))
	opened := js.Global().Get("Object").New()
	opened.Set("location", location)
	opened.Set("close", js.Global().Get("Function").New("throw new Error('close denied')"))
	open := js.FuncOf(func(js.Value, []js.Value) any { return opened })
	defer open.Release()
	setTestWindowOpen(t, open.Value)

	err := openPlatformURL("https://example.invalid/")
	if err == nil || !strings.Contains(err.Error(), "navigation denied") || strings.Contains(err.Error(), "close denied") {
		t.Fatalf("navigation error after close exception = %v", err)
	}
}

func TestWebURLSuccessSeversOpenerAndPreservesTarget(t *testing.T) {
	var navigated string
	replace := js.FuncOf(func(_ js.Value, args []js.Value) any {
		navigated = args[0].String()
		return nil
	})
	defer replace.Release()
	location := js.Global().Get("Object").New()
	location.Set("replace", replace.Value)
	opened := js.Global().Get("Object").New()
	opened.Set("location", location)
	opened.Set("close", js.Global().Get("Function").New("return undefined"))
	open := js.FuncOf(func(js.Value, []js.Value) any { return opened })
	defer open.Release()
	setTestWindowOpen(t, open.Value)

	target := "https://example.invalid/%2Fpath#fragment"
	if err := openPlatformURL(target); err != nil {
		t.Fatalf("open URL: %v", err)
	}
	if navigated != target {
		t.Fatalf("navigated target = %q", navigated)
	}
	if !opened.Get("opener").IsNull() {
		t.Fatalf("window opener was not severed: %v", opened.Get("opener"))
	}
}

func setTestWindowOpen(t *testing.T, replacement js.Value) {
	t.Helper()
	global := js.Global()
	previous := global.Get("open")
	global.Set("open", replacement)
	t.Cleanup(func() {
		if previous.IsUndefined() {
			global.Delete("open")
			return
		}
		global.Set("open", previous)
	})
}
