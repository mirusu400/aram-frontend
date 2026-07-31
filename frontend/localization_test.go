package frontend

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"
)

var formatVerbPattern = regexp.MustCompile(
	`%(?:\[[0-9]+\])?[-+# 0]*(?:[0-9]+|\*)?(?:\.(?:[0-9]+|\*))?[a-zA-Z%]`,
)

func TestLanguageFromLocaleUsesKoreanOnlyForKoreanLocale(t *testing.T) {
	tests := map[string]Language{
		"ko":          LanguageKorean,
		"ko-KR":       LanguageKorean,
		"ko_KR.UTF-8": LanguageKorean,
		"ko.UTF-8":    LanguageKorean,
		"ko:en":       LanguageKorean,
		"en-US":       LanguageEnglish,
		"ja-JP":       LanguageEnglish,
		"":            LanguageEnglish,
	}
	for locale, want := range tests {
		if got := languageFromLocale(locale); got != want {
			t.Errorf("languageFromLocale(%q) = %q, want %q", locale, got, want)
		}
	}
}

func TestLocaleCatalogsHaveMatchingKeys(t *testing.T) {
	english := reflect.ValueOf(localeCatalogs[LanguageEnglish]).MapKeys()
	korean := reflect.ValueOf(localeCatalogs[LanguageKorean]).MapKeys()
	if len(english) != len(korean) {
		t.Fatalf("locale key count: en=%d ko=%d", len(english), len(korean))
	}
	for _, key := range english {
		message := key.String()
		if localeCatalogs[LanguageKorean][message] == "" {
			t.Errorf("Korean locale is missing %q", message)
		}
		sourceVerbs := formatVerbPattern.FindAllString(message, -1)
		translatedVerbs := formatVerbPattern.FindAllString(
			localeCatalogs[LanguageKorean][message],
			-1,
		)
		if !reflect.DeepEqual(sourceVerbs, translatedVerbs) {
			t.Errorf(
				"Korean format verbs for %q = %v, want %v",
				message,
				translatedVerbs,
				sourceVerbs,
			)
		}
	}
}

func TestLegacySettingsRetainSystemLanguageDefault(t *testing.T) {
	settings := defaultSettings()
	systemDefault := settings.Language
	if err := json.Unmarshal([]byte(`{"theme_mode":"dark"}`), &settings); err != nil {
		t.Fatal(err)
	}
	settings.normalize()
	if settings.Language != systemDefault {
		t.Fatalf(
			"legacy language = %q, want system default %q",
			settings.Language,
			systemDefault,
		)
	}
	if systemDefault != string(LanguageEnglish) && systemDefault != string(LanguageKorean) {
		t.Fatalf("system default language = %q", systemDefault)
	}
}

func TestLanguageSelectionPersistsAndRebuildsInterface(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)

	settings := defaultSettings()
	settings.Language = string(LanguageEnglish)
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}

	shell := NewShell(NullBackend{}, nil, "")
	shell.openSettingsPanel()
	previousUI := shell.interfaceUI
	shell.cycleLanguage()

	if shell.settings.Language != string(LanguageKorean) {
		t.Fatalf("selected language = %q, want ko", shell.settings.Language)
	}
	if shell.interfaceUI == previousUI {
		t.Fatal("language change did not rebuild the interface")
	}
	command, ok := shell.findCommand("file.open")
	if !ok || command.DisplayLabel(shell) != "파일 열기..." {
		t.Fatalf("localized open command = %q, found=%t", command.DisplayLabel(shell), ok)
	}
	if reloaded := loadSettings(); reloaded.Language != string(LanguageKorean) {
		t.Fatalf("persisted language = %q, want ko", reloaded.Language)
	}
}

func TestKoreanCatalogTranslatesCoreSurfaces(t *testing.T) {
	for _, message := range []string{
		"File",
		"Settings",
		"Language",
		"Ready - use File > Open File...",
	} {
		if translated := translate(LanguageKorean, message); translated == message {
			t.Errorf("%q does not have a Korean translation", message)
		}
	}
}
