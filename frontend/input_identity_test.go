package frontend

import (
	"strings"
	"testing"
)

func TestJavaMEPanelsPreserveBackendFormatAndProfile(t *testing.T) {
	for _, profile := range []string{"j2me-1.0/generic/generic", "j2me-1.0/lgt/generic"} {
		for _, language := range []Language{LanguageEnglish, LanguageKorean} {
			for _, kind := range []string{"properties", "compatibility"} {
				t.Run(profile+"/"+string(language)+"/"+kind, func(t *testing.T) {
					settings := defaultSettings()
					settings.Language = string(language)
					shell := &Shell{
						backend: NullBackend{}, settings: settings,
						input: &InputInfo{DisplayName: "synthetic.jar", Format: "j2me", ProfileID: profile},
						panel: &Panel{Kind: kind},
					}
					text := strings.Join(shell.panelLines(), "\n")
					if !strings.Contains(text, "j2me") || !strings.Contains(text, profile) {
						t.Fatalf("backend identity missing from %s: %s", kind, text)
					}
					if strings.Contains(text, "wipi-1.2.1") || strings.Contains(text, "/skt/") {
						t.Fatalf("Java profile misrepresented as WIPI/SKT: %s", text)
					}
				})
			}
		}
	}
}
