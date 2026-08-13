package frontend

import "testing"

func TestApplicationVersionPrefersReleaseMetadata(t *testing.T) {
	build := debugBuildReport{
		Main:     debugModuleReport{Version: "(devel)"},
		Revision: "0123456789abcdef",
	}
	for _, test := range []struct {
		override string
		want     string
	}{
		{override: "v1.2.3", want: "v1.2.3"},
		{override: "Nightly-1234567", want: "Nightly 1234567"},
		{
			override: "Nightly-1234567-c2345678-f3456789",
			want:     "Nightly 1234567-c2345678-f3456789",
		},
	} {
		if got := applicationVersion(test.override, build); got != test.want {
			t.Fatalf(
				"applicationVersion(%q) = %q, want %q",
				test.override,
				got,
				test.want,
			)
		}
	}
}

func TestApplicationVersionFallsBackToGoBuildInfo(t *testing.T) {
	if got := applicationVersion("", debugBuildReport{
		Main: debugModuleReport{Version: "v0.9.0"},
	}); got != "v0.9.0" {
		t.Fatalf("module version = %q", got)
	}
	if got := applicationVersion("", debugBuildReport{
		Main:     debugModuleReport{Version: "(devel)"},
		Revision: "0123456789abcdef",
		Modified: true,
	}); got != "Development 0123456 (modified)" {
		t.Fatalf("development version = %q", got)
	}
	if got := applicationVersion("", debugBuildReport{}); got !=
		"Development build" {
		t.Fatalf("empty version = %q", got)
	}
}

func TestNightlyBuildStampOnlyLabelsNightlyBuilds(t *testing.T) {
	vcsBuild := debugBuildReport{Time: "2026-08-13T05:04:00Z"}
	for _, test := range []struct {
		name      string
		override  string
		timestamp string
		build     debugBuildReport
		want      string
	}{
		{
			name:      "stable release has no stamp",
			override:  "v1.2.3",
			timestamp: "2026-08-13T05:04:00Z",
			want:      "",
		},
		{
			name:      "development build has no stamp",
			override:  "Development-1234567",
			timestamp: "2026-08-13T05:04:00Z",
			want:      "",
		},
		{
			name:      "nightly uses the injected timestamp",
			override:  "Nightly-1234567",
			timestamp: "2026-08-13T05:04:00+09:00",
			want:      "2026-08-12 20:04 UTC",
		},
		{
			name:     "nightly falls back to the commit time",
			override: "Nightly-1234567",
			build:    vcsBuild,
			want:     "2026-08-13 05:04 UTC",
		},
		{
			name:     "nightly without any time has no stamp",
			override: "Nightly-1234567",
			want:     "",
		},
		{
			name:      "unparsable timestamp is shown raw",
			override:  "Nightly-1234567",
			timestamp: "next tuesday",
			want:      "next tuesday",
		},
	} {
		got := nightlyBuildStamp(test.override, test.timestamp, test.build)
		if got != test.want {
			t.Errorf(
				"%s: nightlyBuildStamp(%q, %q) = %q, want %q",
				test.name,
				test.override,
				test.timestamp,
				got,
				test.want,
			)
		}
	}
}
