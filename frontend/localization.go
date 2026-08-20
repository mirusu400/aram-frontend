package frontend

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type Language string

const (
	LanguageEnglish Language = "en"
	LanguageKorean  Language = "ko"
)

//go:embed locales/*.json
var localeFiles embed.FS

var localeCatalogs = loadLocaleCatalogs()

func loadLocaleCatalogs() map[Language]map[string]string {
	catalogs := make(map[Language]map[string]string, 2)
	for _, language := range []Language{LanguageEnglish, LanguageKorean} {
		data, err := localeFiles.ReadFile("locales/" + string(language) + ".json")
		if err != nil {
			panic("load " + string(language) + " locale: " + err.Error())
		}
		catalog := make(map[string]string)
		if err := json.Unmarshal(data, &catalog); err != nil {
			panic("parse " + string(language) + " locale: " + err.Error())
		}
		catalogs[language] = catalog
	}
	return catalogs
}

func normalizeLanguage(value string) Language {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(LanguageKorean):
		return LanguageKorean
	default:
		return LanguageEnglish
	}
}

func languageFromLocale(locale string) Language {
	locale = strings.ToLower(strings.TrimSpace(locale))
	locale = strings.ReplaceAll(locale, "_", "-")
	if locale == "ko" ||
		strings.HasPrefix(locale, "ko-") ||
		strings.HasPrefix(locale, "ko.") ||
		strings.HasPrefix(locale, "ko@") ||
		strings.HasPrefix(locale, "ko:") {
		return LanguageKorean
	}
	return LanguageEnglish
}

// hostLocale is a locale tag declared by a native host. Android starts Go
// without the environment variables the desktop platforms expose, so its
// binding hands the device language in before the shell loads settings; the
// desktop builds leave this empty and read the OS directly.
var hostLocale struct {
	sync.RWMutex
	tag string
}

// SetHostLocale records the device language for platforms Go cannot query.
// It only affects the first-run default, so it has to be called before the
// Shell is constructed; afterwards the saved language setting wins.
func SetHostLocale(tag string) {
	hostLocale.Lock()
	hostLocale.tag = tag
	hostLocale.Unlock()
}

func platformLocale() string {
	hostLocale.RLock()
	tag := hostLocale.tag
	hostLocale.RUnlock()
	if tag != "" {
		return tag
	}
	return osLocale()
}

func systemLanguage() Language {
	return languageFromLocale(platformLocale())
}

func translate(language Language, message string) string {
	language = normalizeLanguage(string(language))
	if translated := localeCatalogs[language][message]; translated != "" {
		return translated
	}
	if fallback := localeCatalogs[LanguageEnglish][message]; fallback != "" {
		return fallback
	}
	return message
}

func translatef(language Language, message string, args ...any) string {
	return fmt.Sprintf(translate(language, message), args...)
}

func languageLabel(language Language, displayLanguage Language) string {
	switch normalizeLanguage(string(language)) {
	case LanguageKorean:
		return translate(displayLanguage, "Korean")
	default:
		return translate(displayLanguage, "English")
	}
}

func settingValueLabel(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func stateValueLabel(value string) string {
	switch value {
	case "backend-unavailable":
		return "Backend unavailable"
	case "guest-faulted":
		return "Guest faulted"
	case "malformed-input":
		return "Malformed input"
	case "unsupported-profile":
		return "Unsupported profile"
	default:
		return settingValueLabel(value)
	}
}
