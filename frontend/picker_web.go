//go:build js && wasm

package frontend

import (
	"strings"
	"sync"
	"syscall/js"
)

// webPicker is the web/wasm document picker. A browser has no filesystem path
// for a user selection, so instead of returning a path (desktop) or deferring
// to a native host that later calls OpenExternalDocument (mobile), it opens a
// transient <input type="file">, reads the chosen File into memory, and hands
// the bytes to a sink registered by Run (Shell.OpenExternalBytes). OpenFile
// therefore returns ErrPickerDeferred: the selection completes asynchronously
// on the browser's file-dialog callback, exactly like the mobile bridge.
type webPicker struct{}

// webInputSink receives input bytes read from a browser File. Run registers
// Shell.OpenExternalBytes here so a selection reaches the shell without the
// picker importing shell internals.
type webInputSink func(name string, data []byte, firmware bool)

var webBridge struct {
	sync.RWMutex
	sink webInputSink
}

func setWebInputSink(sink webInputSink) {
	webBridge.Lock()
	webBridge.sink = sink
	webBridge.Unlock()
}

func deliverWebInput(name string, data []byte, firmware bool) {
	webBridge.RLock()
	sink := webBridge.sink
	webBridge.RUnlock()
	if sink != nil {
		sink(name, data, firmware)
	}
}

func NewPlatformPicker() Picker { return webPicker{} }

func (webPicker) OpenFile() (string, error) {
	return "", requestWebFile(false, wipiPackagePatterns())
}

func (webPicker) OpenFontFile() (string, error) {
	return "", ErrPickerUnavailable
}

func (webPicker) OpenFirmwareDirectory(string) (string, error) {
	// A firmware set is a directory of pieces; the browser file dialog cannot
	// deliver one as in-memory bytes, so it is unavailable on web for now.
	return "", ErrPickerUnavailable
}

func (webPicker) ChooseRecent([]string) (string, error) {
	return "", ErrPickerUnavailable
}

// requestWebFile opens the browser file dialog and, on selection, reads the
// first chosen File into a Go byte slice and delivers it to the sink. It
// returns ErrPickerDeferred so the shell shows its waiting state until the
// asynchronous read completes. ErrPickerUnavailable is returned when no DOM is
// present (e.g. a non-browser wasm host).
func requestWebFile(firmware bool, patterns []string) error {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return ErrPickerUnavailable
	}
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", acceptAttribute(patterns))

	var onChange js.Func
	onChange = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer onChange.Release()
		files := input.Get("files")
		if !files.Truthy() || files.Get("length").Int() == 0 {
			return nil
		}
		file := files.Index(0)
		name := file.Get("name").String()
		readBrowserFile(file, func(data []byte) {
			deliverWebInput(name, data, firmware)
		})
		return nil
	})
	input.Call("addEventListener", "change", onChange)
	input.Call("click")
	return ErrPickerDeferred
}

// readBrowserFile reads a browser File's bytes via its arrayBuffer() promise
// and invokes done on the Go/wasm event loop with a copy owned by Go.
func readBrowserFile(file js.Value, done func([]byte)) {
	promise := file.Call("arrayBuffer")

	var onResolve, onReject js.Func
	onResolve = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer onResolve.Release()
		defer onReject.Release()
		if len(args) == 0 {
			return nil
		}
		view := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, view.Get("length").Int())
		js.CopyBytesToGo(data, view)
		done(data)
		return nil
	})
	onReject = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer onResolve.Release()
		defer onReject.Release()
		return nil
	})
	promise.Call("then", onResolve).Call("catch", onReject)
}

// acceptAttribute turns glob patterns like "*.dat" into an <input accept>
// value like ".dat,.jar,.zip".
func acceptAttribute(patterns []string) string {
	exts := make([]string, 0, len(patterns))
	seen := make(map[string]bool, len(patterns))
	for _, pattern := range patterns {
		ext := strings.ToLower(strings.TrimPrefix(pattern, "*"))
		if ext == "" || seen[ext] {
			continue
		}
		seen[ext] = true
		exts = append(exts, ext)
	}
	return strings.Join(exts, ",")
}
