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

// TestHostLocaleDrivesFirstRunLanguage covers the platforms Go cannot ask for
// a locale. Android starts the runtime without LANG or its relatives, so the
// binding declares the device language instead; a saved setting still wins,
// because the host value only feeds the first-run default.
func TestHostLocaleDrivesFirstRunLanguage(t *testing.T) {
	t.Cleanup(func() { SetHostLocale("") })

	for _, tc := range []struct {
		tag  string
		want Language
	}{
		{"ko-KR", LanguageKorean},
		{"ko", LanguageKorean},
		{"en-US", LanguageEnglish},
		{"ja-JP", LanguageEnglish},
	} {
		SetHostLocale(tc.tag)
		if got := systemLanguage(); got != tc.want {
			t.Errorf("host locale %q resolved to %q, want %q", tc.tag, got, tc.want)
		}
		if got := defaultSettings().Language; got != string(tc.want) {
			t.Errorf("host locale %q gave a default of %q, want %q",
				tc.tag, got, tc.want)
		}
	}

	SetHostLocale("ko-KR")
	saved := defaultSettings()
	saved.Language = string(LanguageEnglish)
	saved.normalize()
	if saved.Language != string(LanguageEnglish) {
		t.Errorf("a saved language was replaced by the host locale: %q", saved.Language)
	}
}
