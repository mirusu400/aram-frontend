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
