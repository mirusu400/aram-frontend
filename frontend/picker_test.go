package frontend

import (
	"slices"
	"testing"
)

func TestInputPickerPatternsIncludeZIPPackages(t *testing.T) {
	if !slices.Contains(supportedInputPatterns(), "*.zip") {
		t.Fatal("supported input picker patterns do not include ZIP packages")
	}
	if !slices.Contains(wipiPackagePatterns(), "*.zip") {
		t.Fatal("WIPI package picker patterns do not include ZIP packages")
	}
}

func TestInputPickerPreservesExistingFormatsAndIncludesJavaME(t *testing.T) {
	patterns := supportedInputPatterns()
	for _, pattern := range []string{
		"*.dat", "*.jar", "*.JAR", "*.zip", "*.ZIP", "*.sgs", "*.SGS",
		"*.wbin", "*.wbt", "*.bin", "*.rom", "*.img", "*.mbn",
	} {
		if !slices.Contains(patterns, pattern) {
			t.Errorf("supported inputs omit %s", pattern)
		}
	}
	for _, pattern := range []string{"*.jar", "*.JAR", "*.zip", "*.ZIP"} {
		if !slices.Contains(j2mePackagePatterns(), pattern) {
			t.Errorf("Java ME inputs omit %s", pattern)
		}
	}
	// A standalone JAD requires a companion-document contract. Do not imply
	// it is supported or that its MIDlet-Jar-URL will be downloaded.
	for _, patterns := range [][]string{patterns, j2mePackagePatterns()} {
		for _, pattern := range []string{"*.jad", "*.JAD"} {
			if slices.Contains(patterns, pattern) {
				t.Errorf("unsupported standalone descriptor advertised: %s", pattern)
			}
		}
	}
}

func TestJ2MEPickerLabelsAreLocalized(t *testing.T) {
	for _, language := range []Language{LanguageEnglish, LanguageKorean} {
		for _, key := range []string{"Open application package or firmware", "J2ME packages (JAR or JAD/JAR ZIP)", "GVM inputs (recognition only)"} {
			if got := translate(language, key); got == "" || (language == LanguageKorean && got == key) {
				t.Errorf("missing %s translation for %q", language, key)
			}
		}
	}
}
