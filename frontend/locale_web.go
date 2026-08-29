//go:build js && wasm

package frontend

import "syscall/js"

// osLocale reads the browser's preferred language so a first visit starts ARAM
// in the viewer's browser/OS language. Under GOOS=js the LC_ALL/LANG/... env
// vars the generic osLocale checks are always empty, so without this the web
// build would ignore a Korean Chrome and always default to English. It only
// sets the first-run default; once a language is saved that choice wins on
// later loads. Returns "" if navigator is missing or throws (older or
// locked-down contexts), leaving the caller's English fallback in place.
func osLocale() (tag string) {
	defer func() { _ = recover() }()
	global := js.Global()
	if !global.Truthy() {
		return ""
	}
	navigator := global.Get("navigator")
	if !navigator.Truthy() {
		return ""
	}
	// navigator.languages is the ordered preference list; its first entry is
	// the most preferred and matches navigator.language on Chrome.
	if languages := navigator.Get("languages"); languages.Truthy() && languages.Length() > 0 {
		if first := languages.Index(0); first.Truthy() {
			return first.String()
		}
	}
	if language := navigator.Get("language"); language.Truthy() {
		return language.String()
	}
	return ""
}
